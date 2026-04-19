package main

import (
	"fmt"
	"skat/agent"
	"skat/game"
)

func main() {
	fmt.Println("Loading Trained Bidding Agent")
	fmt.Println("=============================")
	fmt.Println()

	// Create agent and load saved Q-table
	biddingAgent := agent.NewSkatAgent("Agent", 500)

	fmt.Println("Loading Q-table from bidding_qtable.json...")
	if err := biddingAgent.LoadQTable("bidding_qtable.json"); err != nil {
		fmt.Printf("Error loading Q-table: %v\n", err)
		fmt.Println("\nRun 'go run cmd/train_bidding/main.go' first to train and save an agent.")
		return
	}

	fmt.Println("✓ Q-table loaded successfully\n")

	// Show stats
	stats := biddingAgent.GetQTableStats()
	fmt.Printf("Q-table statistics:\n")
	fmt.Printf("  States learned: %v\n", stats["total_states"])
	fmt.Printf("  State-actions:  %v\n", stats["total_state_actions"])
	fmt.Printf("  Q-value range:  %.3f to %.3f\n", stats["min_q"], stats["max_q"])
	fmt.Printf("  Average Q:      %.3f\n", stats["avg_q"])
	fmt.Printf("  Epsilon:        %.3f\n", stats["epsilon"])

	// Test on a few hands
	fmt.Println("\nTesting loaded agent on random hands:")
	fmt.Println("--------------------------------------")

	biddingAgent.Epsilon = 0.0 // No exploration

	for i := 0; i < 10; i++ {
		g := game.NewGame()
		hand := g.Players[0].Hand

		handScore := evaluateHandStrength(hand)
		bid := biddingAgent.Bid(g, 18)

		fmt.Printf("Hand %2d: Score=%2d, Bid=%2d", i+1, handScore, bid)

		// Show a few cards
		fmt.Printf(" [")
		for j := 0; j < 3 && j < len(hand); j++ {
			fmt.Printf("%s", hand[j])
			if j < 2 {
				fmt.Print(" ")
			}
		}
		fmt.Println("...]")
	}
}

func evaluateHandStrength(hand []game.Card) int {
	score := 0
	jacks := 0
	jackSuits := make(map[game.Suit]bool)

	for _, card := range hand {
		if card.Rank == game.Jack {
			jacks++
			jackSuits[card.Suit] = true
		}
		score += card.Value()
	}

	score += jacks * 15

	if jackSuits[game.Clubs] {
		score += 10
		if jackSuits[game.Spades] {
			score += 8
		}
	}

	if score > 100 {
		score = 100
	}

	return score
}
