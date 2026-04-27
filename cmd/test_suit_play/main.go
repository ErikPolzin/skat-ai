package main

import (
	"fmt"
	"skat/agent"
	"skat/game"
	"strings"
)

func main() {
	fmt.Println("Testing Win/Loss with Correct vs Wrong Suit Choices")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println()

	// Strong Clubs hand
	declarerHand := []game.Card{
		{Suit: game.Clubs, Rank: game.Jack},
		{Suit: game.Clubs, Rank: game.Ace},
		{Suit: game.Clubs, Rank: game.Ten},
		{Suit: game.Clubs, Rank: game.King},
		{Suit: game.Clubs, Rank: game.Queen},
		{Suit: game.Clubs, Rank: game.Nine},
		{Suit: game.Clubs, Rank: game.Eight},
		{Suit: game.Spades, Rank: game.Ace},
		{Suit: game.Hearts, Rank: game.Ten},
		{Suit: game.Diamonds, Rank: game.King},
	}

	fmt.Println("Declarer Hand: 7 Clubs (♣J ♣A ♣10 ♣K ♣Q ♣9 ♣8) + ♠A ♥10 ♦K")
	fmt.Println("Strong in Clubs, weak in other suits")
	fmt.Println()

	testCases := []struct {
		name       string
		trumpSuit  game.Suit
		expected   string
	}{
		{"Playing Clubs (CORRECT - 7 trumps)", game.Clubs, "HIGH win rate"},
		{"Playing Spades (WRONG - 1 trump)", game.Spades, "LOW win rate"},
		{"Playing Hearts (WRONG - 1 trump)", game.Hearts, "LOW win rate"},
		{"Playing Diamonds (WRONG - 1 trump)", game.Diamonds, "LOW win rate"},
	}

	numGames := 1000
	var clubsReward float64

	for _, tc := range testCases {
		fmt.Printf("%s\n", tc.name)
		fmt.Println(strings.Repeat("-", 70))

		wins, totalPoints := playGamesWithSuit(declarerHand, tc.trumpSuit, numGames)

		winRate := float64(wins) / float64(numGames) * 100
		avgPoints := float64(totalPoints) / float64(numGames)
		avgReward := avgPoints / 60.0

		if tc.trumpSuit == game.Clubs {
			clubsReward = avgReward
		}

		fmt.Printf("  Games: %d\n", numGames)
		fmt.Printf("  Wins: %d (%.1f%%)\n", wins, winRate)
		fmt.Printf("  Avg PlayerPoints: %.1f\n", avgPoints)
		fmt.Printf("  Avg Reward: %.2f\n", avgReward)

		if tc.trumpSuit != game.Clubs {
			diff := avgReward - clubsReward
			fmt.Printf("  Reward vs Clubs: %.2f (diff: %.2f)\n", clubsReward, diff)
		}

		fmt.Println()
	}
}

func playGamesWithSuit(declarerHand []game.Card, trumpSuit game.Suit, numGames int) (wins, totalPoints int) {
	cardPlay := &agent.HeuristicCardPlayStrategy{}

	for i := 0; i < numGames; i++ {
		g := game.NewGame()

		// Initialize Players array
		for j := 0; j < 3; j++ {
			g.Players[j] = &game.PlayerState{
				ID:      fmt.Sprintf("player-%d", j),
				Name:    fmt.Sprintf("Player %d", j),
				Hand:    []game.Card{},
				IsAgent: true,
			}
		}

		g.Players[0].Hand = make([]game.Card, len(declarerHand))
		copy(g.Players[0].Hand, declarerHand)

		remaining := getRemainingCards(declarerHand)
		for j := range remaining {
			k := (j*17 + i*13) % len(remaining)
			remaining[j], remaining[k] = remaining[k], remaining[j]
		}

		// Need to copy slices, not just reference them
		g.Players[1].Hand = make([]game.Card, 10)
		copy(g.Players[1].Hand, remaining[:10])
		g.Players[2].Hand = make([]game.Card, 10)
		copy(g.Players[2].Hand, remaining[10:20])
		g.Skat = game.SkatCards{remaining[20], remaining[21]}

		declarer := game.GamePosition(0)
		g.Declarer = &declarer
		g.Mode = game.ModeSuit
		g.TrumpSuit = trumpSuit
		g.BidValue = 18
		g.Phase = game.PhasePlaying
		g.CurrentPlayer = game.GamePosition(0)
		g.TrickStarter = game.GamePosition(0)

		for g.Phase == game.PhasePlaying {
			validMoves := g.GetValidMoves()
			if len(validMoves) == 0 {
				break
			}
			card := cardPlay.SelectMove(g, validMoves)
			_, err := g.PlayCard(card)
			if err != nil {
				break
			}

			// Resolve trick if complete
			if len(g.Trick) == 3 {
				g.ResolveTrick()
			}
		}

		if g.Phase == game.PhaseComplete {
			result := g.Result()
			totalPoints += result.Value
			if result.DeclarerWon {
				wins++
			}
		}
	}

	return
}

func getRemainingCards(hand []game.Card) []game.Card {
	allCards := []game.Card{}
	suits := []game.Suit{game.Clubs, game.Spades, game.Hearts, game.Diamonds}
	ranks := []game.Rank{game.Seven, game.Eight, game.Nine, game.Ten, game.Jack, game.Queen, game.King, game.Ace}

	for _, suit := range suits {
		for _, rank := range ranks {
			allCards = append(allCards, game.Card{Suit: suit, Rank: rank})
		}
	}

	handMap := make(map[game.Card]bool)
	for _, card := range hand {
		handMap[card] = true
	}

	remaining := []game.Card{}
	for _, card := range allCards {
		if !handMap[card] {
			remaining = append(remaining, card)
		}
	}

	return remaining
}
