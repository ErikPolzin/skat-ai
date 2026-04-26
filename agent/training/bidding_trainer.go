package training

import (
	"fmt"
	"skat/agent"
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
	// Run parallel training
	runParallelTraining(episodes, bt.trainSingleEpisode)
	fmt.Println("Training complete!")
}

// trainSingleEpisode runs one training episode
func (bt *BiddingTrainer) trainSingleEpisode() {
	g := PlayFullGame(bt.agents[0], bt.agents[1], bt.agents[2])
	// All agents passed
	if g.Declarer == nil {
		return
	}
	// Update all bidding agents
	for i, pr := range g.PlayerResults() {
		if qStrat, ok := bt.agents[i].GetBiddingStrategy().(*agent.QLearningBiddingStrategy); ok {
			qStrat.OnGameEnd(pr)
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
