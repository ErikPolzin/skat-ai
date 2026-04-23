package training

import (
	"fmt"
	"math/rand"
	"skat/agent"
	"skat/game"
)

// BiddingTrainer trains the bidding agent
type BiddingTrainer struct {
	biddingAgents [3]*agent.SkatAgent
	mctsAgents    [3]*agent.SkatAgent
}

func NewBiddingTrainer() *BiddingTrainer {
	return &BiddingTrainer{
		biddingAgents: [3]*agent.SkatAgent{
			agent.NewSkatAgent("Agent", 100),
			agent.NewSkatAgent("Agent", 100),
			agent.NewSkatAgent("Agent", 100),
		},
		mctsAgents: [3]*agent.SkatAgent{
			agent.NewSkatAgent("MCTS-1", 100),
			agent.NewSkatAgent("MCTS-2", 100),
			agent.NewSkatAgent("MCTS-3", 100),
		},
	}
}

// TrainBidding runs episodes to train bidding agents
func (bt *BiddingTrainer) TrainBidding(episodes int) {
	fmt.Printf("Training bidding agents for %d episodes...\n", episodes)

	wins := make([]int, 3)
	totalGames := make([]int, 3)

	gamesPlayed := 0
	for ep := 0; ep < episodes; ep++ {
		// Simple progress indicator every 100 episodes
		if (ep+1)%100 == 0 {
			fmt.Printf(".")
			if (ep+1)%1000 == 0 {
				fmt.Printf(" %d\n", ep+1)
			}
		}

		g := game.NewGame()

		// Initialize players for training
		for i := 0; i < 3; i++ {
			g.Players[i] = &game.PlayerState{
				ID:      fmt.Sprintf("player-%d", i),
				Name:    fmt.Sprintf("Player %d", i),
				Hand:    []game.Card{},
				IsAgent: true,
			}
		}

		// Deal cards
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
		// Skat (2 cards)
		g.Skat[0] = deck[30]
		g.Skat[1] = deck[31]

		// Conduct bidding
		declarer, finalBid := bt.runBidding(g)

		if declarer == -1 {
			// Everyone passed - episode still counts
			continue
		}

		gamesPlayed++

		g.Declarer = declarer
		declarerInt := int(declarer)

		// Use proper game flow for skat exchange
		g.Phase = game.PhaseSkatExchange
		g.CurrentPlayer = declarer

		// Declarer picks up skat
		if _, err := g.SkatDecision(true); err != nil {
			panic(fmt.Sprintf("SkatDecision error: %v", err))
		}

		// Declarer discards 2 cards
		card1, card2 := bt.getCardsToDiscard(g, declarer)
		if _, err := g.Discard(card1, card2); err != nil {
			panic(fmt.Sprintf("Discard error: %v", err))
		}

		// Now in PhaseDeclarerChoice - choose game mode
		mode, trump := bt.biddingAgents[declarer].ChooseGame(g)
		if _, err := g.DeclareGame(mode, trump); err != nil {
			// Agent bid too high - treat as automatic loss with heavy penalty
			declarerWon := false
			gameValue := g.BidValue

			// Update agent with heavy penalty for overbidding
			for i := 0; i < 3; i++ {
				becameDeclarer := i == int(declarer)
				bt.biddingAgents[i].OnGameEnd(becameDeclarer, declarerWon, gameValue, 0)
			}

			// Decay exploration
			for i := 0; i < 3; i++ {
				bt.biddingAgents[i].Epsilon = max(bt.biddingAgents[i].Epsilon*0.995, 0.01)
			}

			continue // Skip to next game
		}

		// Play the game using MCTS
		for g.Phase == game.PhasePlaying {
			validMoves := g.GetValidMoves()
			if len(validMoves) == 0 {
				break
			}

			move := bt.mctsAgents[g.CurrentPlayer].SelectMove(g, validMoves)
			if _, err := g.PlayCard(move); err != nil {
				panic(fmt.Sprintf("PlayCard error: %v", err))
			}

			// Resolve trick if complete
			if len(g.Trick) == 3 {
				if _, err := g.ResolveTrick(); err != nil {
					panic(fmt.Sprintf("ResolveTrick error: %v", err))
				}
			}
		}

		// Determine outcome
		declarerWon := g.DeclarerScore >= 61
		gameValue := finalBid

		if declarerWon {
			wins[declarer]++
		}
		totalGames[declarer]++

		// Update all bidding agents
		for i := 0; i < 3; i++ {
			becameDeclarer := i == declarerInt
			bt.biddingAgents[i].OnGameEnd(becameDeclarer, declarerWon, gameValue, g.DeclarerScore)
		}

		// Decay exploration (higher minimum for more exploration)
		for i := 0; i < 3; i++ {
			bt.biddingAgents[i].DecayEpsilon(0.15)
		}

		// Progress reporting - more frequent for long training
		reportInterval := 100
		if episodes > 1000 {
			reportInterval = 1000
		}

		if (ep+1)%reportInterval == 0 {
			fmt.Printf("Episode %d (games: %d): ", ep+1, gamesPlayed)
			for i := 0; i < 3; i++ {
				if totalGames[i] > 0 {
					winRate := float64(wins[i]) / float64(totalGames[i]) * 100
					fmt.Printf("P%d: %.1f%% (%d/%d) ", i, winRate, wins[i], totalGames[i])
				} else {
					fmt.Printf("P%d: - (0/0) ", i)
				}
			}
			qSize := bt.biddingAgents[0].GetQTableSize()
			eps := fmt.Sprintf("%.3f", bt.biddingAgents[0].Epsilon)
			fmt.Printf("Q-states: %d, ε: %s\n", qSize, eps)
		}
	}

	fmt.Println("\nBidding training complete!")
	fmt.Println("\nFinal win rates as declarer:")
	for i := 0; i < 3; i++ {
		if totalGames[i] > 0 {
			winRate := float64(wins[i]) / float64(totalGames[i]) * 100
			fmt.Printf("Player %d: %.1f%% (%d wins in %d games)\n", i, winRate, wins[i], totalGames[i])
		}
	}
}

// GetBiddingAgent returns a trained bidding agent
func (bt *BiddingTrainer) GetBiddingAgent(index int) *agent.SkatAgent {
	if index < 0 || index >= 3 {
		return nil
	}
	return bt.biddingAgents[index]
}

// runBidding conducts the bidding phase
// Returns (declarer index, final bid) or (-1, 0) if all passed
func (bt *BiddingTrainer) runBidding(g *game.GameState) (game.GamePosition, int) {
	// Use the game's bidding logic directly
	g.Phase = game.PhaseBidding
	g.CurrentPlayer = game.Speaker
	g.BidValue = 0
	g.ListenerPassed = false
	g.SpeakerPassed = false
	g.DealerPassed = false

	// Run bidding until 2+ players have passed
	maxBids := 100 // Prevent infinite loops
	for bidCount := 0; bidCount < maxBids; bidCount++ {
		currentAgent := bt.biddingAgents[g.CurrentPlayer]
		accept := currentAgent.Bid(g)

		// Make the bid in the game
		if _, err := g.Bid(accept); err != nil {
			panic(fmt.Sprintf("Bid error: %v", err))
		}

		// Check if bidding is complete
		if g.Phase != game.PhaseBidding {
			break
		}
	}

	// Check if all passed
	if g.Declarer == -1 {
		return -1, 0
	}

	return g.Declarer, g.BidValue
}

func (bt *BiddingTrainer) getCardsToDiscard(g *game.GameState, declarer game.GamePosition) (game.Card, game.Card) {
	player := g.Players[declarer]

	// Simple: discard 2 lowest value non-jacks
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

	// Bubble sort
	for i := 0; i < len(cards)-1; i++ {
		for j := i + 1; j < len(cards); j++ {
			if cards[i].value > cards[j].value {
				cards[i], cards[j] = cards[j], cards[i]
			}
		}
	}

	return cards[0].card, cards[1].card
}
