package training

import (
	"fmt"
	"skat/agent"
	"skat/game"
)

// Trainer manages the self-play training loop
type Trainer struct {
	agents      []*agent.MCTSAgent
	randomAgent *agent.RandomAgent
}

func NewTrainer() *Trainer {
	return &Trainer{
		agents: []*agent.MCTSAgent{
			agent.NewMCTSAgent("MCTS-1", 500),
			agent.NewMCTSAgent("MCTS-2", 500),
			agent.NewMCTSAgent("MCTS-3", 500),
		},
		randomAgent: agent.NewRandomAgent("Random"),
	}
}

// SetSimulations updates the simulation count for all MCTS agents
func (t *Trainer) SetSimulations(sims int) {
	for _, agent := range t.agents {
		agent.SetSimulations(sims)
	}
}

// RunEpisode plays one complete game
func (t *Trainer) RunEpisode(episodeNum int) {
	g := game.NewGame()

	// Simplified: skip bidding, just pick first player as declarer
	g.Declarer = 0

	// Declarer picks up the Skat (gets 2 more cards)
	g.Players[g.Declarer].Hand = append(g.Players[g.Declarer].Hand, g.Skat[:]...)

	// Declarer discards 2 cards (simplified: discard lowest value cards)
	t.declarerDiscard(g)

	g.Phase = game.PhasePlaying
	g.Mode = game.ModeGrand
	g.TrumpSuit = game.Clubs // Doesn't matter for Grand
	g.CurrentPlayer = 0

	moveCount := 0
	for g.Phase == game.PhasePlaying {
		currentAgent := t.agents[g.CurrentPlayer]
		validMoves := g.GetValidMoves()

		if len(validMoves) == 0 {
			break
		}

		move := currentAgent.SelectMove(g, validMoves)
		_, err := g.PlayCard("", move)
		if err != nil {
			fmt.Printf("Error playing card: %v\n", err)
			break
		}

		moveCount++
	}

	// Calculate results
	declarerWon := g.DeclarerScore >= 61

	if episodeNum%10 == 0 {
		winStr := "Lost"
		if declarerWon {
			winStr = "Won"
		}
		fmt.Printf("Episode %d: Declarer %s with %d points (%d moves)\n",
			episodeNum, winStr, g.DeclarerScore, moveCount)
	}
}

// Train runs the full self-play loop
func (t *Trainer) Train(numEpisodes int) {
	fmt.Printf("Starting self-play for %d episodes...\n", numEpisodes)
	fmt.Println("MCTS agents will improve through experience playing against each other")

	for i := 0; i < numEpisodes; i++ {
		t.RunEpisode(i)
	}

	fmt.Println("Self-play complete!")
}

// Evaluate tests the MCTS agent against random baseline
func (t *Trainer) Evaluate(numGames int) {
	wins := 0
	totalPoints := 0

	for i := 0; i < numGames; i++ {
		g := game.NewGame()
		g.Declarer = 0

		// Declarer picks up the Skat
		g.Players[g.Declarer].Hand = append(g.Players[g.Declarer].Hand, g.Skat[:]...)
		t.declarerDiscard(g)

		g.Phase = game.PhasePlaying
		g.Mode = game.ModeGrand
		g.TrumpSuit = game.Clubs
		g.CurrentPlayer = 0

		// MCTS agent as declarer vs 2 random agents
		for g.Phase == game.PhasePlaying {
			validMoves := g.GetValidMoves()
			if len(validMoves) == 0 {
				break
			}

			var move game.Card
			if g.CurrentPlayer == 0 {
				move = t.agents[0].SelectMove(g, validMoves)
			} else {
				move = t.randomAgent.SelectMove(g, validMoves)
			}

			g.PlayCard("", move)
		}

		totalPoints += g.DeclarerScore
		if g.DeclarerScore >= 61 {
			wins++
		}
	}

	winRate := float64(wins) / float64(numGames) * 100
	avgPoints := float64(totalPoints) / float64(numGames)
	fmt.Printf("Evaluation: Won %d/%d games (%.1f%% win rate, avg %.1f points)\n",
		wins, numGames, winRate, avgPoints)
}

// declarerDiscard has declarer discard 2 cards to the Skat
func (t *Trainer) declarerDiscard(g *game.GameState) {
	declarer := g.Players[g.Declarer]

	// Simple strategy: discard 2 lowest-value non-jack cards
	type cardValue struct {
		card  game.Card
		value int
	}

	cards := make([]cardValue, len(declarer.Hand))
	for i, card := range declarer.Hand {
		val := card.Value()
		// Never discard jacks (they're trump in Grand)
		if card.Rank == game.Jack {
			val = 100 // High value to avoid discarding
		}
		cards[i] = cardValue{card, val}
	}

	// Sort by value (ascending)
	for i := 0; i < len(cards)-1; i++ {
		for j := i + 1; j < len(cards); j++ {
			if cards[i].value > cards[j].value {
				cards[i], cards[j] = cards[j], cards[i]
			}
		}
	}

	// Discard the 2 lowest value cards
	discard1 := cards[0].card
	discard2 := cards[1].card

	// Remove from hand
	newHand := make([]game.Card, 0, 10)
	discardCount := 0
	for _, card := range declarer.Hand {
		if discardCount < 2 && (card == discard1 || card == discard2) {
			discardCount++
			continue
		}
		newHand = append(newHand, card)
	}
	declarer.Hand = newHand

	// Points from discarded cards go to declarer at end
	// (simplified: we'll ignore this for now)
}
