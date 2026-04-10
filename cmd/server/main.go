package main

import (
	"log"
	"net/http"
	"os"
	"skat/server"

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
	var db *server.Database
	if os.Getenv("DB_PASSWORD") != "" {
		var err error
		db, err = server.NewDatabase()
		if err != nil {
			log.Printf("Warning: Failed to connect to database: %v", err)
			log.Println("Continuing without database persistence")
			db = nil
		} else {
			defer db.Close()

			// Initialize schema
			if err := db.InitSchema(); err != nil {
				log.Printf("Warning: Failed to initialize database schema: %v", err)
			}
		}
	} else {
		log.Println("No database configured (DB_PASSWORD not set)")
		log.Println("Running in memory-only mode")
	}

	srv := server.NewServer(db)
	router := srv.SetupRoutes()

	log.Printf("Starting Skat server on port %s", port)
	if err := http.ListenAndServe(":"+port, router); err != nil {
		log.Fatal(err)
	}
}
