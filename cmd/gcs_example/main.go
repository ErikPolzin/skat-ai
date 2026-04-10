package main

import (
	"context"
	"fmt"
	"os"
	"skat/agent"
)

// Example: Upload and download Q-tables to/from Google Cloud Storage
//
// Prerequisites:
//   1. gcloud auth application-default login
//   2. export GCS_BUCKET=your-bucket-name
//   3. gsutil mb gs://your-bucket-name (create bucket)

func main() {
	ctx := context.Background()

	// Get bucket name from environment
	bucketName := os.Getenv("GCS_BUCKET")
	if bucketName == "" {
		fmt.Println("Error: Set GCS_BUCKET environment variable")
		fmt.Println("Example: export GCS_BUCKET=my-skat-models")
		os.Exit(1)
	}

	fmt.Println("GCS Q-Table Upload/Download Example")
	fmt.Println("====================================\n")

	// Load existing Q-table
	ba := agent.NewBiddingAgent()
	if err := ba.LoadQTable("bidding_qtable.json"); err != nil {
		fmt.Printf("Error: %v\n", err)
		fmt.Println("Run 'go run cmd/train_bidding/main.go' first")
		os.Exit(1)
	}

	stats := ba.GetQTableStats()
	fmt.Printf("Loaded Q-table: %v states, %v state-actions\n\n",
		stats["total_states"], stats["total_state_actions"])

	// Example 1: Upload binary format
	objectPath := "qtables/bidding_latest.gob"
	fmt.Printf("Uploading to gs://%s/%s ...\n", bucketName, objectPath)

	if err := ba.SaveQTableToGCS(ctx, bucketName, objectPath, true); err != nil {
		fmt.Printf("✗ Upload failed: %v\n", err)
		fmt.Println("\nTroubleshooting:")
		fmt.Println("  1. Authenticate: gcloud auth application-default login")
		fmt.Println("  2. Create bucket: gsutil mb gs://" + bucketName)
		fmt.Println("  3. Check IAM permissions")
		os.Exit(1)
	}
	fmt.Println("✓ Uploaded binary Q-table")

	// Example 2: Upload JSON format (for debugging)
	jsonPath := "qtables/bidding_latest.json"
	fmt.Printf("Uploading to gs://%s/%s ...\n", bucketName, jsonPath)

	if err := ba.SaveQTableToGCS(ctx, bucketName, jsonPath, false); err != nil {
		fmt.Printf("✗ Upload failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ Uploaded JSON Q-table")

	// Example 3: Download from GCS
	fmt.Printf("\nDownloading from gs://%s/%s ...\n", bucketName, objectPath)

	ba2 := agent.NewBiddingAgent()
	if err := ba2.LoadQTableFromGCS(ctx, bucketName, objectPath, true); err != nil {
		fmt.Printf("✗ Download failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ Downloaded Q-table from GCS")

	// Verify data integrity
	stats2 := ba2.GetQTableStats()
	if stats["total_state_actions"] != stats2["total_state_actions"] {
		fmt.Println("✗ Data mismatch after download!")
		os.Exit(1)
	}
	fmt.Println("✓ Data integrity verified")

	// Example 4: List GCS objects
	fmt.Printf("\nUploaded objects:\n")
	fmt.Printf("  gs://%s/%s\n", bucketName, objectPath)
	fmt.Printf("  gs://%s/%s\n", bucketName, jsonPath)
	fmt.Printf("\nView with: gsutil ls -lh gs://%s/qtables/\n", bucketName)

	fmt.Println("\n✓ All GCS operations successful!")
}
