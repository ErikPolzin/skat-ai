package agent

import (
	"fmt"
	"skat/agent/strategies"
	"skat/game"
)

// Re-export strategy types for backwards compatibility
type (
	RandomBiddingStrategy       = strategies.RandomBiddingStrategy
	HeuristicBiddingStrategy    = strategies.HeuristicBiddingStrategy
	QLearningBiddingStrategy    = strategies.QLearningBiddingStrategy
	RandomGameChoiceStrategy    = strategies.RandomGameChoiceStrategy
	HeuristicGameChoiceStrategy = strategies.HeuristicGameChoiceStrategy
	QLearningGameChoiceStrategy = strategies.QLearningGameChoiceStrategy
	RandomCardPlayStrategy      = strategies.RandomCardPlayStrategy
	HeuristicCardPlayStrategy   = strategies.HeuristicCardPlayStrategy
	MCTSCardPlayStrategy        = strategies.MCTSCardPlayStrategy
)

// Re-export constructor functions
var (
	NewQLearningBiddingStrategy       = strategies.NewQLearningBiddingStrategy
	NewQLearningGameChoiceStrategy    = strategies.NewQLearningGameChoiceStrategy
	NewMCTSCardPlayStrategyWithParams = strategies.NewMCTSCardPlayStrategyWithParams
)

// BiddingStrategy interface for bidding decisions
type BiddingStrategy interface {
	GetName() string
	ShouldBid(gs *game.GameState, hand []game.Card, currentBid int) bool
}

// GameChoiceStrategy interface for game choice decisions
type GameChoiceStrategy interface {
	GetName() string
	ChooseGame(hand []game.Card, bidValue int) (game.GameMode, game.Suit)
	ChooseSkatDiscard(hand []game.Card, mode game.GameMode, trumpSuit game.Suit) (game.Card, game.Card)
}

// CardPlayStrategy interface for card play decisions
type CardPlayStrategy interface {
	GetName() string
	SelectMove(gs *game.GameState, validMoves []game.Card) game.Card
}

// SkatAgent uses strategies for different aspects of play
type SkatAgent struct {
	name string

	// Strategies
	biddingStrategy    BiddingStrategy
	gameChoiceStrategy GameChoiceStrategy
	cardPlayStrategy   CardPlayStrategy
}

// Agent interface implementation

func (sa *SkatAgent) Bid(state *game.GameState) bool {
	playerPos := state.CurrentPlayer
	hand := state.Players[playerPos].Hand
	currentBid := int(state.BidValue)
	if currentBid == 0 {
		currentBid = 18
	}
	return sa.biddingStrategy.ShouldBid(state, hand, currentBid)
}

func (sa *SkatAgent) ChooseGame(state *game.GameState) (game.GameMode, game.Suit) {
	hand := state.Players[state.Declarer].Hand
	bidValue := int(state.BidValue)
	return sa.gameChoiceStrategy.ChooseGame(hand, bidValue)
}

func (sa *SkatAgent) SelectMove(state *game.GameState, validMoves []game.Card) game.Card {
	return sa.cardPlayStrategy.SelectMove(state, validMoves)
}

func (sa *SkatAgent) Name() string {
	return sa.name
}

// ChooseSkatDiscard selects which 2 cards to discard
func (sa *SkatAgent) ChooseSkatDiscard(hand []game.Card, mode game.GameMode, trumpSuit game.Suit) (game.Card, game.Card) {
	return sa.gameChoiceStrategy.ChooseSkatDiscard(hand, mode, trumpSuit)
}

// Constructors

// NewSkatAgent creates an agent with Q-learning for bidding/game choice and MCTS for card play
func NewSkatAgent(name string, simulations int) *SkatAgent {
	return &SkatAgent{
		name:               name,
		biddingStrategy:    NewQLearningBiddingStrategy(0.15),
		gameChoiceStrategy: NewQLearningGameChoiceStrategy(0.15),
		cardPlayStrategy:   NewMCTSCardPlayStrategyWithParams(simulations, 1.41, 10),
	}
}

// NewAgentWithStrategies creates an agent with custom strategies
func NewAgentWithStrategies(name string, bidding BiddingStrategy, gameChoice GameChoiceStrategy, cardPlay CardPlayStrategy) *SkatAgent {
	return &SkatAgent{
		name:               name,
		biddingStrategy:    bidding,
		gameChoiceStrategy: gameChoice,
		cardPlayStrategy:   cardPlay,
	}
}

// NewHeuristicAgent creates an agent using all heuristic strategies
func NewHeuristicAgent(name string) *SkatAgent {
	return &SkatAgent{
		name:               name,
		biddingStrategy:    &HeuristicBiddingStrategy{},
		gameChoiceStrategy: &HeuristicGameChoiceStrategy{},
		cardPlayStrategy:   &HeuristicCardPlayStrategy{},
	}
}

// NewRandomAgent creates an agent that makes random decisions
func NewRandomAgent(name string) *SkatAgent {
	return &SkatAgent{
		name:               name,
		biddingStrategy:    &RandomBiddingStrategy{},
		gameChoiceStrategy: &RandomGameChoiceStrategy{},
		cardPlayStrategy:   &RandomCardPlayStrategy{},
	}
}

// NewHybridAgent creates an agent with mixed strategies (for experimentation)
func NewHybridAgent(name string, biddingType, gameChoiceType, cardPlayType string, simulations int) *SkatAgent {
	agent := &SkatAgent{name: name}

	// Configure bidding strategy
	switch biddingType {
	case "heuristic":
		agent.biddingStrategy = &HeuristicBiddingStrategy{}
	case "qlearning":
		agent.biddingStrategy = NewQLearningBiddingStrategy(0.15)
	case "random":
		agent.biddingStrategy = &RandomBiddingStrategy{}
	default:
		agent.biddingStrategy = &HeuristicBiddingStrategy{}
	}

	// Configure game choice strategy
	switch gameChoiceType {
	case "heuristic":
		agent.gameChoiceStrategy = &HeuristicGameChoiceStrategy{}
	case "qlearning":
		agent.gameChoiceStrategy = NewQLearningGameChoiceStrategy(0.15)
	case "random":
		agent.gameChoiceStrategy = &RandomGameChoiceStrategy{}
	default:
		agent.gameChoiceStrategy = &HeuristicGameChoiceStrategy{}
	}

	// Configure card play strategy
	switch cardPlayType {
	case "heuristic":
		agent.cardPlayStrategy = &HeuristicCardPlayStrategy{}
	case "mcts":
		agent.cardPlayStrategy = NewMCTSCardPlayStrategyWithParams(simulations, 1.41, 10)
	case "random":
		agent.cardPlayStrategy = &RandomCardPlayStrategy{}
	default:
		agent.cardPlayStrategy = &HeuristicCardPlayStrategy{}
	}

	return agent
}

// Utility methods

// SetSimulations updates the simulation count for MCTS strategy
// Note: Currently not supported with new strategy architecture
func (sa *SkatAgent) SetSimulations(sims int) {
	// TODO: Add setter to MCTSCardPlayStrategy if needed
}

// GetStrategyNames returns a description of the strategies being used
func (sa *SkatAgent) GetStrategyNames() string {
	return fmt.Sprintf("Bid:%s, Game:%s, Play:%s",
		sa.biddingStrategy.GetName(),
		sa.gameChoiceStrategy.GetName(),
		sa.cardPlayStrategy.GetName())
}

// GetBiddingStrategy returns the bidding strategy (for Q-table I/O)
func (sa *SkatAgent) GetBiddingStrategy() BiddingStrategy {
	return sa.biddingStrategy
}

// GetGameChoiceStrategy returns the game choice strategy (for Q-table I/O)
func (sa *SkatAgent) GetGameChoiceStrategy() GameChoiceStrategy {
	return sa.gameChoiceStrategy
}

// GetCardPlayStrategy returns the card play strategy
func (sa *SkatAgent) GetCardPlayStrategy() CardPlayStrategy {
	return sa.cardPlayStrategy
}
