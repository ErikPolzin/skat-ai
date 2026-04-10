package config

import (
	"context"
	"fmt"
	"os"
	"skat/agent"
	"strings"
)

// StorageConfig manages Q-table storage configuration
type StorageConfig struct {
	// Local file paths
	JSONPath   string
	BinaryPath string

	// Google Cloud Storage
	GCSBucket     string
	GCSObjectPath string
	UseGCS        bool
	UseBinary     bool
}

// LoadFromEnv loads storage configuration from environment variables
func LoadFromEnv() *StorageConfig {
	return &StorageConfig{
		// Local paths (defaults)
		JSONPath:   getEnv("QTABLE_JSON_PATH", "bidding_qtable.json"),
		BinaryPath: getEnv("QTABLE_BINARY_PATH", "bidding_qtable.gob"),

		// GCS configuration
		GCSBucket:     os.Getenv("GCS_BUCKET"),
		GCSObjectPath: getEnv("GCS_QTABLE_PATH", "qtables/bidding_latest.gob"),
		UseGCS:        os.Getenv("GCS_BUCKET") != "",
		UseBinary:     strings.ToLower(getEnv("QTABLE_FORMAT", "binary")) == "binary",
	}
}

// LoadQTable loads a Q-table using the configured storage backend
func (c *StorageConfig) LoadQTable(ba *agent.BiddingAgent) error {
	if c.UseGCS {
		// Load from Google Cloud Storage
		ctx := context.Background()
		err := ba.LoadQTableFromGCS(ctx, c.GCSBucket, c.GCSObjectPath, c.UseBinary)
		if err != nil {
			return fmt.Errorf("failed to load from GCS: %w", err)
		}
		return nil
	}

	// Load from local file
	if c.UseBinary {
		if err := ba.LoadQTableBinary(c.BinaryPath); err != nil {
			return fmt.Errorf("failed to load binary: %w", err)
		}
	} else {
		if err := ba.LoadQTable(c.JSONPath); err != nil {
			return fmt.Errorf("failed to load JSON: %w", err)
		}
	}

	return nil
}

// SaveQTable saves a Q-table using the configured storage backend
func (c *StorageConfig) SaveQTable(ba *agent.BiddingAgent) error {
	// Always save locally first
	if c.UseBinary {
		if err := ba.SaveQTableBinary(c.BinaryPath); err != nil {
			return fmt.Errorf("failed to save binary: %w", err)
		}
	} else {
		if err := ba.SaveQTable(c.JSONPath); err != nil {
			return fmt.Errorf("failed to save JSON: %w", err)
		}
	}

	// Also save to GCS if configured
	if c.UseGCS {
		ctx := context.Background()
		if err := ba.SaveQTableToGCS(ctx, c.GCSBucket, c.GCSObjectPath, c.UseBinary); err != nil {
			return fmt.Errorf("failed to upload to GCS: %w", err)
		}
	}

	return nil
}

// String returns a human-readable description of the config
func (c *StorageConfig) String() string {
	if c.UseGCS {
		format := "JSON"
		if c.UseBinary {
			format = "binary"
		}
		return fmt.Sprintf("GCS (gs://%s/%s, %s)", c.GCSBucket, c.GCSObjectPath, format)
	}

	if c.UseBinary {
		return fmt.Sprintf("Local binary (%s)", c.BinaryPath)
	}
	return fmt.Sprintf("Local JSON (%s)", c.JSONPath)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
