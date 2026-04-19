package main

import (
	"fmt"
	"skat/agent"
)

func main() {
	fmt.Println("Q-Table Analysis")
	fmt.Println("================")
	fmt.Println()

	ba := agent.NewSkatAgent("Agent", 500)
	if err := ba.LoadQTableBinary("bidding_qtable.gob"); err != nil {
		fmt.Printf("Error loading Q-table: %v\n", err)
		return
	}

	stats := ba.GetQTableStats()
	fmt.Printf("Overall Statistics:\n")
	fmt.Printf("  States learned:    %v\n", stats["total_states"])
	fmt.Printf("  State-actions:     %v\n", stats["total_state_actions"])
	fmt.Printf("  Q-value range:     %.3f to %.3f\n", stats["min_q"], stats["max_q"])
	fmt.Printf("  Average Q-value:   %.3f\n", stats["avg_q"])
	fmt.Println()

	// Analyze bidding strategy for different hand strengths
	fmt.Println("Learned Bidding Strategy:")
	fmt.Println(repeat("-", 60))

	testScores := []int{20, 30, 40, 50, 60, 70, 80, 90, 100}

	for _, score := range testScores {
		bestBid, bestQ := ba.GetBestAction(score, 18)

		action := "PASS"
		if bestBid > 0 {
			action = fmt.Sprintf("BID %d", bestBid)
		}

		confidence := ""
		if bestQ > 0.1 {
			confidence = "✓✓ High confidence"
		} else if bestQ > 0 {
			confidence = "✓ Positive"
		} else if bestQ > -0.1 {
			confidence = "~ Uncertain"
		} else {
			confidence = "✗ Negative"
		}

		fmt.Printf("  Hand score %3d: %-12s (Q=%.3f) %s\n",
			score, action, bestQ, confidence)
	}

	fmt.Println()
	fmt.Println("Interpretation:")
	fmt.Println("  • Positive Q-values = good decisions (won games)")
	fmt.Println("  • Negative Q-values = bad decisions (lost games)")
	fmt.Println("  • Missing states = not explored during training")
	fmt.Println()

	// Count how many states prefer bidding vs passing
	bidders := 0
	passers := 0

	allScores := []int{}
	// Get all learned states
	// Since we can't access qTable directly, we'll test common scores
	for score := 0; score <= 100; score++ {
		bestBid, _ := ba.GetBestAction(score, 18)
		if bestBid > 0 {
			bidders++
		} else {
			passers++
		}
		allScores = append(allScores, score)
	}

	fmt.Printf("Strategy Summary:\n")
	fmt.Printf("  Scores where agent bids:   %d/101\n", bidders)
	fmt.Printf("  Scores where agent passes: %d/101\n", passers)
}

func repeat(s string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}
