package main

import (
	"fmt"
	"skat/agent"
	"skat/game"
	"skat/training"
	"strings"
)

func main() {
	fmt.Println("Bidding Agent Evaluation")
	fmt.Println("========================")
	fmt.Println()

	// First train an agent
	fmt.Println("Training bidding agent...")
	trainer := training.NewBiddingTrainer()
	trainer.TrainBidding(5000)

	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("EVALUATION: Trained vs Heuristic Bidding")
	fmt.Println(strings.Repeat("=", 50) + "\n")

	// Test: Trained agent vs heuristic bidders
	trainedAgent := trainer.GetBiddingAgent(0)
	trainedAgent.Epsilon = 0.0 // No exploration during eval

	mctsPlayer := agent.NewSkatAgent("MCTS", 300)

	trainedWins := 0
	trainedGames := 0
	heuristicWins := 0
	heuristicGames := 0

	numGames := 500

	for i := 0; i < numGames; i++ {
		g := game.NewGame()

		// Initialize players
		for p := 0; p < 3; p++ {
			g.Players[p] = &game.PlayerState{
				ID:      fmt.Sprintf("player-%d", p),
				Name:    fmt.Sprintf("Player %d", p),
				Hand:    []game.Card{},
				IsAgent: true,
			}
		}

		// Deal cards
		deck := game.NewDeck()
		idx := 0
		for round := 0; round < 3; round++ {
			for p := 0; p < 3; p++ {
				count := 3
				if round == 1 {
					count = 4
				}
				for j := 0; j < count; j++ {
					g.Players[p].Hand = append(g.Players[p].Hand, deck[idx])
					idx++
				}
			}
		}
		g.Skat[0] = deck[30]
		g.Skat[1] = deck[31]

		// Bidding phase - use game's bidding logic
		g.Phase = game.PhaseBidding
		g.CurrentPlayer = game.Speaker
		g.BidValue = 0

		// Simple bidding: each player decides once whether to accept starting bid
		accepts := [3]bool{}
		for p := 0; p < 3; p++ {
			g.CurrentPlayer = game.GamePosition(p)
			if p == 0 {
				// Trained agent
				accepts[p] = trainedAgent.Bid(g)
			} else {
				// Heuristic: accept if hand is strong enough
				handScore := evaluateHandSimple(g.Players[p].Hand)
				accepts[p] = handScore > 40
			}
		}

		// Find declarer (first to accept)
		declarer := -1
		for p := 0; p < 3; p++ {
			if accepts[p] {
				declarer = p
				break
			}
		}

		if declarer == -1 {
			continue // Everyone passed
		}

		// Play the game
		g.Declarer = game.GamePosition(declarer)
		g.Players[declarer].Hand = append(g.Players[declarer].Hand, g.Skat[:]...)
		discardSimple(g, declarer)

		g.Phase = game.PhasePlaying
		g.Mode = game.ModeGrand
		g.TrumpSuit = game.Clubs
		g.CurrentPlayer = game.GamePosition(declarer)

		for g.Phase == game.PhasePlaying {
			validMoves := g.GetValidMoves()
			if len(validMoves) == 0 {
				break
			}
			move := mctsPlayer.SelectMove(g, validMoves)
			g.PlayCard("", move)
		}

		won := g.DeclarerScore >= 61

		if declarer == 0 {
			trainedGames++
			if won {
				trainedWins++
			}
		} else {
			heuristicGames++
			if won {
				heuristicWins++
			}
		}

		if (i+1)%100 == 0 {
			tWR := 0.0
			if trainedGames > 0 {
				tWR = float64(trainedWins) / float64(trainedGames) * 100
			}
			hWR := 0.0
			if heuristicGames > 0 {
				hWR = float64(heuristicWins) / float64(heuristicGames) * 100
			}
			fmt.Printf("Game %d: Trained %.1f%% (%d/%d) | Heuristic %.1f%% (%d/%d)\n",
				i+1, tWR, trainedWins, trainedGames, hWR, heuristicWins, heuristicGames)
		}
	}

	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("FINAL RESULTS")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Printf("\nTrained Agent:    %.1f%% win rate (%d/%d games as declarer)\n",
		float64(trainedWins)/float64(trainedGames)*100, trainedWins, trainedGames)
	fmt.Printf("Heuristic Agents: %.1f%% win rate (%d/%d games as declarer)\n",
		float64(heuristicWins)/float64(heuristicGames)*100, heuristicWins, heuristicGames)

	improvement := (float64(trainedWins)/float64(trainedGames) - float64(heuristicWins)/float64(heuristicGames)) * 100
	fmt.Printf("\nImprovement: %+.1f percentage points\n", improvement)
}

func evaluateHandSimple(hand []game.Card) int {
	score := 0
	for _, card := range hand {
		if card.Rank == game.Jack {
			score += 15
		}
		score += card.Value()
	}
	return score
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
