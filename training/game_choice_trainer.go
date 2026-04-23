package training

import (
	"fmt"
	"math/rand"
	"skat/agent"
	"skat/game"
)

// GameChoiceTrainer trains the game mode selection agent
type GameChoiceTrainer struct {
	agents [3]*agent.SkatAgent
}

func NewGameChoiceTrainer() *GameChoiceTrainer {
	return &GameChoiceTrainer{
		agents: [3]*agent.SkatAgent{
			agent.NewSkatAgent("Agent-0", 100),
			agent.NewSkatAgent("Agent-1", 100),
			agent.NewSkatAgent("Agent-2", 100),
		},
	}
}

// TrainGameChoice runs episodes where agents learn to choose optimal game modes
func (gct *GameChoiceTrainer) TrainGameChoice(episodes int) {
	fmt.Printf("Training game choice agents for %d episodes...\n", episodes)

	for ep := 0; ep < episodes; ep++ {
		if (ep+1)%100 == 0 {
			fmt.Printf(".")
			if (ep+1)%1000 == 0 {
				fmt.Printf(" %d\n", ep+1)
			}
		}

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

		// Deal cards manually
		deck := game.NewDeck()
		rand.Shuffle(len(deck), func(i, j int) {
			deck[i], deck[j] = deck[j], deck[i]
		})

		// Deal: 3-4-3 pattern to each player
		idx := 0
		for round := 0; round < 3; round++ {
			for p := 0; p < 3; p++ {
				count := 3
				if round == 1 {
					count = 4
				}
				for i := 0; i < count; i++ {
					g.Players[p].Hand = append(g.Players[p].Hand, deck[idx])
					idx++
				}
			}
		}

		// Put remaining 2 cards in skat
		g.Skat[0] = deck[idx]
		g.Skat[1] = deck[idx+1]

		// Skip bidding - randomly assign declarer
		declarer := game.GamePosition(rand.Intn(3))
		g.Declarer = declarer
		g.Phase = game.PhaseSkatExchange
		g.CurrentPlayer = declarer

		// Declarer picks up skat using game logic
		declarerPlayer := g.Players[declarer]
		if _, err := g.SkatDecision(true); err != nil {
			panic(fmt.Sprintf("Skat pickup error: %v", err))
		}

		// Discard 2 cards (simple heuristic: discard lowest value non-jacks)
		// Re-fetch player after skat pickup
		hand := g.Players[declarer].Hand
		if len(hand) != 12 {
			panic(fmt.Sprintf("Expected 12 cards after skat pickup, got %d", len(hand)))
		}
		sortedCards := make([]struct {
			card  game.Card
			value int
		}, len(hand))

		for i, c := range hand {
			val := c.Value()
			if c.Rank == game.Jack {
				val = 1000 // Never discard jacks
			}
			sortedCards[i] = struct {
				card  game.Card
				value int
			}{c, val}
		}

		// Sort by value (ascending)
		for i := 0; i < len(sortedCards); i++ {
			for j := i + 1; j < len(sortedCards); j++ {
				if sortedCards[j].value < sortedCards[i].value {
					sortedCards[i], sortedCards[j] = sortedCards[j], sortedCards[i]
				}
			}
		}

		// Discard 2 lowest
		discard1 := sortedCards[0].card
		discard2 := sortedCards[1].card

		if _, err := g.Discard(discard1, discard2); err != nil {
			panic(fmt.Sprintf("Discard error: %v", err))
		}

		// Agent chooses game mode (this is what we're training!)
		currentAgent := gct.agents[declarer]
		mode, trumpSuit := currentAgent.ChooseGame(g)

		// Store hand state for later learning
		handState := currentAgent.EvaluateHandWithSkat(declarerPlayer.Hand)

		// Declare the game
		if _, err := g.DeclareGame(mode, trumpSuit); err != nil {
			// Agent chose invalid game or bid too high - automatic loss
			currentAgent.OnGameChoiceEnd(handState, false, 0)
			continue
		}

		// Play the game out
		gct.playGame(g)

		// Update Q-values based on outcome
		declarerWon := g.DeclarerScore >= 61
		currentAgent.OnGameChoiceEnd(handState, declarerWon, g.DeclarerScore)

		// Decay exploration
		currentAgent.GameChoiceEpsilon = max(0.05, currentAgent.GameChoiceEpsilon*0.995)
	}

	fmt.Println("\nTraining complete!")
}

func (gct *GameChoiceTrainer) playGame(g *game.GameState) {
	maxTricks := 10
	tricksPlayed := 0

	for g.Phase == game.PhasePlaying && tricksPlayed < maxTricks {
		validMoves := g.GetValidMoves()

		if len(validMoves) == 0 {
			break
		}

		// Simple random play for training (focus is on game choice, not card play)
		move := validMoves[rand.Intn(len(validMoves))]
		g.PlayCard(move)

		if len(g.Trick) == 3 {
			g.ResolveTrick()
			tricksPlayed++
		}
	}
}

func (gct *GameChoiceTrainer) GetAgent(pos int) *agent.SkatAgent {
	return gct.agents[pos]
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
