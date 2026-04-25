package main

import (
	"flag"
	"fmt"
	"skat/agent"
	"skat/agent/training"
	"skat/config"
)

func main() {
	episodes := flag.Int("episodes", 5000, "Number of training episodes")
	flag.Parse()

	fmt.Println("Skat Bidding Agent Training")
	fmt.Println("============================")
	fmt.Printf("Episodes: %d\n\n", *episodes)

	trainer := training.NewBiddingTrainer()

	// Train bidding through self-play
	trainer.TrainBidding(*episodes)

	// Save the trained Q-table using config system
	fmt.Println("\nSaving trained agent...")
	trainedAgent := trainer.GetBiddingAgent(0)
	qStrat, ok := trainedAgent.GetBiddingStrategy().(*agent.QLearningBiddingStrategy)
	if !ok {
		fmt.Println("✗ Agent is not using Q-learning bidding strategy")
		return
	}

	cfg := config.LoadFromEnv()
	fmt.Printf("Storage backend: %s\n", cfg)

	if err := cfg.SaveBiddingQTable(qStrat); err != nil {
		fmt.Printf("✗ Save failed: %v\n", err)
	} else {
		fmt.Println("✓ Saved successfully")
	}

	stats := agent.GetQTableStats(qStrat.GetQTable(), qStrat.GetEpsilon())
	fmt.Printf("\nQ-table statistics:\n")
	fmt.Printf("  States learned: %v\n", stats["total_states"])
	fmt.Printf("  State-actions:  %v\n", stats["total_state_actions"])
	fmt.Printf("  Q-value range:  %.3f to %.3f\n", stats["min_q"], stats["max_q"])
	fmt.Printf("  Average Q:      %.3f\n", stats["avg_q"])
	fmt.Printf("  Final epsilon:  %.3f\n", stats["epsilon"])
}
