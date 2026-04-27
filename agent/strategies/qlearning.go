package strategies

import (
	"math"
	"math/rand"
	"skat/game"
	"sync"
)

// QTable represents a Q-learning table
type QTable struct {
	table map[int]map[int]float64
	alpha float64 // Learning rate
	gamma float64 // Discount factor
	mu    sync.RWMutex
}

// NewQTable creates a new Q-learning table
func NewQTable(alpha, gamma float64) *QTable {
	return &QTable{
		table: make(map[int]map[int]float64),
		alpha: alpha,
		gamma: gamma,
	}
}

// Get returns the Q-value for a state-action pair
func (qt *QTable) Get(state, action int) float64 {
	qt.mu.RLock()
	defer qt.mu.RUnlock()
	if qt.table[state] == nil {
		return 0.0
	}
	return qt.table[state][action]
}

// Set updates the Q-value for a state-action pair
func (qt *QTable) Set(state, action int, value float64) {
	qt.mu.Lock()
	defer qt.mu.Unlock()
	if qt.table[state] == nil {
		qt.table[state] = make(map[int]float64)
	}
	qt.table[state][action] = value
}

// Update applies Q-learning update rule
func (qt *QTable) Update(state, action int, reward float64) {
	qt.mu.Lock()
	defer qt.mu.Unlock()
	if qt.table[state] == nil {
		qt.table[state] = make(map[int]float64)
	}
	oldQ := qt.table[state][action]
	qt.table[state][action] = oldQ + qt.alpha*(reward-oldQ)
}

// Size returns the total number of state-action pairs learned
func (qt *QTable) Size() int {
	qt.mu.RLock()
	defer qt.mu.RUnlock()
	total := 0
	for _, actions := range qt.table {
		total += len(actions)
	}
	return total
}

// GetTable returns the underlying Q-table for serialization
func (qt *QTable) GetTable() map[int]map[int]float64 {
	qt.mu.RLock()
	defer qt.mu.RUnlock()
	// Return a copy to avoid race conditions
	tableCopy := make(map[int]map[int]float64, len(qt.table))
	for state, actions := range qt.table {
		tableCopy[state] = make(map[int]float64, len(actions))
		for action, value := range actions {
			tableCopy[state][action] = value
		}
	}
	return tableCopy
}

// SetTable sets the underlying Q-table (for deserialization)
func (qt *QTable) SetTable(table map[int]map[int]float64) {
	qt.mu.Lock()
	defer qt.mu.Unlock()
	qt.table = table
}

// ExplorationSchedule manages epsilon decay for exploration
type ExplorationSchedule struct {
	epsilon    float64
	minEpsilon float64
	decayRate  float64
	mu         sync.RWMutex
}

// NewExplorationSchedule creates a new exploration schedule
func NewExplorationSchedule(initialEpsilon, minEpsilon, decayRate float64) *ExplorationSchedule {
	return &ExplorationSchedule{
		epsilon:    initialEpsilon,
		minEpsilon: minEpsilon,
		decayRate:  decayRate,
	}
}

// Get returns the current epsilon value
func (es *ExplorationSchedule) Get() float64 {
	es.mu.RLock()
	defer es.mu.RUnlock()
	return es.epsilon
}

// Decay reduces epsilon according to the decay schedule
func (es *ExplorationSchedule) Decay() {
	es.mu.Lock()
	defer es.mu.Unlock()
	es.epsilon = math.Max(es.minEpsilon, es.epsilon*es.decayRate)
}

// Set sets epsilon to a specific value
func (es *ExplorationSchedule) Set(value float64) {
	es.mu.Lock()
	defer es.mu.Unlock()
	es.epsilon = value
}

// EncodeGameAction encodes game mode + suit choice into a single integer
// Grand: 0, Suit Clubs: 1, Suit Spades: 2, Suit Hearts: 3, Suit Diamonds: 4
// Note: Suit enum is NoSuit=0, Clubs=1, Spades=2, Hearts=3, Diamonds=4
func EncodeGameAction(mode game.GameMode, suit game.Suit) int {
	if mode == game.ModeGrand {
		return 0
	}
	return int(suit) // Suit values already match: Clubs=1, Spades=2, Hearts=3, Diamonds=4
}

// DecodeGameAction decodes an integer action into game mode and suit
func DecodeGameAction(action int) (game.GameMode, game.Suit) {
	if action == 0 {
		return game.ModeGrand, game.NoSuit // Suit doesn't matter for Grand
	}
	return game.ModeSuit, game.Suit(action)
}

// StrategyMetrics tracks strategy performance metrics
type StrategyMetrics struct {
	enabled        bool
	unseenStates   int // Count of states not in Q-table
	totalDecisions int // Total number of decisions made
}

// Track records a decision and checks if the state was in the Q-table
func (m *StrategyMetrics) Track(state int, qtable map[int]map[int]float64) {
	if !m.enabled {
		return
	}
	m.totalDecisions++
	if _, exists := qtable[state]; !exists {
		m.unseenStates++
	}
}

// Get returns the current metrics
func (m *StrategyMetrics) Get() (unseenStates, totalDecisions int) {
	return m.unseenStates, m.totalDecisions
}

// Reset clears the metrics counters
func (m *StrategyMetrics) Reset() {
	m.unseenStates = 0
	m.totalDecisions = 0
}

// Enable turns on metrics collection
func (m *StrategyMetrics) Enable() {
	m.enabled = true
}

// Disable turns off metrics collection
func (m *StrategyMetrics) Disable() {
	m.enabled = false
}

// QLearningBiddingStrategy uses Q-learning for bidding decisions
type QLearningBiddingStrategy struct {
	qTable            *QTable
	epsilon           *ExplorationSchedule
	heuristicFallback *HeuristicBiddingStrategy

	// Track current episode for training
	currentHandScore int
	currentBid       int

	// Metrics tracking (disabled by default)
	metrics StrategyMetrics
}

// NewQLearningBiddingStrategy creates a new Q-learning bidding strategy
func NewQLearningBiddingStrategy(epsilon float64) *QLearningBiddingStrategy {
	return &QLearningBiddingStrategy{
		qTable:            NewQTable(0.1, 0.9),
		epsilon:           NewExplorationSchedule(epsilon, 0.01, 0.995),
		heuristicFallback: &HeuristicBiddingStrategy{},
	}
}

func (q *QLearningBiddingStrategy) GetName() string {
	return "QLearningBidding"
}

// EncodeHand returns a state encoding for the given hand and current bid
// State space: 5×5×5×11×12 = 16,500 states
func (q *QLearningBiddingStrategy) EncodeHand(hand []game.Card, currentBid int) int {
	aces := 0
	tens := 0
	jacks := 0
	suitCounts := make(map[game.Suit]int)

	for _, card := range hand {
		if card.Rank == game.Jack {
			jacks++
		}
		if card.Rank == game.Ace {
			aces++
		}
		if card.Rank == game.Ten {
			tens++
		}
		suitCounts[card.Suit]++
	}

	// Find longest suit (best potential trump suit)
	maxSuitCount := 0
	for _, count := range suitCounts {
		if count > maxSuitCount {
			maxSuitCount = count
		}
	}

	// Calculate best game value (highest value we can declare)
	cards := game.Cards(hand)
	bestGameValue := 0

	// Check Grand
	grandValue := cards.GameValue(game.ModeGrand, game.NoSuit)
	if grandValue > bestGameValue {
		bestGameValue = grandValue
	}

	// Check each suit
	for suit := game.Clubs; suit <= game.Diamonds; suit++ {
		suitValue := cards.GameValue(game.ModeSuit, suit)
		if suitValue > bestGameValue {
			bestGameValue = suitValue
		}
	}

	// Calculate safety margin (how much buffer we have above the current bid)
	// Negative means we're overbidding, positive means we have room
	safetyMargin := bestGameValue - currentBid

	// Bucket safety margin into ranges: -60+ (0), -50 to -41 (1), ..., 50+ (11)
	// This gives us 12 buckets centered around 0
	safetyMarginBucket := (safetyMargin + 60) / 10
	if safetyMarginBucket < 0 {
		safetyMarginBucket = 0
	} else if safetyMarginBucket > 11 {
		safetyMarginBucket = 11
	}

	return aces*3300 + tens*660 + jacks*132 + maxSuitCount*12 + safetyMarginBucket
}

func (q *QLearningBiddingStrategy) ShouldBid(gs *game.GameState, hand []game.Card, currentBid int) bool {
	handScore := q.EncodeHand(hand, currentBid)
	q.currentHandScore = handScore

	// Get next bid value
	nextBid := gs.GetNextBidValue()
	if nextBid == 0 {
		q.currentBid = 0
		return false
	}

	// Track metrics: check if this state exists in Q-table
	q.metrics.Track(handScore, q.qTable.table)

	qPass := q.qTable.Get(handScore, 0)
	qAccept := q.qTable.Get(handScore, nextBid)

	var accept bool
	if rand.Float64() < q.epsilon.Get() {
		// Explore: random choice
		accept = rand.Float64() < 0.5
	} else {
		// Exploit: choose best action
		// If untrained (both zero), use heuristic fallback
		if qAccept == 0.0 && qPass == 0.0 {
			accept = q.heuristicFallback.ShouldBid(gs, hand, nextBid)
		} else if qAccept > qPass {
			accept = true
		} else if qPass > qAccept {
			accept = false
		} else {
			// Tied (both non-zero) - randomize
			accept = rand.Float64() < 0.5
		}
	}

	if accept {
		q.currentBid = nextBid
	} else {
		q.currentBid = 0
	}

	return accept
}

// Training methods

// CalculateReward calculates the reward for a bidding decision based on game outcome
func (q *QLearningBiddingStrategy) CalculateReward(playerResult game.PlayerResultState) float64 {
	reward := 0.0

	if playerResult.IsDeclarer {
		if playerResult.IsOverbid {
			reward = -5.0
		} else {
			// Use actual PlayerPoints scaled to reasonable range
			// PlayerPoints range: typically -240 to +120
			// Scale to reward range roughly -4.0 to +2.0
			reward = float64(playerResult.PlayerPoints) / 60.0
		}
	} else {
		if q.currentBid == 0 {
			// Agent passed - reward based on whether defenders won
			// PlayerPoints is always 0 for defenders, so use IsWinner instead
			if playerResult.IsWinner {
				// Defenders won, passing was correct
				reward = 1.0
			} else {
				// Declarer won, might have missed opportunity to bid
				reward = -0.1
			}
		} else {
			// Lost bidding war - small penalty
			reward = -0.05
		}
	}

	return reward
}

// OnGameEnd updates Q-values based on game outcome
func (q *QLearningBiddingStrategy) OnGameEnd(playerResult game.PlayerResultState) {
	reward := q.CalculateReward(playerResult)
	q.qTable.Update(q.currentHandScore, q.currentBid, reward)
}

// DecayEpsilon reduces exploration over time
func (q *QLearningBiddingStrategy) DecayEpsilon(minEpsilon float64) {
	q.epsilon.Decay()
	if q.epsilon.Get() < minEpsilon {
		q.epsilon.Set(minEpsilon)
	}
}

// GetQTableSize returns the number of states learned
func (q *QLearningBiddingStrategy) GetQTableSize() int {
	return q.qTable.Size()
}

// GetEpsilon returns current exploration rate
func (q *QLearningBiddingStrategy) GetEpsilon() float64 {
	return q.epsilon.Get()
}

// SetEpsilon sets exploration rate
func (q *QLearningBiddingStrategy) SetEpsilon(eps float64) {
	q.epsilon.Set(eps)
}

// GetQTable returns the underlying Q-table for serialization
func (q *QLearningBiddingStrategy) GetQTable() map[int]map[int]float64 {
	return q.qTable.GetTable()
}

// SetQTable sets the underlying Q-table (for deserialization)
func (q *QLearningBiddingStrategy) SetQTable(table map[int]map[int]float64) {
	q.qTable.SetTable(table)
}

// ShareQTable shares the Q-table reference with another strategy
func (q *QLearningBiddingStrategy) ShareQTable(other *QLearningBiddingStrategy) {
	other.qTable = q.qTable
}

// GetMetrics returns metrics about unseen states
func (q *QLearningBiddingStrategy) GetMetrics() (unseenStates, totalBids int) {
	return q.metrics.Get()
}

// ResetMetrics resets the metrics counters
func (q *QLearningBiddingStrategy) ResetMetrics() {
	q.metrics.Reset()
}

// EnableMetrics turns on metrics collection
func (q *QLearningBiddingStrategy) EnableMetrics() {
	q.metrics.Enable()
}

// DisableMetrics turns off metrics collection
func (q *QLearningBiddingStrategy) DisableMetrics() {
	q.metrics.Disable()
}

// QLearningGameChoiceStrategy uses Q-learning for game choice
type QLearningGameChoiceStrategy struct {
	qTable            *QTable
	epsilon           *ExplorationSchedule
	heuristicFallback *HeuristicGameChoiceStrategy

	// Track current episode for training
	currentHandState  int
	currentGameChoice int
	currentBidValue   int

	// Metrics tracking (disabled by default)
	metrics StrategyMetrics
}

// NewQLearningGameChoiceStrategy creates a new Q-learning game choice strategy
func NewQLearningGameChoiceStrategy(epsilon float64) *QLearningGameChoiceStrategy {
	return &QLearningGameChoiceStrategy{
		qTable:            NewQTable(0.1, 0.9),
		epsilon:           NewExplorationSchedule(epsilon, 0.01, 0.995),
		heuristicFallback: &HeuristicGameChoiceStrategy{},
	}
}

func (q *QLearningGameChoiceStrategy) GetName() string {
	return "QLearningGameChoice"
}

// EncodeHand returns a state encoding for the given hand (after picking up skat)
// State space: 8×8×8×8×5×9 = 23,040 theoretical states
// Practical: C(11,3) × 5 × 9 = 7,425 states
func (q *QLearningGameChoiceStrategy) EncodeHand(hand []game.Card) int {
	// Track suit lengths and total high cards
	suitLengths := make(map[game.Suit]int)
	jackCount := 0
	totalHighCards := 0 // Count Aces + Tens across all suits

	for _, card := range hand {
		if card.Rank == game.Jack {
			jackCount++
		} else {
			// Count non-jack cards in each suit
			suitLengths[card.Suit]++
			if card.Rank == game.Ace || card.Rank == game.Ten {
				totalHighCards++
			}
		}
	}

	// Get length for each suit (0-7, since jacks are counted separately)
	clubsLength := suitLengths[game.Clubs]
	spadesLength := suitLengths[game.Spades]
	heartsLength := suitLengths[game.Hearts]
	diamondsLength := suitLengths[game.Diamonds]

	// totalHighCards ranges from 0-8 (4 aces + 4 tens)
	if totalHighCards > 8 {
		totalHighCards = 8
	}

	return clubsLength*2880 + spadesLength*360 + heartsLength*45 + diamondsLength*5 + jackCount*9 + totalHighCards
}

func (q *QLearningGameChoiceStrategy) ChooseGame(hand []game.Card, bidValue int) (game.GameMode, game.Suit) {
	handState := q.EncodeHand(hand)
	q.currentHandState = handState
	q.currentBidValue = bidValue

	// Get valid game actions (those that meet the bid value)
	cards := game.Cards(hand)
	validActions := make([]int, 0, 5)

	// Check Grand (action 0)
	if cards.GameValue(game.ModeGrand, game.NoSuit) >= bidValue {
		validActions = append(validActions, EncodeGameAction(game.ModeGrand, game.NoSuit))
	}

	// Check each suit (actions 1-4)
	for suit := game.Clubs; suit <= game.Diamonds; suit++ {
		if cards.GameValue(game.ModeSuit, suit) >= bidValue {
			validActions = append(validActions, EncodeGameAction(game.ModeSuit, suit))
		}
	}

	// If no valid games, fall back to all games
	if len(validActions) == 0 {
		for i := 0; i < 5; i++ {
			validActions = append(validActions, i)
		}
	}

	var mode game.GameMode
	var suit game.Suit

	// Track metrics: check if this state exists in Q-table
	q.metrics.Track(handState, q.qTable.table)

	if rand.Float64() < q.epsilon.Get() {
		// Explore: choose random VALID game
		action := validActions[rand.Intn(len(validActions))]
		mode, suit = DecodeGameAction(action)
		q.currentGameChoice = action
	} else {
		// Exploit: choose best Q-value among VALID actions
		bestAction := validActions[0]
		bestQ := q.qTable.Get(handState, validActions[0])
		allZero := true

		for _, action := range validActions {
			qVal := q.qTable.Get(handState, action)
			if qVal != 0.0 {
				allZero = false
			}
			if qVal > bestQ {
				bestQ = qVal
				bestAction = action
			}
		}

		// If all Q-values are zero (untrained), explore randomly instead of using heuristic
		// This ensures we learn about all game types, not just Grand
		if allZero {
			action := validActions[rand.Intn(len(validActions))]
			mode, suit = DecodeGameAction(action)
			q.currentGameChoice = action
		} else {
			mode, suit = DecodeGameAction(bestAction)
			q.currentGameChoice = bestAction
		}
	}

	return mode, suit
}

func (q *QLearningGameChoiceStrategy) ChooseSkatDiscard(hand []game.Card, mode game.GameMode, trumpSuit game.Suit) (game.Card, game.Card) {
	return q.heuristicFallback.ChooseSkatDiscard(hand, mode, trumpSuit)
}

// Training methods

// CalculateReward calculates the reward for a game choice based on outcome
func (q *QLearningGameChoiceStrategy) CalculateReward(playerResult game.PlayerResultState) float64 {
	// Was overbid, nothing we could do
	if playerResult.IsDeclarer && playerResult.IsOverbid {
		return 0.0
	}
	// Use actual PlayerPoints scaled to reasonable range
	// PlayerPoints range: typically -240 to +120
	// Scale to reward range roughly -4.0 to +2.0
	return float64(playerResult.PlayerPoints) / 60.0
}

// OnGameChoiceEnd updates Q-values for game choice based on outcome
func (q *QLearningGameChoiceStrategy) OnGameChoiceEnd(playerResult game.PlayerResultState) {
	reward := q.CalculateReward(playerResult)
	q.qTable.Update(q.currentHandState, q.currentGameChoice, reward)
}

// DecayEpsilon reduces exploration over time
func (q *QLearningGameChoiceStrategy) DecayEpsilon(minEpsilon float64) {
	q.epsilon.Decay()
	if q.epsilon.Get() < minEpsilon {
		q.epsilon.Set(minEpsilon)
	}
}

// GetQTableSize returns the number of states learned
func (q *QLearningGameChoiceStrategy) GetQTableSize() int {
	return q.qTable.Size()
}

// GetEpsilon returns current exploration rate
func (q *QLearningGameChoiceStrategy) GetEpsilon() float64 {
	return q.epsilon.Get()
}

// SetEpsilon sets exploration rate
func (q *QLearningGameChoiceStrategy) SetEpsilon(eps float64) {
	q.epsilon.Set(eps)
}

// GetQTable returns the underlying Q-table for serialization
func (q *QLearningGameChoiceStrategy) GetQTable() map[int]map[int]float64 {
	return q.qTable.GetTable()
}

// SetQTable sets the underlying Q-table (for deserialization)
func (q *QLearningGameChoiceStrategy) SetQTable(table map[int]map[int]float64) {
	q.qTable.SetTable(table)
}

// GetMetrics returns metrics about unseen states
func (q *QLearningGameChoiceStrategy) GetMetrics() (unseenStates, totalChoices int) {
	return q.metrics.Get()
}

// ResetMetrics resets the metrics counters
func (q *QLearningGameChoiceStrategy) ResetMetrics() {
	q.metrics.Reset()
}

// EnableMetrics turns on metrics collection
func (q *QLearningGameChoiceStrategy) EnableMetrics() {
	q.metrics.Enable()
}

// DisableMetrics turns off metrics collection
func (q *QLearningGameChoiceStrategy) DisableMetrics() {
	q.metrics.Disable()
}

// ShareQTable shares the Q-table reference with another strategy
func (q *QLearningGameChoiceStrategy) ShareQTable(other *QLearningGameChoiceStrategy) {
	other.qTable = q.qTable
}
