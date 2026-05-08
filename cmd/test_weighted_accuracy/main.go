package main

import (
	"flag"
	"fmt"
	"strings"

	"skat/agent"
	"skat/agent/strategies"
	"skat/game"
)

func main() {
	weightsFile := flag.String("weights", ".data/models/weighted_temp100.json", "Path to weights file")
	gamesPerHand := flag.Int("games", 100, "Number of games to play per hand/mode combination")
	flag.Parse()

	// Load weights
	weights, err := strategies.LoadBidWeights(*weightsFile)
	if err != nil {
		fmt.Printf("Error loading weights: %v\n", err)
		return
	}

	fmt.Println("============================================================")
	fmt.Println("Weighted Heuristic Accuracy Test")
	fmt.Println("============================================================")
	fmt.Printf("\nWeights file: %s\n", *weightsFile)
	fmt.Printf("Temperature: %.1f\n", weights.SigmoidTemperature)
	fmt.Printf("Games per hand/mode: %d\n\n", *gamesPerHand)

	// Define test hands
	testHands := []struct {
		name    string
		handStr string
	}{
		{
			name:    "Strong Grand Hand",
			handStr: "J.♣-J.♠-J.♥-J.♦-A.♠-A.♥-A.♦-10.♣-10.♠-10.♥",
		},
		{
			name:    "Strong Clubs Hand",
			handStr: "J.♣-J.♠-A.♣-10.♣-K.♣-Q.♣-9.♣-A.♠-10.♥-K.♦",
		},
		{
			name:    "Medium Hearts Hand",
			handStr: "J.♣-J.♠-A.♥-10.♥-K.♥-Q.♥-9.♥-A.♣-K.♠-8.♦",
		},
		{
			name:    "Weak Hand",
			handStr: "J.♣-K.♥-Q.♥-9.♣-8.♣-Q.♠-9.♠-7.♥-8.♥-7.♦",
		},
		{
			name:    "Long Diamonds Suit",
			handStr: "J.♣-J.♠-A.♦-10.♦-K.♦-Q.♦-9.♦-8.♦-A.♥-10.♠",
		},
	}

	strategy := strategies.NewWeightedHeuristicBiddingStrategyWithWeights(weights)

	// Test each hand
	for _, th := range testHands {
		hand, err := game.ParseCards(th.handStr)
		if err != nil {
			fmt.Printf("Error parsing hand: %v\n", err)
			continue
		}

		fmt.Println(strings.Repeat("=", 70))
		fmt.Printf("%s\n", th.name)
		fmt.Printf("%s\n\n", th.handStr)

		// Test Grand
		grandProb := strategy.EvaluateGameProbability(game.Cards(hand), game.ModeGrand, game.NoSuit)
		grandWins, grandGames := playGames(hand, game.ModeGrand, game.NoSuit, *gamesPerHand)
		grandActual := float64(grandWins) / float64(grandGames)

		fmt.Printf("Grand:\n")
		fmt.Printf("  Predicted: %.1f%%\n", grandProb*100)
		fmt.Printf("  Actual:    %.1f%% (%d/%d wins)\n", grandActual*100, grandWins, grandGames)
		fmt.Printf("  Error:     %+.1f%%\n\n", (grandActual-grandProb)*100)

		// Test each suit
		suits := []struct {
			name string
			suit game.Suit
		}{
			{"Clubs", game.Clubs},
			{"Spades", game.Spades},
			{"Hearts", game.Hearts},
			{"Diamonds", game.Diamonds},
		}

		for _, s := range suits {
			suitProb := strategy.EvaluateGameProbability(game.Cards(hand), game.ModeSuit, s.suit)
			suitWins, suitGames := playGames(hand, game.ModeSuit, s.suit, *gamesPerHand)
			suitActual := float64(suitWins) / float64(suitGames)

			fmt.Printf("%s:\n", s.name)
			fmt.Printf("  Predicted: %.1f%%\n", suitProb*100)
			fmt.Printf("  Actual:    %.1f%% (%d/%d wins)\n", suitActual*100, suitWins, suitGames)
			fmt.Printf("  Error:     %+.1f%%\n\n", (suitActual-suitProb)*100)
		}
	}
}

// playGames simulates multiple games with the given hand and game mode
func playGames(declarerHand []game.Card, mode game.GameMode, trumpSuit game.Suit, numGames int) (wins, gamesPlayed int) {
	// Create agents - test agent will be declarer, baseline agents will be defenders
	testAgent := agent.NewHeuristicAgent("Test")
	defender1 := agent.NewHeuristicAgent("Defender1")
	defender2 := agent.NewHeuristicAgent("Defender2")

	winsCount := 0
	gamesCount := 0

	for i := 0; i < numGames; i++ {
		// Create a fresh game for each iteration
		g := game.NewGame()

		// Set up players manually without using complex config
		g.Players[0] = &game.PlayerState{
			ID:      "test-agent",
			Name:    "Test",
			Hand:    game.Cards{},
			IsAgent: true,
		}
		g.Players[1] = &game.PlayerState{
			ID:      "defender1",
			Name:    "Defender1",
			Hand:    game.Cards{},
			IsAgent: true,
		}
		g.Players[2] = &game.PlayerState{
			ID:      "defender2",
			Name:    "Defender2",
			Hand:    game.Cards{},
			IsAgent: true,
		}

		agent.SetAgentForPlayer(g.Players[0], testAgent)
		agent.SetAgentForPlayer(g.Players[1], defender1)
		agent.SetAgentForPlayer(g.Players[2], defender2)

		g.Phase = game.PhaseDealing

		// Set up the game with the test agent as declarer at position 0 (Speaker)
		// Get remaining cards and shuffle them for randomness
		remaining := game.Cards(declarerHand).GetRemainingCards()
		remaining.Shuffle()

		// Deal cards manually with shuffled remaining cards
		g.Players[0].Hand = make(game.Cards, 10)
		g.Players[1].Hand = make(game.Cards, 10)
		g.Players[2].Hand = make(game.Cards, 10)
		copy(g.Players[game.Speaker].Hand, declarerHand)
		copy(g.Players[(game.Speaker+1)%3].Hand, remaining[:10])
		copy(g.Players[(game.Speaker+2)%3].Hand, remaining[10:20])
		g.Skat = game.SkatCards{remaining[20], remaining[21]}
		g.Phase = game.PhaseBidding
		g.CurrentPlayer = game.Speaker

		g = g.WithDeclarer(game.Speaker, 0)
		g = g.WithSkatPickedUp(false)
		g = g.WithGame(mode, trumpSuit)

		if !g.Overbid {
			g = agent.WithAgentCardPlay(g)

			// Count wins manually
			playerResults := g.PlayerResults()
			if playerResults != nil {
				gamesCount++
				declarerResult := playerResults[game.Speaker] // Declarer is Speaker (position 2)
				if declarerResult.IsWinner {
					winsCount++
				}
			}
		}
	}

	return winsCount, gamesCount
}
