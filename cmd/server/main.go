package main

import (
	"log"
	"net/http"
	"os"
	"skat/logger"
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

	// Enable Cloud Logging automatically when running on Cloud Run
	isCloudRun := os.Getenv("K_SERVICE") != ""
	if isCloudRun && os.Getenv("CLOUD_LOGGING_ENABLED") == "" {
		os.Setenv("CLOUD_LOGGING_ENABLED", "true")
	}

	// Initialize logger
	appLogger, err := logger.Initialize("skat-server")
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer appLogger.Close()

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
				logger.Warning("Failed to connect to Turso database", "error", err)
			}
		} else if strings.HasPrefix(dbURL, "postgres://") || strings.HasPrefix(dbURL, "postgresql://") || strings.Contains(dbURL, "host=") {
			// PostgreSQL database
			database, err = db.NewPgDatabase(dbURL)
			if err != nil {
				logger.Warning("Failed to connect to PostgreSQL database", "error", err)
			}
		} else {
			logger.Warning("Unknown database URL scheme", "dbURL", dbURL)
		}

		if database != nil {
			defer database.Close()
			// Initialize schema
			if err := database.InitSchema(); err != nil {
				logger.Warning("Failed to initialize database schema", "error", err)
			}
		}
	}

	if database == nil {
		logger.Info("No database configured - using in-memory database")
		database = db.NewMemoryDatabase()
	}

	// Ensure we always have a database (fallback to memory)
	if database == nil {
		logger.Info("Falling back to in-memory database")
		database = db.NewMemoryDatabase()
	}

	srv := server.NewServer(database)
	router := srv.SetupRoutes()

	// Start cleanup task: check every 5 minutes for games inactive for 15+ minutes
	srv.StartCleanupTask(5, 15)

	logger.Info("Starting Skat server", "port", port)
	if err := http.ListenAndServe(":"+port, router); err != nil {
		logger.Fatal("Server failed", "error", err)
	}
}
