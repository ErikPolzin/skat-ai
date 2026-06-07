package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"skat/logger"
	"skat/server"
	cachepkg "skat/server/cache"
	"skat/server/db"
	"strconv"
	"strings"
	"time"

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

	// Initialize database
	var database db.Database
	dbURL := os.Getenv("DATABASE_URL")

	if dbURL != "" {
		var err error
		if strings.HasPrefix(dbURL, "libsql://") || strings.HasPrefix(dbURL, "https://") {
			// Turso/LibSQL database
			database, err = db.NewTursoDatabase(dbURL)
			if err != nil {
				logger.Warning("Failed to connect to Turso database: %e", err)
			}
		} else if strings.HasPrefix(dbURL, "postgres://") || strings.HasPrefix(dbURL, "postgresql://") || strings.Contains(dbURL, "host=") {
			// PostgreSQL database
			database, err = db.NewPgDatabase(dbURL)
			if err != nil {
				logger.Warning("Failed to connect to PostgreSQL database: %e", err)
			}
		} else {
			logger.Warning("Unknown database URL scheme %s", dbURL)
		}

		if database != nil {
			defer database.Close()
			// Initialize schema
			if err := database.InitSchema(); err != nil {
				logger.Warning("Failed to initialize database schema: %e", err)
			}
		}
	}

	if database == nil {
		logger.Fatal("DATABASE_URL must be configured")
	}

	srv := newConfiguredServer(database)
	router := srv.SetupRoutes()

	// Start cleanup task: check every 5 minutes for games inactive for 15+ minutes
	// Start cleanup task: check every 5 minutes for stale games and timeouts
	srv.StartCleanupTask(5, 15)

	logger.Info("Starting Skat server on port %s", port)
	if err := http.ListenAndServe(":"+port, router); err != nil {
		logger.Fatal("Server failed: %e", err)
	}
}

func newConfiguredServer(database db.Database) *server.Server {
	backend := strings.ToLower(os.Getenv("CACHE_BACKEND"))
	cacheTTL := envDuration("CACHE_TTL", 30*time.Minute)

	switch backend {
	case "redis":
		addr := os.Getenv("REDIS_ADDR")
		if addr == "" {
			addr = "localhost:6379"
		}
		redisDB, _ := strconv.Atoi(os.Getenv("REDIS_DB"))
		redisBackend := cachepkg.NewRedisBackend(addr, os.Getenv("REDIS_PASSWORD"), redisDB)
		if err := redisBackend.Ping(context.Background()); err != nil {
			logger.Warning("Failed to connect to Redis cache backend: %e", err)
			return server.NewServer(database)
		}
		cache := cachepkg.NewDistributedCache(database, redisBackend, redisBackend, cacheTTL)
		clients := server.NewClientManager(server.WithClientBackend(redisBackend))
		srv := server.NewServer(
			database,
			server.WithGameCache(cache),
			server.WithClientManager(clients),
		)
		clients.StartMessageBus(context.Background())
		clients.StartPresenceHeartbeat(context.Background())
		cachepkg.StartSyncWorker(context.Background(), database, redisBackend)
		logger.Info("Using Redis cache, client manager, and cache-sync queue at %s", addr)
		return srv
	default:
		return server.NewServer(database)
	}
}

func envDuration(name string, fallback time.Duration) time.Duration {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback
	}
	duration, err := time.ParseDuration(raw)
	if err != nil {
		logger.Warning("Invalid %s=%s, using %s", name, raw, fallback)
		return fallback
	}
	return duration
}
