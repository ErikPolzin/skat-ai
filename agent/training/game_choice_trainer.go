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
	// Run parallel training
	runParallelTraining(episodes, gct.trainSingleEpisode)
	fmt.Println("Training complete!")
}

// trainSingleEpisode runs one training episode
func (gct *GameChoiceTrainer) trainSingleEpisode() {
	g := initializeGameWithDeal()
	// Skip bidding - randomly assign declarer
	declarer := game.GamePosition(rand.Intn(3))
	g.Declarer = &declarer
	g.Phase = game.PhaseSkatExchange
	g.CurrentPlayer = declarer
	// Agent chooses game mode first (this is what we're training!)
	currentAgent := gct.agents[declarer]
	// Play the game, using this agent to declare the game
	PlayGameToCompletion(g, gct.agents)
	results := g.PlayerResults()
	// Update Q-values based on outcome
	if qStrat, ok := currentAgent.GetGameChoiceStrategy().(*agent.QLearningGameChoiceStrategy); ok {
		qStrat.OnGameChoiceEnd(results[declarer])
		qStrat.DecayEpsilon(0.05)
	}
}

// GetGameChoiceAgent returns a trained game choice agent
func (gct *GameChoiceTrainer) GetGameChoiceAgent(index int) *agent.SkatAgent {
	if index < 0 || index >= 3 {
		return nil
	}
	return gct.agents[index]
}
