package training

import (
	"fmt"
	"skat/agent"
)

// GameChoiceTrainer trains the game mode selection agent
type GameChoiceTrainer struct {
	agents [3]*agent.SkatAgent
}

func NewGameChoiceTrainer() *GameChoiceTrainer {
	// Create 3 agents with Q-learning game choice but heuristic bidding/play
	// This ensures agents learn game choice in a stable environment
	// Higher epsilon (0.3) encourages more exploration of suit games vs Grand
	qChoice0 := agent.NewQLearningGameChoiceStrategy(0.3)
	qChoice1 := agent.NewQLearningGameChoiceStrategy(0.3)
	qChoice2 := agent.NewQLearningGameChoiceStrategy(0.3)

	// Share the same Q-table across all game choice strategies
	qChoice0.ShareQTable(qChoice1)
	qChoice0.ShareQTable(qChoice2)

	agent0 := agent.NewAgentWithStrategies(
		"Agent-0",
		&agent.HeuristicBiddingStrategy{},
		qChoice0,
		&agent.HeuristicCardPlayStrategy{},
	)
	agent1 := agent.NewAgentWithStrategies(
		"Agent-1",
		&agent.HeuristicBiddingStrategy{},
		qChoice1,
		&agent.HeuristicCardPlayStrategy{},
	)
	agent2 := agent.NewAgentWithStrategies(
		"Agent-2",
		&agent.HeuristicBiddingStrategy{},
		qChoice2,
		&agent.HeuristicCardPlayStrategy{},
	)

	return &GameChoiceTrainer{
		agents: [3]*agent.SkatAgent{agent0, agent1, agent2},
	}
}

// NewGameChoiceTrainerWithQLearningBidding creates a trainer with Q-learning bidding
// This allows game choice to train with the same bidding strategy it will use in production
func NewGameChoiceTrainerWithQLearningBidding(biddingQTable map[int]map[int]float64) *GameChoiceTrainer {
	// Create Q-learning bidding strategies with the trained Q-table
	qBidding0 := agent.NewQLearningBiddingStrategy(0.0) // No exploration, use trained policy
	qBidding1 := agent.NewQLearningBiddingStrategy(0.0)
	qBidding2 := agent.NewQLearningBiddingStrategy(0.0)

	qBidding0.SetQTable(biddingQTable)
	qBidding1.SetQTable(biddingQTable)
	qBidding2.SetQTable(biddingQTable)

	// Create Q-learning game choice strategies (these will be trained)
	// Higher epsilon (0.3) encourages more exploration of suit games vs Grand
	qChoice0 := agent.NewQLearningGameChoiceStrategy(0.3)
	qChoice1 := agent.NewQLearningGameChoiceStrategy(0.3)
	qChoice2 := agent.NewQLearningGameChoiceStrategy(0.3)

	// Share the same Q-table across all game choice strategies
	qChoice0.ShareQTable(qChoice1)
	qChoice0.ShareQTable(qChoice2)

	agent0 := agent.NewAgentWithStrategies(
		"Agent-0",
		qBidding0,
		qChoice0,
		&agent.HeuristicCardPlayStrategy{},
	)
	agent1 := agent.NewAgentWithStrategies(
		"Agent-1",
		qBidding1,
		qChoice1,
		&agent.HeuristicCardPlayStrategy{},
	)
	agent2 := agent.NewAgentWithStrategies(
		"Agent-2",
		qBidding2,
		qChoice2,
		&agent.HeuristicCardPlayStrategy{},
	)

	return &GameChoiceTrainer{
		agents: [3]*agent.SkatAgent{agent0, agent1, agent2},
	}
}

// TrainGameChoice runs episodes where agents learn to choose optimal game modes
func (gct *GameChoiceTrainer) TrainGameChoice(episodes int) {
	// Run parallel training
	runParallelTraining(episodes, gct.trainSingleEpisode)
	fmt.Println("Training complete!")
}

// trainSingleEpisode runs one training episode
func (gct *GameChoiceTrainer) trainSingleEpisode() {
	// Play a full game INCLUDING bidding
	g := PlayFullGame(gct.agents[0], gct.agents[1], gct.agents[2])

	// All agents passed, no game to learn from
	if g.Declarer == nil {
		return
	}

	results := g.PlayerResults()
	declarer := *g.Declarer

	// Update Q-values for the declarer's game choice strategy
	if qStrat, ok := gct.agents[declarer].GetGameChoiceStrategy().(*agent.QLearningGameChoiceStrategy); ok {
		qStrat.OnGameChoiceEnd(results[declarer])
		qStrat.DecayEpsilon(0.01) // Sweet spot for convergence
	}
}

// GetGameChoiceAgent returns a trained game choice agent
func (gct *GameChoiceTrainer) GetGameChoiceAgent(index int) *agent.SkatAgent {
	if index < 0 || index >= 3 {
		return nil
	}
	return gct.agents[index]
}
