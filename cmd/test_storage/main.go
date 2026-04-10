package main

import (
	"context"
	"fmt"
	"os"
	"skat/agent"
)

func main() {
	fmt.Println("Q-Table Storage Format Test")
	fmt.Println("============================\n")

	// Load existing Q-table
	ba := agent.NewBiddingAgent()
	if err := ba.LoadQTable("bidding_qtable.json"); err != nil {
		fmt.Printf("Error loading Q-table: %v\n", err)
		return
	}

	fmt.Println("✓ Loaded Q-table from JSON")
	stats := ba.GetQTableStats()
	fmt.Printf("  States: %v, State-actions: %v\n\n",
		stats["total_states"], stats["total_state_actions"])

	// Test 1: Save/Load Binary Format
	fmt.Println("Test 1: Binary Format (gob)")
	fmt.Println("----------------------------")

	if err := ba.SaveQTableBinary("bidding_qtable.gob"); err != nil {
		fmt.Printf("✗ Failed to save binary: %v\n", err)
		return
	}
	fmt.Println("✓ Saved to bidding_qtable.gob")

	// Compare file sizes
	jsonInfo, _ := os.Stat("bidding_qtable.json")
	gobInfo, _ := os.Stat("bidding_qtable.gob")

	jsonSize := jsonInfo.Size()
	gobSize := gobInfo.Size()
	reduction := float64(jsonSize-gobSize) / float64(jsonSize) * 100

	fmt.Printf("  JSON size: %d bytes\n", jsonSize)
	fmt.Printf("  GOB size:  %d bytes\n", gobSize)
	fmt.Printf("  Reduction: %.1f%%\n\n", reduction)

	// Load binary and verify
	ba2 := agent.NewBiddingAgent()
	if err := ba2.LoadQTableBinary("bidding_qtable.gob"); err != nil {
		fmt.Printf("✗ Failed to load binary: %v\n", err)
		return
	}

	stats2 := ba2.GetQTableStats()
	if stats["total_state_actions"] != stats2["total_state_actions"] {
		fmt.Printf("✗ Data mismatch after binary load!\n")
		return
	}
	fmt.Println("✓ Binary load successful - data verified\n")

	// Test 2: Google Cloud Storage (optional - requires credentials)
	fmt.Println("Test 2: Google Cloud Storage")
	fmt.Println("-----------------------------")

	ctx := context.Background()
	bucketName := os.Getenv("GCS_BUCKET") // e.g. "my-skat-models"

	if bucketName == "" {
		fmt.Println("⊘ Skipped - Set GCS_BUCKET environment variable to test")
		fmt.Println("  Example: export GCS_BUCKET=my-skat-models")
		fmt.Println("  Requires: GOOGLE_APPLICATION_CREDENTIALS or gcloud auth")
		return
	}

	// Save to GCS (binary)
	objectName := "qtables/bidding_test.gob"
	fmt.Printf("  Uploading to gs://%s/%s ...\n", bucketName, objectName)

	if err := ba.SaveQTableToGCS(ctx, bucketName, objectName, true); err != nil {
		fmt.Printf("✗ Failed to upload to GCS: %v\n", err)
		fmt.Println("  Check: gcloud auth application-default login")
		return
	}
	fmt.Println("✓ Uploaded binary Q-table to GCS")

	// Load from GCS
	ba3 := agent.NewBiddingAgent()
	if err := ba3.LoadQTableFromGCS(ctx, bucketName, objectName, true); err != nil {
		fmt.Printf("✗ Failed to download from GCS: %v\n", err)
		return
	}

	stats3 := ba3.GetQTableStats()
	if stats["total_state_actions"] != stats3["total_state_actions"] {
		fmt.Printf("✗ Data mismatch after GCS load!\n")
		return
	}
	fmt.Println("✓ Downloaded from GCS - data verified")

	// Test JSON format to GCS as well
	objectNameJSON := "qtables/bidding_test.json"
	fmt.Printf("  Uploading to gs://%s/%s ...\n", bucketName, objectNameJSON)

	if err := ba.SaveQTableToGCS(ctx, bucketName, objectNameJSON, false); err != nil {
		fmt.Printf("✗ Failed to upload JSON to GCS: %v\n", err)
		return
	}
	fmt.Println("✓ Uploaded JSON Q-table to GCS")

	fmt.Println("\n✓ All storage tests passed!")
}
