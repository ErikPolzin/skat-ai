package main

import (
	"fmt"
	"skat/agent"
	"skat/game"
)

func main() {
	fmt.Println("Testing All Game Modes")
	fmt.Println("======================")
	fmt.Println()

	// Create test agents
	mcts := agent.NewMCTSAgent("MCTS", 100) // Fewer sims for speed
	bidding := agent.NewBiddingAgent()

	// Test each game mode
	modes := []struct {
		mode game.GameMode
		suit game.Suit
		name string
	}{
		{game.ModeGrand, game.Clubs, "Grand"},
		{game.ModeSuit, game.Clubs, "Clubs"},
		{game.ModeSuit, game.Spades, "Spades"},
		{game.ModeSuit, game.Hearts, "Hearts"},
		{game.ModeSuit, game.Diamonds, "Diamonds"},
		{game.ModeNull, game.Clubs, "Null"},
	}

	for _, testMode := range modes {
		fmt.Printf("Testing %s...\n", testMode.name)

		g := game.NewGame()
		g.Declarer = 0
		g.Players[0].Hand = append(g.Players[0].Hand, g.Skat[:]...)

		// Discard 2 lowest cards
		discardSimple(g, 0)

		g.Mode = testMode.mode
		g.TrumpSuit = testMode.suit
		g.Phase = game.PhasePlaying
		g.CurrentPlayer = 0

		// Play game
		moves := 0
		for g.Phase == game.PhasePlaying && moves < 30 {
			validMoves := g.GetValidMoves()
			if len(validMoves) == 0 {
				break
			}

			move := mcts.SelectMove(g, validMoves)
			if _, err := g.PlayCard("", move); err != nil {
				fmt.Printf("  ✗ Error: %v\n", err)
				break
			}
			moves++
		}

		// Check results
		if g.Phase != game.PhaseComplete {
			fmt.Printf("  ✗ Game didn't complete (moves: %d)\n", moves)
			continue
		}

		// Test game mode selection
		if testMode.mode != game.ModeNull {
			selectedMode, selectedSuit := bidding.ChooseGameMode(g.Players[0].Hand)
			fmt.Printf("  → Bidding agent would choose: %s", modeName(selectedMode))
			if selectedMode == game.ModeSuit {
				fmt.Printf(" (%s)", suitName(selectedSuit))
			}
			fmt.Println()
		}

		fmt.Println()
	}

	fmt.Println("All game modes tested successfully!")
}

func discardSimple(g *game.GameState, declarer int) {
	player := g.Players[declarer]
	type cv struct {
		card  game.Card
		value int
	}

	cards := make([]cv, len(player.Hand))
	for i, card := range player.Hand {
		val := card.Value()
		if card.Rank == game.Jack {
			val = 100
		}
		cards[i] = cv{card, val}
	}

	for i := 0; i < len(cards)-1; i++ {
		for j := i + 1; j < len(cards); j++ {
			if cards[i].value > cards[j].value {
				cards[i], cards[j] = cards[j], cards[i]
			}
		}
	}

	discard1 := cards[0].card
	discard2 := cards[1].card

	newHand := make([]game.Card, 0, 10)
	discarded := 0
	for _, card := range player.Hand {
		if discarded < 2 && (card == discard1 || card == discard2) {
			discarded++
			continue
		}
		newHand = append(newHand, card)
	}
	player.Hand = newHand
}

func modeName(mode game.GameMode) string {
	switch mode {
	case game.ModeGrand:
		return "Grand"
	case game.ModeSuit:
		return "Suit"
	case game.ModeNull:
		return "Null"
	default:
		return "Unknown"
	}
}

func suitName(suit game.Suit) string {
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
		return "Unknown"
	}
}
