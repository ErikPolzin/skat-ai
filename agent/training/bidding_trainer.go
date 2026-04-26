package training

import (
	"fmt"
	"skat/agent"
	"skat/game"
	"sync/atomic"
)

// BiddingTrainer trains the bidding agent
type BiddingTrainer struct {
	agents [3]*agent.SkatAgent
}

func NewBiddingTrainer() *BiddingTrainer {
	return &BiddingTrainer{
		agents: [3]*agent.SkatAgent{
			agent.NewSkatAgent("Agent-0", 100),
			agent.NewSkatAgent("Agent-1", 100),
			agent.NewSkatAgent("Agent-2", 100),
		},
	}
}

// TrainBidding runs episodes to train bidding agents using all available CPUs
func (bt *BiddingTrainer) TrainBidding(episodes int) {
	var winsAtomic [3]atomic.Int64
	var totalGamesAtomic [3]atomic.Int64
	var gamesPlayedAtomic atomic.Int64

	// Run parallel training
	runParallelTraining(episodes, func() {
		bt.trainSingleEpisode(&winsAtomic, &totalGamesAtomic, &gamesPlayedAtomic)
	})

	// Collect final stats
	wins := [3]int{int(winsAtomic[0].Load()), int(winsAtomic[1].Load()), int(winsAtomic[2].Load())}
	totalGames := [3]int{int(totalGamesAtomic[0].Load()), int(totalGamesAtomic[1].Load()), int(totalGamesAtomic[2].Load())}
	gamesPlayed := int(gamesPlayedAtomic.Load())

	fmt.Printf("Training complete!\n")
	fmt.Printf("Games played: %d/%d (%.1f%% resulted in a declarer)\n",
		gamesPlayed, episodes, 100.0*float64(gamesPlayed)/float64(episodes))
	for i := 0; i < 3; i++ {
		if totalGames[i] > 0 {
			winRate := 100.0 * float64(wins[i]) / float64(totalGames[i])
			fmt.Printf("Agent %d: %d/%d wins (%.1f%%)\n", i, wins[i], totalGames[i], winRate)
		}
	}
}

// trainSingleEpisode runs one training episode
func (bt *BiddingTrainer) trainSingleEpisode(winsAtomic *[3]atomic.Int64, totalGamesAtomic *[3]atomic.Int64, gamesPlayedAtomic *atomic.Int64) {
	g := initializeGameWithDeal()

	// Conduct bidding
	declarer, finalBid := bt.runBidding(g)

	if declarer == -1 {
		// Everyone passed - episode still counts but no declarer
		return
	}

	gamesPlayedAtomic.Add(1)

	g.Declarer = &declarer
	declarerInt := int(declarer)

	// Use proper game flow for skat exchange
	g.Phase = game.PhaseSkatExchange
	g.CurrentPlayer = declarer

	// Declarer picks up skat
	if _, err := g.SkatDecision(true); err != nil {
		panic(fmt.Sprintf("SkatDecision error: %v", err))
	}

	// Agent chooses game mode first to determine discard
	mode, trump := bt.agents[declarer].ChooseGame(g)

	// Agent discards 2 cards based on chosen game
	hand := g.Players[declarer].Hand
	card1, card2 := bt.agents[declarer].ChooseSkatDiscard(hand, mode, trump)
	if _, err := g.Discard(card1, card2); err != nil {
		panic(fmt.Sprintf("Discard error: %v", err))
	}

	// Now declare the game (no announcements in training)
	if _, err := g.DeclareGame(mode, trump, false, false); err != nil {
		// Agent bid too high - treat as automatic loss with heavy penalty
		declarerWon := false
		gameValue := g.BidValue

		// Update agent with heavy penalty for overbidding
		for i := 0; i < 3; i++ {
			becameDeclarer := i == int(declarer)
			if qStrat, ok := bt.agents[i].GetBiddingStrategy().(*agent.QLearningBiddingStrategy); ok {
				qStrat.OnGameEnd(becameDeclarer, declarerWon, gameValue, 0)
				qStrat.SetEpsilon(max(qStrat.GetEpsilon()*0.995, 0.01))
			}
		}

		return
	}

	// Play the game using MCTS
	PlayGameToCompletion(g, bt.agents)

	// Determine outcome
	declarerWon := g.DeclarerScore >= 61
	gameValue := finalBid

	if declarerWon {
		winsAtomic[declarer].Add(1)
	}
	totalGamesAtomic[declarer].Add(1)

	// Update all bidding agents
	for i := 0; i < 3; i++ {
		becameDeclarer := i == declarerInt
		if qStrat, ok := bt.agents[i].GetBiddingStrategy().(*agent.QLearningBiddingStrategy); ok {
			qStrat.OnGameEnd(becameDeclarer, declarerWon, gameValue, g.DeclarerScore)
		}
	}

	// Decay exploration (higher minimum for more exploration)
	for i := 0; i < 3; i++ {
		if qStrat, ok := bt.agents[i].GetBiddingStrategy().(*agent.QLearningBiddingStrategy); ok {
			qStrat.DecayEpsilon(0.15)
		}
	}
}

// GetBiddingAgent returns a trained bidding agent
func (bt *BiddingTrainer) GetBiddingAgent(index int) *agent.SkatAgent {
	if index < 0 || index >= 3 {
		return nil
	}
	return bt.agents[index]
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
		currentAgent := bt.agents[g.CurrentPlayer]
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
	if g.Declarer == nil {
		return -1, 0
	}

	return *g.Declarer, g.BidValue
}
