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
	// Create 3 agents with Q-learning bidding but heuristic game choice/play
	// This ensures agents learn bidding in a stable environment
	qBidding0 := agent.NewQLearningBiddingStrategy(0.15)
	qBidding1 := agent.NewQLearningBiddingStrategy(0.15)
	qBidding2 := agent.NewQLearningBiddingStrategy(0.15)

	// Share the same Q-table across all bidding strategies
	qBidding0.ShareQTable(qBidding1)
	qBidding0.ShareQTable(qBidding2)

	agent0 := agent.NewAgentWithStrategies(
		"Agent-0",
		qBidding0,
		&agent.HeuristicGameChoiceStrategy{},
		&agent.HeuristicCardPlayStrategy{},
	)
	agent1 := agent.NewAgentWithStrategies(
		"Agent-1",
		qBidding1,
		&agent.HeuristicGameChoiceStrategy{},
		&agent.HeuristicCardPlayStrategy{},
	)
	agent2 := agent.NewAgentWithStrategies(
		"Agent-2",
		qBidding2,
		&agent.HeuristicGameChoiceStrategy{},
		&agent.HeuristicCardPlayStrategy{},
	)

	return &BiddingTrainer{
		agents: [3]*agent.SkatAgent{agent0, agent1, agent2},
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
			qStrat.DecayEpsilon(0.01) // Sweet spot for convergence
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
