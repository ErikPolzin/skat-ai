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
			agent.NewSkatAgent("Agent", 50), // Reduced for faster training
			agent.NewSkatAgent("Agent", 50),
			agent.NewSkatAgent("Agent", 50),
		},
		mctsAgents: [3]*agent.SkatAgent{
			agent.NewSkatAgent("MCTS-1", 50), // Reduced for faster training
			agent.NewSkatAgent("MCTS-2", 50),
			agent.NewSkatAgent("MCTS-3", 50),
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

		// Declarer picks up skat and discards
		g.Players[declarer].Hand = append(g.Players[declarer].Hand, g.Skat[:]...)
		bt.discardCards(g, declarer)

		// Declarer chooses game mode based on hand
		g.Declarer = game.GamePosition(declarer)
		g.CurrentPlayer = game.GamePosition(declarer)
		g.Mode, g.TrumpSuit = bt.biddingAgents[declarer].ChooseGame(g)
		g.Phase = game.PhasePlaying
		// In Skat, forehand (player 0) leads first
		g.CurrentPlayer = 0

		// Play the game using MCTS
		for g.Phase == game.PhasePlaying {
			validMoves := g.GetValidMoves()
			if len(validMoves) == 0 {
				break
			}

			move := bt.mctsAgents[g.CurrentPlayer].SelectMove(g, validMoves)
			g.PlayCard("", move)
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
			wonBid := i == declarerInt || (i != declarerInt && bt.biddingAgents[i].CurrentBid > 0)
			becameDeclarer := i == declarerInt
			bt.biddingAgents[i].OnGameEnd(wonBid, becameDeclarer, declarerWon, gameValue, g.DeclarerScore)
		}

		// Decay exploration
		for i := 0; i < 3; i++ {
			bt.biddingAgents[i].DecayEpsilon(0.05)
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
	// Simplified Skat bidding: middlehand vs forehand, then winner vs rearhand
	// Players: 0=forehand, 1=middlehand, 2=rearhand

	currentBid := 17 // Minimum bid is 18

	// Phase 1: Middlehand (1) vs Forehand (0)
	middlehand := game.Listener
	forehand := game.Dealer
	active := [2]bool{true, true}

	for active[0] || active[1] {
		// Middlehand bids first
		if active[1] {
			g.CurrentPlayer = middlehand
			bid := bt.biddingAgents[middlehand].Bid(g, currentBid)
			if bid == 0 || bid <= currentBid {
				active[1] = false
			} else {
				currentBid = bid
			}
		}

		if !active[1] {
			break
		}

		// Forehand responds
		if active[0] {
			g.CurrentPlayer = forehand
			bid := bt.biddingAgents[forehand].Bid(g, currentBid)
			if bid == 0 || bid < currentBid {
				active[0] = false
			} else if bid > currentBid {
				currentBid = bid
			}
		}

		if !active[0] {
			break
		}

		// Prevent infinite loops
		if currentBid > 100 {
			break
		}
	}

	// Determine phase 1 winner
	var phase1Winner game.GamePosition
	if active[0] {
		phase1Winner = forehand
	} else if active[1] {
		phase1Winner = middlehand
	} else {
		// Both passed, check who bid last
		phase1Winner = forehand // Default
	}

	// Phase 2: Winner vs Rearhand (2)
	rearhand := game.Speaker
	active2 := [2]bool{true, true}

	for active2[0] && active2[1] {
		// Rearhand bids
		if active2[1] {
			g.CurrentPlayer = rearhand
			bid := bt.biddingAgents[rearhand].Bid(g, currentBid)
			if bid == 0 || bid <= currentBid {
				active2[1] = false
			} else {
				currentBid = bid
			}
		}

		if !active2[1] {
			break
		}

		// Phase 1 winner responds
		if active2[0] {
			g.CurrentPlayer = phase1Winner
			bid := bt.biddingAgents[phase1Winner].Bid(g, currentBid)
			if bid == 0 || bid < currentBid {
				active2[0] = false
			} else if bid > currentBid {
				currentBid = bid
			}
		}

		if !active2[0] {
			break
		}

		if currentBid > 100 {
			break
		}
	}

	// Determine final winner
	var finalWinner game.GamePosition
	if active2[0] {
		finalWinner = phase1Winner
	} else if active2[1] {
		finalWinner = rearhand
	} else {
		finalWinner = phase1Winner
	}

	// Check if everyone passed
	if currentBid == 17 {
		return -1, 0
	}

	return finalWinner, currentBid
}

func (bt *BiddingTrainer) discardCards(g *game.GameState, declarer game.GamePosition) {
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
