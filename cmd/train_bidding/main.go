package main

import (
	"fmt"
	"skat/config"
	"skat/training"
)

func main() {
	fmt.Println("Skat Bidding Agent Training")
	fmt.Println("============================")
	fmt.Println()

	trainer := training.NewBiddingTrainer()

	// Train bidding through self-play (reduced for testing new rewards)
	trainer.TrainBidding(300)

	// Save the trained Q-table using config system
	fmt.Println("\nSaving trained agent...")
	agent := trainer.GetBiddingAgent(0)

	cfg := config.LoadFromEnv()
	fmt.Printf("Storage backend: %s\n", cfg)

	if err := cfg.SaveQTable(agent); err != nil {
		fmt.Printf("✗ Save failed: %v\n", err)
	} else {
		fmt.Println("✓ Saved successfully")
	}

	stats := agent.GetQTableStats()
	fmt.Printf("\nQ-table statistics:\n")
	fmt.Printf("  States learned: %v\n", stats["total_states"])
	fmt.Printf("  State-actions:  %v\n", stats["total_state_actions"])
	fmt.Printf("  Q-value range:  %.3f to %.3f\n", stats["min_q"], stats["max_q"])
	fmt.Printf("  Average Q:      %.3f\n", stats["avg_q"])
	fmt.Printf("  Final epsilon:  %.3f\n", stats["epsilon"])
}
