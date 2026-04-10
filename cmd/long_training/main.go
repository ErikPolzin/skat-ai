package main

import (
	"fmt"
	"os"
	"time"
	"skat/training"
)

func main() {
	fmt.Println("Skat AI - Long Training Session")
	fmt.Println("================================")
	fmt.Println()

	startTime := time.Now()

	// Check if we should resume from existing Q-table
	resuming := false
	if _, err := os.Stat("bidding_qtable_long.json"); err == nil {
		fmt.Println("Found existing Q-table: bidding_qtable_long.json")
		fmt.Println("This will continue training from the saved state.")
		fmt.Println()
		resuming = true
	}

	trainer := training.NewBiddingTrainer()

	// If resuming, load the Q-table
	if resuming {
		fmt.Println("Loading previous training state...")
		if err := trainer.GetBiddingAgent(0).LoadQTable("bidding_qtable_long.json"); err != nil {
			fmt.Printf("Warning: Could not load Q-table: %v\n", err)
			fmt.Println("Starting fresh training instead.")
			resuming = false
		} else {
			fmt.Println("✓ Loaded previous Q-table")
			stats := trainer.GetBiddingAgent(0).GetQTableStats()
			fmt.Printf("  States: %v, State-actions: %v, Epsilon: %.3f\n\n",
				stats["total_states"], stats["total_state_actions"], stats["epsilon"])
		}
	}

	// Train for 10,000 episodes
	episodes := 10000
	fmt.Printf("Training for %d episodes (this will take ~10-15 minutes)...\n", episodes)
	fmt.Println("Progress updates every 1000 episodes")
	fmt.Println()

	trainer.TrainBidding(episodes)

	// Save with special filename
	fmt.Println("\nSaving trained agent to bidding_qtable_long.json...")
	agent := trainer.GetBiddingAgent(0)

	if err := agent.SaveQTable("bidding_qtable_long.json"); err != nil {
		fmt.Printf("Error saving Q-table: %v\n", err)
	} else {
		fmt.Println("✓ Saved successfully")

		stats := agent.GetQTableStats()
		fmt.Printf("\nFinal Q-table statistics:\n")
		fmt.Printf("  States learned:     %v\n", stats["total_states"])
		fmt.Printf("  State-actions:      %v\n", stats["total_state_actions"])
		fmt.Printf("  Q-value range:      %.3f to %.3f\n", stats["min_q"], stats["max_q"])
		fmt.Printf("  Average Q:          %.3f\n", stats["avg_q"])
		fmt.Printf("  Final epsilon:      %.3f\n", stats["epsilon"])
	}

	elapsed := time.Since(startTime)
	fmt.Printf("\nTotal training time: %v\n", elapsed.Round(time.Second))

	// Also save as the default Q-table for easy use
	fmt.Println("\nCopying to bidding_qtable.json for general use...")
	if err := agent.SaveQTable("bidding_qtable.json"); err != nil {
		fmt.Printf("Warning: Could not save to default location: %v\n", err)
	} else {
		fmt.Println("✓ Default Q-table updated")
	}

	fmt.Println("\n" + repeat("=", 70))
	fmt.Println("Training complete! 🎉")
	fmt.Println(repeat("=", 70))
	fmt.Println("\nTo evaluate the trained agent:")
	fmt.Println("  go run cmd/full_game/main.go")
	fmt.Println("\nTo analyze the Q-table:")
	fmt.Println("  go run cmd/analyze_qtable/main.go")
}

func repeat(s string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}
