package main

import (
	"fmt"
	"skat/agent"
	"skat/game"
	"strings"
)

func main() {
	// Load the trained Q-learning agents
	gameChoiceData, err := agent.LoadQTableData("game_choice_qtable.gob", true)
	if err != nil {
		fmt.Printf("Error loading game choice Q-table: %v\n", err)
		return
	}

	// Create Q-learning game choice agent
	qGameChoice := agent.NewQLearningGameChoiceStrategy(0.0)
	qGameChoice.SetQTable(gameChoiceData.QTable)
	qGameChoice.SetEpsilon(0.0)

	// Create heuristic for comparison
	heuristicGameChoice := &agent.HeuristicGameChoiceStrategy{}

	// Test cases: strong suit hands
	testCases := []struct {
		name         string
		hand         []game.Card
		expectedSuit game.Suit
		description  string
	}{
		{
			name: "Strong Clubs",
			hand: []game.Card{
				{Suit: game.Clubs, Rank: game.Jack},
				{Suit: game.Clubs, Rank: game.Ace},
				{Suit: game.Clubs, Rank: game.Ten},
				{Suit: game.Clubs, Rank: game.King},
				{Suit: game.Clubs, Rank: game.Queen},
				{Suit: game.Clubs, Rank: game.Nine},
				{Suit: game.Clubs, Rank: game.Eight},
				{Suit: game.Spades, Rank: game.Ace},
				{Suit: game.Spades, Rank: game.Ten},
				{Suit: game.Diamonds, Rank: game.King},
			},
			expectedSuit: game.Clubs,
			description:  "7 clubs including J+A+10",
		},
		{
			name: "Strong Hearts",
			hand: []game.Card{
				{Suit: game.Hearts, Rank: game.Jack},
				{Suit: game.Hearts, Rank: game.Ace},
				{Suit: game.Hearts, Rank: game.Ten},
				{Suit: game.Hearts, Rank: game.King},
				{Suit: game.Hearts, Rank: game.Queen},
				{Suit: game.Hearts, Rank: game.Nine},
				{Suit: game.Spades, Rank: game.Jack},
				{Suit: game.Spades, Rank: game.Ace},
				{Suit: game.Clubs, Rank: game.Ten},
				{Suit: game.Diamonds, Rank: game.Ace},
			},
			expectedSuit: game.Hearts,
			description:  "6 hearts including J+A+10, plus ♠J",
		},
		{
			name: "Strong Diamonds",
			hand: []game.Card{
				{Suit: game.Diamonds, Rank: game.Jack},
				{Suit: game.Diamonds, Rank: game.Ace},
				{Suit: game.Diamonds, Rank: game.Ten},
				{Suit: game.Diamonds, Rank: game.King},
				{Suit: game.Diamonds, Rank: game.Queen},
				{Suit: game.Diamonds, Rank: game.Nine},
				{Suit: game.Diamonds, Rank: game.Eight},
				{Suit: game.Spades, Rank: game.Ace},
				{Suit: game.Clubs, Rank: game.Ace},
				{Suit: game.Hearts, Rank: game.Ten},
			},
			expectedSuit: game.Diamonds,
			description:  "7 diamonds including J+A+10",
		},
		{
			name: "Strong Spades",
			hand: []game.Card{
				{Suit: game.Spades, Rank: game.Jack},
				{Suit: game.Spades, Rank: game.Ace},
				{Suit: game.Spades, Rank: game.Ten},
				{Suit: game.Spades, Rank: game.King},
				{Suit: game.Spades, Rank: game.Queen},
				{Suit: game.Spades, Rank: game.Nine},
				{Suit: game.Clubs, Rank: game.Jack},
				{Suit: game.Clubs, Rank: game.Ace},
				{Suit: game.Hearts, Rank: game.Ace},
				{Suit: game.Diamonds, Rank: game.Ten},
			},
			expectedSuit: game.Spades,
			description:  "6 spades including J+A+10, plus ♣J",
		},
		{
			name: "Long Clubs, Weak",
			hand: []game.Card{
				{Suit: game.Clubs, Rank: game.King},
				{Suit: game.Clubs, Rank: game.Queen},
				{Suit: game.Clubs, Rank: game.Nine},
				{Suit: game.Clubs, Rank: game.Eight},
				{Suit: game.Clubs, Rank: game.Seven},
				{Suit: game.Spades, Rank: game.Jack},
				{Suit: game.Spades, Rank: game.Ace},
				{Suit: game.Spades, Rank: game.Ten},
				{Suit: game.Hearts, Rank: game.Ace},
				{Suit: game.Diamonds, Rank: game.Ten},
			},
			expectedSuit: game.Clubs,
			description:  "5 clubs (no high cards), but length advantage",
		},
	}

	fmt.Println("Testing Q-Learning Game Choice on Strong Suit Hands")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println()

	correctChoices := 0
	totalTests := len(testCases)

	for _, tc := range testCases {
		fmt.Printf("%s: %s\n", tc.name, tc.description)

		// Test Q-learning choice
		qMode, qSuit := qGameChoice.ChooseGame(tc.hand, 18)

		// Test heuristic choice
		hMode, hSuit := heuristicGameChoice.ChooseGame(tc.hand, 18)

		fmt.Printf("  Q-Learning: %s", qMode)
		if qMode == game.ModeSuit {
			fmt.Printf(" (%s)", suitSymbol(qSuit))
		}
		fmt.Println()

		fmt.Printf("  Heuristic:  %s", hMode)
		if hMode == game.ModeSuit {
			fmt.Printf(" (%s)", suitSymbol(hSuit))
		}
		fmt.Println()

		fmt.Printf("  Expected:   Suit (%s)\n", suitSymbol(tc.expectedSuit))

		qCorrect := qMode == game.ModeSuit && qSuit == tc.expectedSuit

		if qCorrect {
			fmt.Printf("  Choice: ✓ Correct\n")
			correctChoices++
		} else {
			fmt.Printf("  Choice: ✗ Wrong\n")
		}

		// Simulate what PlayerPoints reward each choice would get
		fmt.Println()
		fmt.Println("  Estimated rewards (game value if played perfectly):")

		// Calculate game values for each choice
		cards := game.Cards(tc.hand)

		qValue := cards.GameValue(qMode, qSuit)
		hValue := cards.GameValue(hMode, hSuit)
		optimalValue := cards.GameValue(game.ModeSuit, tc.expectedSuit)

		fmt.Printf("    Q-Learning choice (%s %s): Game value = %d, Reward ≈ %.2f\n",
			qMode, suitSymbol(qSuit), qValue, float64(qValue)/60.0)
		fmt.Printf("    Heuristic choice  (%s %s): Game value = %d, Reward ≈ %.2f\n",
			hMode, suitSymbol(hSuit), hValue, float64(hValue)/60.0)
		fmt.Printf("    Expected choice   (Suit %s): Game value = %d, Reward ≈ %.2f\n",
			suitSymbol(tc.expectedSuit), optimalValue, float64(optimalValue)/60.0)

		fmt.Println()
	}

	fmt.Println(strings.Repeat("=", 70))
	accuracy := float64(correctChoices) / float64(totalTests) * 100
	fmt.Printf("Q-Learning accuracy on strong suit hands: %.1f%% (%d/%d correct)\n",
		accuracy, correctChoices, totalTests)
}

func suitSymbol(suit game.Suit) string {
	switch suit {
	case game.Clubs:
		return "Clubs"
	case game.Spades:
		return "Spades"
	case game.Hearts:
		return "Hearts"
	case game.Diamonds:
		return "Diamonds"
	default:
		return "?"
	}
}
