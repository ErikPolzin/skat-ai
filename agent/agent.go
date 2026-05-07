package agent

import (
	"fmt"
	"skat/agent/strategies"
	"skat/game"
	"sync"
	"sync/atomic"
)

// Re-export strategy types for backwards compatibility
type (
	RandomBiddingStrategy               = strategies.RandomBiddingStrategy
	HeuristicBiddingStrategy            = strategies.HeuristicBiddingStrategy
	WeightedHeuristicBiddingStrategy    = strategies.WeightedHeuristicBiddingStrategy
	RandomGameChoiceStrategy            = strategies.RandomGameChoiceStrategy
	HeuristicGameChoiceStrategy         = strategies.HeuristicGameChoiceStrategy
	WeightedHeuristicGameChoiceStrategy = strategies.WeightedHeuristicGameChoiceStrategy
	RandomCardPlayStrategy              = strategies.RandomCardPlayStrategy
	HeuristicCardPlayStrategy           = strategies.HeuristicCardPlayStrategy
	MCTSCardPlayStrategy                = strategies.MCTSCardPlayStrategy
)

// Re-export constructor functions
var (
	NewMCTSCardPlayStrategyWithParams      = strategies.NewMCTSCardPlayStrategyWithParams
	NewHeuristicCardPlayStrategy           = strategies.NewHeuristicCardPlayStrategy
	NewWeightedHeuristicBiddingStrategy    = strategies.NewWeightedHeuristicBiddingStrategy
	NewWeightedHeuristicGameChoiceStrategy = strategies.NewWeightedHeuristicGameChoiceStrategy
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

// AgentMetrics tracks performance metrics for an agent
type AgentMetrics struct {
	// Declarer metrics (when agent is declarer)
	wins       atomic.Int64
	games      atomic.Int64
	points     atomic.Int64
	overbid    atomic.Int64
	grandGames atomic.Int64
	grandWins  atomic.Int64
	suitGames  atomic.Int64
	suitWins   atomic.Int64
	nullGames  atomic.Int64
	nullWins   atomic.Int64

	// Defender metrics (when agent is defending)
	defenderGames atomic.Int64
	defenderWins  atomic.Int64

	// Bidding metrics
	mu                   sync.Mutex
	biddingAccepts       map[int]int  // bid value -> count of accepts
	biddingRejects       map[int]int  // bid value -> count of rejects
	passedGames          atomic.Int64 // games where all players passed
	passedGamesWon       atomic.Int64 // Zwangsspiel games won as declarer
	predictedProbability []float64    // predicted win probabilities for games won
	actualOutcomes       []bool       // actual outcomes (true = won, false = lost)
}

// SkatAgent uses strategies for different aspects of play
type SkatAgent struct {
	name string

	// Strategies
	biddingStrategy    BiddingStrategy
	gameChoiceStrategy GameChoiceStrategy
	cardPlayStrategy   CardPlayStrategy

	// Metrics
	metrics *AgentMetrics

	// Cached clone for performance (lazily created on first CachedClone() call)
	cachedClone *SkatAgent

	// Last predicted win probability (for calibration tracking)
	lastPredictedWinProb float64
	mu                   sync.Mutex
}

// Agent interface implementation

func (sa *SkatAgent) Bid(state *game.GameState) bool {
	playerPos := state.CurrentPlayer
	hand := state.Players[playerPos].Hand
	currentBid := int(state.BidValue)
	if currentBid == 0 {
		currentBid = 18
	}
	accept := sa.biddingStrategy.ShouldBid(state, hand, currentBid)

	// Record bidding decision in metrics
	if sa.metrics != nil {
		sa.metrics.mu.Lock()
		if accept {
			sa.metrics.biddingAccepts[currentBid]++
		} else {
			sa.metrics.biddingRejects[currentBid]++
		}
		sa.metrics.mu.Unlock()
	}

	return accept
}

func (sa *SkatAgent) ChooseGame(state *game.GameState) (game.GameMode, game.Suit) {
	if state.Declarer == nil {
		return game.ModeGrand, game.Clubs // Default fallback
	}
	hand := state.Players[*state.Declarer].Hand
	bidValue := int(state.BidValue)
	mode, suit := sa.gameChoiceStrategy.ChooseGame(hand, bidValue)

	// Calculate predicted win probability for the chosen game type
	if weightedStrat, ok := sa.biddingStrategy.(*WeightedHeuristicBiddingStrategy); ok {
		winProb := weightedStrat.EvaluateGameProbability(game.Cards(hand), mode, suit)
		sa.mu.Lock()
		sa.lastPredictedWinProb = winProb
		sa.mu.Unlock()
	}

	return mode, suit
}

func (sa *SkatAgent) SelectMove(state *game.GameState, validMoves []game.Card) game.Card {
	return sa.cardPlayStrategy.SelectMove(state, validMoves)
}

func (sa *SkatAgent) Name() string {
	return sa.name
}

// Clone creates a copy of the agent with cloned neural strategies (if any).
// Metrics are NOT copied - each clone starts with fresh metrics that need to be
// enabled separately if needed.
func (sa *SkatAgent) Clone() *SkatAgent {
	clone := &SkatAgent{
		name: sa.name,
	}

	// Bidding strategy is typically shared (heuristic or Q-learning)
	clone.biddingStrategy = sa.biddingStrategy

	// Game choice strategy is typically shared (heuristic)
	clone.gameChoiceStrategy = sa.gameChoiceStrategy

	// Clone card play strategy if neural (to avoid mutex contention on VM)
	if neuralCard, ok := sa.cardPlayStrategy.(*strategies.NeuralCardPlayStrategy); ok {
		clone.cardPlayStrategy = neuralCard.Clone()
	} else {
		clone.cardPlayStrategy = sa.cardPlayStrategy // Share strategy if not cloneable
	}

	// Metrics are NOT copied - clone starts with nil metrics
	// Caller should call EnableMetrics() if metrics collection is needed
	clone.metrics = nil

	return clone
}

// CachedClone returns a cached clone of the agent, creating it lazily on first call.
// This is much faster than Clone() when called repeatedly, as it reuses the same clone.
// Use this for performance in tight loops where you need multiple clones.
func (sa *SkatAgent) CachedClone() *SkatAgent {
	if sa.cachedClone == nil {
		sa.cachedClone = sa.Clone()
	}
	return sa.cachedClone
}

// ChooseSkatDiscard selects which 2 cards to discard
func (sa *SkatAgent) ChooseSkatDiscard(hand []game.Card, mode game.GameMode, trumpSuit game.Suit) (game.Card, game.Card) {
	return sa.gameChoiceStrategy.ChooseSkatDiscard(hand, mode, trumpSuit)
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
		biddingStrategy:    strategies.NewHeuristicBiddingStrategy(),
		gameChoiceStrategy: &HeuristicGameChoiceStrategy{},
		cardPlayStrategy:   NewHeuristicCardPlayStrategy(),
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

// HybridAgentConfig holds configuration for creating hybrid agents
type HybridAgentConfig struct {
	BiddingType        string
	BiddingThreshold   float64                 // For weighted heuristic bidding
	BiddingWeightsPath string                  // Path to trained weights JSON file (shared by bidding and game choice)
	BiddingQTable      map[int]map[int]float64 // For Q-learning bidding

	GameChoiceType   string
	GameChoiceQTable map[int]map[int]float64 // For Q-learning game choice

	CardPlayType      string
	MCTSSimulations   int    // For MCTS card play
	MinimaxDepth      int    // For minimax card play
	NeuralWeightsPath string // For neural network card play (combined declarer+defender weights)
}

// NewHybridAgent creates an agent with mixed strategies (for experimentation)
func NewHybridAgent(name string, config HybridAgentConfig) (*SkatAgent, error) {
	agent := &SkatAgent{name: name}

	// Configure bidding strategy
	switch config.BiddingType {
	case "heuristic":
		if config.BiddingThreshold == 0 {
			agent.biddingStrategy = strategies.NewHeuristicBiddingStrategy()
		} else {
			agent.biddingStrategy = strategies.NewHeuristicBiddingStrategyWithThreshold(config.BiddingThreshold)
		}
	case "weighted":
		var weighted *strategies.WeightedHeuristicBiddingStrategy
		var err error

		// Load weights from file if provided
		if config.BiddingWeightsPath != "" {
			weighted, err = strategies.NewWeightedHeuristicBiddingStrategyFromFile(config.BiddingWeightsPath)
			if err != nil {
				return nil, fmt.Errorf("load bidding weights: %w", err)
			}
		} else {
			weighted = strategies.NewWeightedHeuristicBiddingStrategy()
		}
		// Override threshold if specified
		if config.BiddingThreshold != 0 {
			weighted.SetBiddingThreshold(config.BiddingThreshold)
		}
		agent.biddingStrategy = weighted
	case "random":
		agent.biddingStrategy = &RandomBiddingStrategy{}
	default:
		// Default to weighted heuristic
		weighted := strategies.NewWeightedHeuristicBiddingStrategy()
		agent.biddingStrategy = weighted
	}

	// Configure game choice strategy
	switch config.GameChoiceType {
	case "heuristic":
		agent.gameChoiceStrategy = &HeuristicGameChoiceStrategy{}
	case "weighted":
		// Use the same weights file as bidding if provided
		if config.BiddingWeightsPath != "" {
			weights, err := strategies.LoadBidWeights(config.BiddingWeightsPath)
			if err != nil {
				return nil, fmt.Errorf("load weights for game choice: %w", err)
			}
			agent.gameChoiceStrategy = strategies.NewWeightedHeuristicGameChoiceStrategyWithWeights(weights)
		} else {
			agent.gameChoiceStrategy = strategies.NewWeightedHeuristicGameChoiceStrategy()
		}
	case "random":
		agent.gameChoiceStrategy = &RandomGameChoiceStrategy{}
	default:
		agent.gameChoiceStrategy = &HeuristicGameChoiceStrategy{}
	}

	// Configure card play strategy
	switch config.CardPlayType {
	case "heuristic":
		agent.cardPlayStrategy = NewHeuristicCardPlayStrategy()
	case "mcts":
		simulations := config.MCTSSimulations
		if simulations == 0 {
			simulations = 500 // Default simulations
		}
		agent.cardPlayStrategy = NewMCTSCardPlayStrategyWithParams(simulations, 1.41, 10)
	case "minimax":
		depth := config.MinimaxDepth
		if depth == 0 {
			depth = 7 // Default depth
		}
		agent.cardPlayStrategy = strategies.NewPerfectInfoMinimaxStrategyWithDepth(depth)
	case "neural":
		if config.NeuralWeightsPath == "" {
			return nil, fmt.Errorf("neural card play requires weight path")
		}
		neuralStrategy, err := strategies.NewNeuralCardPlayStrategyFromWeights(config.NeuralWeightsPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load neural weights: %w", err)
		}
		neuralStrategy.SetExploration(0.0) // No exploration for evaluation
		agent.cardPlayStrategy = neuralStrategy
	case "random":
		agent.cardPlayStrategy = &RandomCardPlayStrategy{}
	default:
		agent.cardPlayStrategy = NewHeuristicCardPlayStrategy()
	}

	return agent, nil
}

// Utility methods

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

// OnTrickComplete notifies the card play strategy that a trick was completed
// This allows strategies with memory (like HeuristicCardPlayStrategy) to track played cards
func (sa *SkatAgent) OnTrickComplete(trick []game.Card) {
	// Check if the strategy supports trick tracking
	if tracker, ok := sa.cardPlayStrategy.(interface {
		OnTrickComplete([]game.Card)
	}); ok {
		tracker.OnTrickComplete(trick)
	}
}

// OnGameStart resets any stateful strategies for a new game
func (sa *SkatAgent) OnGameStart() {
	// Check if the strategy supports reset
	if resettable, ok := sa.cardPlayStrategy.(interface{ Reset() }); ok {
		resettable.Reset()
	}
}

// Metrics methods

// EnableMetrics creates and enables metrics collection for this agent
func (sa *SkatAgent) EnableMetrics() {
	sa.metrics = &AgentMetrics{
		biddingAccepts: make(map[int]int),
		biddingRejects: make(map[int]int),
	}
}

// RecordPassedGame records when all players passed (no one bid)
func (sa *SkatAgent) RecordPassedGame() {
	if sa.metrics == nil {
		return
	}
	sa.metrics.passedGames.Add(1)
}

// RecordGameResult records the result of a game for this agent (declarer or defender)
func (sa *SkatAgent) RecordGameResult(gs *game.GameState, playerResult game.PlayerResultState) {
	if sa.metrics == nil {
		return
	}

	if playerResult.IsDeclarer {
		// Check if this is a Zwangsspiel (all players passed)
		isZwangsspiel := gs.IsZwangsspiel()

		// Declarer metrics
		sa.metrics.games.Add(1)
		sa.metrics.points.Add(int64(playerResult.PlayerPoints))

		if playerResult.IsWinner {
			sa.metrics.wins.Add(1)
			if isZwangsspiel {
				sa.metrics.passedGamesWon.Add(1)
			}
		}

		if playerResult.IsOverbid {
			sa.metrics.overbid.Add(1)
		}

		// Record predicted vs actual outcome for calibration
		sa.mu.Lock()
		lastProb := sa.lastPredictedWinProb
		sa.lastPredictedWinProb = 0.0 // Reset for next game
		sa.mu.Unlock()

		if lastProb > 0.0 {
			sa.metrics.mu.Lock()
			sa.metrics.predictedProbability = append(sa.metrics.predictedProbability, lastProb)
			sa.metrics.actualOutcomes = append(sa.metrics.actualOutcomes, playerResult.IsWinner)
			sa.metrics.mu.Unlock()
		}

		// Track by game type
		switch gs.Mode {
		case game.ModeGrand:
			sa.metrics.grandGames.Add(1)
			if playerResult.IsWinner {
				sa.metrics.grandWins.Add(1)
			}
		case game.ModeSuit:
			sa.metrics.suitGames.Add(1)
			if playerResult.IsWinner {
				sa.metrics.suitWins.Add(1)
			}
		case game.ModeNull:
			sa.metrics.nullGames.Add(1)
			if playerResult.IsWinner {
				sa.metrics.nullWins.Add(1)
			}
		}
	} else {
		// Defender metrics
		sa.metrics.defenderGames.Add(1)
		if playerResult.IsWinner {
			sa.metrics.defenderWins.Add(1)
		}
	}
}

// MergeMetrics adds metrics from another agent to this agent
func (sa *SkatAgent) MergeMetrics(other AgentMetricsSnapshot) {
	if sa.metrics == nil {
		return
	}

	sa.metrics.wins.Add(other.Wins)
	sa.metrics.games.Add(other.Games)
	sa.metrics.points.Add(other.Points)
	sa.metrics.overbid.Add(other.Overbid)
	sa.metrics.grandGames.Add(other.GrandGames)
	sa.metrics.grandWins.Add(other.GrandWins)
	sa.metrics.suitGames.Add(other.SuitGames)
	sa.metrics.suitWins.Add(other.SuitWins)
	sa.metrics.nullGames.Add(other.NullGames)
	sa.metrics.nullWins.Add(other.NullWins)
	sa.metrics.defenderGames.Add(other.DefenderGames)
	sa.metrics.defenderWins.Add(other.DefenderWins)
	sa.metrics.passedGames.Add(other.PassedGames)

	// Merge bidding distributions
	sa.metrics.mu.Lock()
	for bid, count := range other.BiddingAccepts {
		sa.metrics.biddingAccepts[bid] += count
	}
	for bid, count := range other.BiddingRejects {
		sa.metrics.biddingRejects[bid] += count
	}
	sa.metrics.mu.Unlock()
}

// GetMetrics returns a snapshot of the agent's metrics
func (sa *SkatAgent) GetMetrics() AgentMetricsSnapshot {
	if sa.metrics == nil {
		return AgentMetricsSnapshot{
			BiddingAccepts: make(map[int]int),
			BiddingRejects: make(map[int]int),
		}
	}

	sa.metrics.mu.Lock()
	defer sa.metrics.mu.Unlock()

	// Copy bidding maps to avoid race conditions
	biddingAccepts := make(map[int]int)
	biddingRejects := make(map[int]int)
	for k, v := range sa.metrics.biddingAccepts {
		biddingAccepts[k] = v
	}
	for k, v := range sa.metrics.biddingRejects {
		biddingRejects[k] = v
	}

	// Copy calibration data
	predictedProb := make([]float64, len(sa.metrics.predictedProbability))
	copy(predictedProb, sa.metrics.predictedProbability)
	actualOutcomes := make([]bool, len(sa.metrics.actualOutcomes))
	copy(actualOutcomes, sa.metrics.actualOutcomes)

	return AgentMetricsSnapshot{
		Wins:                 sa.metrics.wins.Load(),
		Games:                sa.metrics.games.Load(),
		Points:               sa.metrics.points.Load(),
		Overbid:              sa.metrics.overbid.Load(),
		GrandGames:           sa.metrics.grandGames.Load(),
		GrandWins:            sa.metrics.grandWins.Load(),
		SuitGames:            sa.metrics.suitGames.Load(),
		SuitWins:             sa.metrics.suitWins.Load(),
		NullGames:            sa.metrics.nullGames.Load(),
		NullWins:             sa.metrics.nullWins.Load(),
		DefenderGames:        sa.metrics.defenderGames.Load(),
		DefenderWins:         sa.metrics.defenderWins.Load(),
		BiddingAccepts:       biddingAccepts,
		BiddingRejects:       biddingRejects,
		PassedGames:          sa.metrics.passedGames.Load(),
		PassedGamesWon:       sa.metrics.passedGamesWon.Load(),
		PredictedProbability: predictedProb,
		ActualOutcomes:       actualOutcomes,
	}
}

// ResetMetrics clears all collected metrics
func (sa *SkatAgent) ResetMetrics() {
	if sa.metrics == nil {
		return
	}

	sa.metrics.wins.Store(0)
	sa.metrics.games.Store(0)
	sa.metrics.points.Store(0)
	sa.metrics.overbid.Store(0)
	sa.metrics.grandGames.Store(0)
	sa.metrics.grandWins.Store(0)
	sa.metrics.suitGames.Store(0)
	sa.metrics.suitWins.Store(0)
	sa.metrics.nullGames.Store(0)
	sa.metrics.nullWins.Store(0)
	sa.metrics.defenderGames.Store(0)
	sa.metrics.defenderWins.Store(0)
	sa.metrics.passedGames.Store(0)

	sa.metrics.mu.Lock()
	sa.metrics.biddingAccepts = make(map[int]int)
	sa.metrics.biddingRejects = make(map[int]int)
	sa.metrics.mu.Unlock()
}

// AgentMetricsSnapshot is a point-in-time snapshot of agent metrics
type AgentMetricsSnapshot struct {
	Wins                 int64
	Games                int64
	Points               int64
	Overbid              int64
	GrandGames           int64
	GrandWins            int64
	SuitGames            int64
	SuitWins             int64
	NullGames            int64
	NullWins             int64
	DefenderGames        int64
	DefenderWins         int64
	BiddingAccepts       map[int]int
	BiddingRejects       map[int]int
	PassedGames          int64
	PassedGamesWon       int64
	PredictedProbability []float64
	ActualOutcomes       []bool
}

// GetMaxBid returns the highest bid value the agent accepted during evaluation
func (m AgentMetricsSnapshot) GetMaxBid() int {
	maxBid := 0
	for bid := range m.BiddingAccepts {
		if bid > maxBid {
			maxBid = bid
		}
	}
	return maxBid
}

// GetTotalBids returns the total number of bidding decisions made
func (m AgentMetricsSnapshot) GetTotalBids() int {
	total := 0
	for _, count := range m.BiddingAccepts {
		total += count
	}
	for _, count := range m.BiddingRejects {
		total += count
	}
	return total
}
