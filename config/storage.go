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
		GCSObjectPath: getEnv("GCS_QTABLE_PATH", "qtables/bidding_qtable.gob"),
		UseGCS:        os.Getenv("GCS_BUCKET") != "",
		UseBinary:     strings.ToLower(getEnv("QTABLE_FORMAT", "binary")) == "binary",
	}
}

// LoadBiddingQTable loads a bidding Q-table using the configured storage backend
func (c *StorageConfig) LoadBiddingQTable(qStrat *agent.QLearningBiddingStrategy) error {
	var data *agent.QTableData
	var err error

	if c.UseGCS {
		// Load from Google Cloud Storage
		ctx := context.Background()
		data, err = agent.LoadQTableDataFromGCS(ctx, c.GCSBucket, c.GCSObjectPath, c.UseBinary)
		if err != nil {
			return fmt.Errorf("failed to load from GCS: %w", err)
		}
	} else {
		// Load from local file
		path := c.JSONPath
		if c.UseBinary {
			path = c.BinaryPath
		}
		data, err = agent.LoadQTableData(path, c.UseBinary)
		if err != nil {
			return fmt.Errorf("failed to load from file: %w", err)
		}
	}

	qStrat.SetQTable(data.QTable)
	qStrat.SetEpsilon(data.Epsilon)
	return nil
}

// SaveBiddingQTable saves a bidding Q-table using the configured storage backend
func (c *StorageConfig) SaveBiddingQTable(qStrat *agent.QLearningBiddingStrategy) error {
	data := &agent.QTableData{
		QTable:  qStrat.GetQTable(),
		Epsilon: qStrat.GetEpsilon(),
	}

	// Always save locally first
	path := c.JSONPath
	if c.UseBinary {
		path = c.BinaryPath
	}
	if err := agent.SaveQTableData(data, path, c.UseBinary); err != nil {
		return fmt.Errorf("failed to save to file: %w", err)
	}

	// Also save to GCS if configured
	if c.UseGCS {
		ctx := context.Background()
		if err := agent.SaveQTableDataToGCS(ctx, data, c.GCSBucket, c.GCSObjectPath, c.UseBinary); err != nil {
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
