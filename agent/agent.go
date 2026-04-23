package agent

import (
	"skat/game"
)

// Agent represents a player that can make decisions
type Agent interface {
	// SelectMove chooses a card to play from valid moves
	SelectMove(state *game.GameState, validMoves []game.Card) game.Card

	// Bid decides whether to accept the current bid (true) or pass (false)
	Bid(state *game.GameState) bool

	// ChooseGame selects the game mode after winning the bid
	ChooseGame(state *game.GameState) (game.GameMode, game.Suit)

	// Name returns the agent's identifier
	Name() string
}

// SkatAgent combines MCTS for card playing with Q-learning for bidding and game choice
type SkatAgent struct {
	name string

	// MCTS parameters for card playing
	simulations    int
	explorationC   float64
	deterministicC float64

	// Q-learning parameters for bidding
	qTable  map[int]map[int]float64
	alpha   float64 // Learning rate
	gamma   float64 // Discount factor
	Epsilon float64 // Exploration rate (exported)

	// Q-learning for game mode selection
	gameChoiceQTable map[int]map[int]float64 // [handState][gameAction] -> Q-value
	gameChoiceAlpha  float64
	GameChoiceEpsilon float64 // Exploration rate for game choice

	// Track current episode (exported for trainer access)
	CurrentHandScore int
	CurrentBid       int
	CurrentGameChoice int // Encoded game mode + suit choice
}

func NewSkatAgent(name string, simulations int) *SkatAgent {
	return &SkatAgent{
		name:              name,
		simulations:       simulations,
		explorationC:      1.41,
		deterministicC:    10,
		qTable:            make(map[int]map[int]float64),
		alpha:             0.1,
		gamma:             0.9,
		Epsilon:           0.15,
		gameChoiceQTable:  make(map[int]map[int]float64),
		gameChoiceAlpha:   0.1,
		GameChoiceEpsilon: 0.15,
	}
}

func (sa *SkatAgent) Name() string {
	return sa.name
}

func (sa *SkatAgent) SetSimulations(sims int) {
	sa.simulations = sims
}
