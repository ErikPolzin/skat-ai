package training

import (
	"fmt"
	"math/rand"
	"skat/agent"
	"skat/game"
)

// GameChoiceTrainer trains the game mode selection agent
type GameChoiceTrainer struct {
	agents      [3]*agent.SkatAgent
	randomAgent *agent.SkatAgent
}

func NewGameChoiceTrainer() *GameChoiceTrainer {
	return &GameChoiceTrainer{
		agents: [3]*agent.SkatAgent{
			agent.NewSkatAgent("Agent-0", 100),
			agent.NewSkatAgent("Agent-1", 100),
			agent.NewSkatAgent("Agent-2", 100),
		},
		randomAgent: agent.NewRandomAgent("Random"),
	}
}

// TrainGameChoice runs episodes where agents learn to choose optimal game modes
func (gct *GameChoiceTrainer) TrainGameChoice(episodes int) {
	// Run parallel training
	runParallelTraining(episodes, func() {
		gct.trainSingleEpisode()
	})

	fmt.Println("Training complete!")
}

// trainSingleEpisode runs one training episode
func (gct *GameChoiceTrainer) trainSingleEpisode() {
	g := initializeGameWithDeal()

	// Skip bidding - randomly assign declarer
	declarer := game.GamePosition(rand.Intn(3))
	g.Declarer = declarer
	g.Phase = game.PhaseSkatExchange
	g.CurrentPlayer = declarer

	// Agent chooses game mode first (this is what we're training!)
	currentAgent := gct.agents[declarer]
	mode, trumpSuit := currentAgent.ChooseGame(g)

	// Declarer picks up skat using game logic
	if _, err := g.SkatDecision(true); err != nil {
		panic(fmt.Sprintf("Skat pickup error: %v", err))
	}

	// Agent chooses which cards to discard based on chosen game mode
	hand := g.Players[declarer].Hand
	discard1, discard2 := currentAgent.ChooseSkatDiscard(hand, mode, trumpSuit)

	if _, err := g.Discard(discard1, discard2); err != nil {
		panic(fmt.Sprintf("Discard error: %v", err))
	}

	// Declare the game
	if _, err := g.DeclareGame(mode, trumpSuit); err != nil {
		// Agent chose invalid game or bid too high - automatic loss
		// Update the game choice strategy with failure
		if qStrat, ok := currentAgent.GetGameChoiceStrategy().(*agent.QLearningGameChoiceStrategy); ok {
			qStrat.OnGameChoiceEnd(false, 0)
		}
		return
	}

	// Play the game out with random card play
	playAgents := [3]*agent.SkatAgent{gct.randomAgent, gct.randomAgent, gct.randomAgent}
	PlayGameToCompletion(g, playAgents)

	// Update Q-values based on outcome
	declarerWon := g.DeclarerScore >= 61
	if qStrat, ok := currentAgent.GetGameChoiceStrategy().(*agent.QLearningGameChoiceStrategy); ok {
		qStrat.OnGameChoiceEnd(declarerWon, g.DeclarerScore)
		qStrat.DecayEpsilon(0.05)
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
