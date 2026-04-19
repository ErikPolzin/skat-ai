package main

import (
	"log"
	"net/http"
	"os"
	"skat/server"
	"skat/server/db"
	"strings"

	"github.com/joho/godotenv"
)

func main() {
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Initialize database (optional - will run without it)
	var database db.Database
	dbURL := os.Getenv("DATABASE_URL")

	if dbURL != "" {
		var err error
		if strings.HasPrefix(dbURL, "libsql://") || strings.HasPrefix(dbURL, "https://") {
			// Turso/LibSQL database
			database, err = db.NewTursoDatabase(dbURL)
			if err != nil {
				log.Printf("Warning: Failed to connect to Turso database: %v", err)
			}
		} else if strings.HasPrefix(dbURL, "postgres://") || strings.HasPrefix(dbURL, "postgresql://") || strings.Contains(dbURL, "host=") {
			// PostgreSQL database
			database, err = db.NewPgDatabase(dbURL)
			if err != nil {
				log.Printf("Warning: Failed to connect to PostgreSQL database: %v", err)
			}
		} else {
			log.Printf("Warning: Unknown database URL scheme: %s", dbURL)
		}

		if database != nil {
			defer database.Close()
			// Initialize schema
			if err := database.InitSchema(); err != nil {
				log.Printf("Warning: Failed to initialize database schema: %v", err)
			}
		}
	}

	if database == nil {
		log.Println("No database configured - using in-memory database")
		database = db.NewMemoryDatabase()
	}

	// Ensure we always have a database (fallback to memory)
	if database == nil {
		log.Println("Falling back to in-memory database")
		database = db.NewMemoryDatabase()
	}

	srv := server.NewServer(database)
	router := srv.SetupRoutes()

	log.Printf("Starting Skat server on port %s", port)
	if err := http.ListenAndServe(":"+port, router); err != nil {
		log.Fatal(err)
	}
}
