package main

import (
	"fmt"
	"os"
	"skat/agent"
	"skat/config"
)

func main() {
	fmt.Println("Q-Table Storage Configuration Test")
	fmt.Println("===================================\n")

	// Load configuration from environment
	cfg := config.LoadFromEnv()
	fmt.Printf("Storage backend: %s\n\n", cfg)

	// Display current configuration
	fmt.Println("Configuration:")
	fmt.Printf("  QTABLE_JSON_PATH:   %s\n", cfg.JSONPath)
	fmt.Printf("  QTABLE_BINARY_PATH: %s\n", cfg.BinaryPath)
	fmt.Printf("  QTABLE_FORMAT:      %s\n", map[bool]string{true: "binary", false: "json"}[cfg.UseBinary])
	fmt.Printf("  GCS_BUCKET:         %s\n", cfg.GCSBucket)
	fmt.Printf("  GCS_QTABLE_PATH:    %s\n", cfg.GCSObjectPath)
	fmt.Printf("  USE_GCS:            %v\n\n", cfg.UseGCS)

	// Test loading Q-table
	fmt.Println("Test 1: Load Q-table")
	fmt.Println("--------------------")

	ba := agent.NewBiddingAgent()
	if err := cfg.LoadQTable(ba); err != nil {
		fmt.Printf("✗ Load failed: %v\n", err)
		fmt.Println("\nTroubleshooting:")
		if cfg.UseGCS {
			fmt.Println("  1. Authenticate: gcloud auth application-default login")
			fmt.Println("  2. Check bucket: gsutil ls gs://" + cfg.GCSBucket)
			fmt.Println("  3. Check object: gsutil ls gs://" + cfg.GCSBucket + "/" + cfg.GCSObjectPath)
		} else {
			fmt.Printf("  1. Check file exists: %s\n", map[bool]string{true: cfg.BinaryPath, false: cfg.JSONPath}[cfg.UseBinary])
			fmt.Println("  2. Train first: go run cmd/train_bidding/main.go")
		}
		os.Exit(1)
	}

	stats := ba.GetQTableStats()
	fmt.Printf("✓ Loaded successfully\n")
	fmt.Printf("  States:        %v\n", stats["total_states"])
	fmt.Printf("  State-actions: %v\n", stats["total_state_actions"])
	fmt.Printf("  Epsilon:       %.3f\n\n", stats["epsilon"])

	// Test saving Q-table
	fmt.Println("Test 2: Save Q-table")
	fmt.Println("--------------------")

	if err := cfg.SaveQTable(ba); err != nil {
		fmt.Printf("✗ Save failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Saved successfully\n")
	if cfg.UseGCS {
		fmt.Printf("  Local:  %s\n", map[bool]string{true: cfg.BinaryPath, false: cfg.JSONPath}[cfg.UseBinary])
		fmt.Printf("  GCS:    gs://%s/%s\n", cfg.GCSBucket, cfg.GCSObjectPath)
	} else {
		fmt.Printf("  File:   %s\n", map[bool]string{true: cfg.BinaryPath, false: cfg.JSONPath}[cfg.UseBinary])
	}

	fmt.Println("\n✓ All configuration tests passed!")
	fmt.Println("\nEnvironment variable reference:")
	fmt.Println("  PORT=8080                                    # Server port")
	fmt.Println("  QTABLE_FORMAT=binary                         # Format: json or binary")
	fmt.Println("  QTABLE_JSON_PATH=bidding_qtable.json         # JSON file path")
	fmt.Println("  QTABLE_BINARY_PATH=bidding_qtable.gob        # Binary file path")
	fmt.Println("  GCS_BUCKET=my-bucket                         # Enable GCS storage")
	fmt.Println("  GCS_QTABLE_PATH=qtables/bidding_latest.gob   # GCS object path")
}
