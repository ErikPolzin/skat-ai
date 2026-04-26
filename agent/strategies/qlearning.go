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

// Hand state encoding functions

// EncodeHandState converts (aces, tens, jacks, trumps, gamesPlayable) into a single integer key
// State space: 5×5×5×11×7 = 9,625 possible states
func EncodeHandState(aces, tens, jacks, trumps, gamesPlayable int) int {
	return aces*1925 + tens*385 + jacks*77 + trumps*7 + gamesPlayable
}

// EvaluateHand returns a state encoding based on high-value card counts
func EvaluateHand(hand []game.Card, currentBid int) int {
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

	// Calculate how many games can be played at current bid level
	cards := game.Cards(hand)
	gamesPlayable := cards.CountGamesPlayable(currentBid)

	return EncodeHandState(aces, tens, jacks, maxSuitCount, gamesPlayable)
}

// EvaluateHandWithSkat counts high cards INCLUDING skat (for game choice)
func EvaluateHandWithSkat(hand []game.Card) int {
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

	maxSuitCount := 0
	for _, count := range suitCounts {
		if count > maxSuitCount {
			maxSuitCount = count
		}
	}

	// For game choice, we don't have a bid context, so use 0 for gamesPlayable
	return EncodeHandState(aces, tens, jacks, maxSuitCount, 0)
}

// Game action encoding functions

// EncodeGameAction encodes game mode + suit choice into a single integer
// Grand: 0, Suit Clubs: 1, Suit Spades: 2, Suit Hearts: 3, Suit Diamonds: 4
func EncodeGameAction(mode game.GameMode, suit game.Suit) int {
	if mode == game.ModeGrand {
		return 0
	}
	return int(suit) + 1 // Clubs=1, Spades=2, Hearts=3, Diamonds=4
}

// DecodeGameAction decodes an integer action into game mode and suit
func DecodeGameAction(action int) (game.GameMode, game.Suit) {
	if action == 0 {
		return game.ModeGrand, game.Clubs // Suit doesn't matter for Grand
	}
	return game.ModeSuit, game.Suit(action - 1)
}

// QLearningBiddingStrategy uses Q-learning for bidding decisions
type QLearningBiddingStrategy struct {
	qTable            *QTable
	epsilon           *ExplorationSchedule
	heuristicFallback *HeuristicBiddingStrategy

	// Track current episode for training
	currentHandScore int
	currentBid       int
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

func (q *QLearningBiddingStrategy) ShouldBid(gs *game.GameState, hand []game.Card, currentBid int) bool {
	handScore := EvaluateHand(hand, currentBid)
	q.currentHandScore = handScore

	// Get next bid value
	nextBid := gs.GetNextBidValue()
	if nextBid == 0 {
		q.currentBid = 0
		return false
	}

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

// OnGameEnd updates Q-values based on game outcome
func (q *QLearningBiddingStrategy) OnGameEnd(playerResult game.PlayerResultState) {
	reward := 0.0

	if playerResult.IsDeclarer {
		if playerResult.IsWinner {
			safetyMargin := float64(playerResult.PlayerPoints-61) / 60.0
			reward = 1.0 + safetyMargin*0.5
		} else {
			if playerResult.PlayerPoints >= 55 {
				reward = -0.3
			} else if playerResult.PlayerPoints >= 45 {
				reward = -0.6
			} else {
				reward = -1.0
			}
			// Extra penalty for overbidding badly
			if q.currentBid > 30 && playerResult.PlayerPoints < 40 {
				reward -= 0.5
			}
		}
	} else {
		if q.currentBid == 0 {
			// Agent passed
			if playerResult.IsWinner {
				reward = -0.1 // Missed opportunity
			} else {
				reward = 0.1 // Correctly passed
			}
		} else {
			// Lost bidding war
			reward = -0.05
		}
	}

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

// QLearningGameChoiceStrategy uses Q-learning for game choice
type QLearningGameChoiceStrategy struct {
	qTable            *QTable
	epsilon           *ExplorationSchedule
	heuristicFallback *HeuristicGameChoiceStrategy

	// Track current episode for training
	currentHandState  int
	currentGameChoice int
	currentBidValue   int
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

func (q *QLearningGameChoiceStrategy) ChooseGame(hand []game.Card, bidValue int) (game.GameMode, game.Suit) {
	handState := EvaluateHandWithSkat(hand)
	q.currentHandState = handState
	q.currentBidValue = bidValue

	// Get valid game actions (those that meet the bid value)
	cards := game.Cards(hand)
	validActions := make([]int, 0, 5)

	// Check Grand (action 0)
	if cards.GameValue(game.ModeGrand, game.NoSuit) >= bidValue {
		validActions = append(validActions, 0)
	}

	// Check each suit (actions 1-4)
	for suit := game.Clubs; suit <= game.Diamonds; suit++ {
		if cards.GameValue(game.ModeSuit, suit) >= bidValue {
			validActions = append(validActions, int(suit)+1)
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

	if rand.Float64() < q.epsilon.Get() {
		// Explore: choose random VALID game
		action := validActions[rand.Intn(len(validActions))]
		mode, suit = DecodeGameAction(action)
		q.currentGameChoice = action
	} else {
		// Exploit: choose best Q-value among VALID actions
		bestAction := validActions[0]
		bestQ := q.qTable.Get(handState, validActions[0])

		for _, action := range validActions[1:] {
			qVal := q.qTable.Get(handState, action)
			if qVal > bestQ {
				bestQ = qVal
				bestAction = action
			}
		}

		// If all Q-values are zero (untrained), use heuristic
		if bestQ == 0.0 {
			mode, suit = q.heuristicFallback.ChooseGame(hand, bidValue)
			q.currentGameChoice = EncodeGameAction(mode, suit)
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

// OnGameChoiceEnd updates Q-values for game choice based on outcome
func (q *QLearningGameChoiceStrategy) OnGameChoiceEnd(playerResult game.PlayerResultState) {
	reward := 0.0

	if playerResult.IsWinner {
		// Reward proportional to how well we won
		safetyMargin := float64(playerResult.PlayerPoints-61) / 60.0
		reward = 1.0 + safetyMargin*0.3
	} else {
		// Penalty proportional to how badly we lost
		if playerResult.PlayerPoints >= 55 {
			reward = -0.4 // Close loss
		} else if playerResult.PlayerPoints >= 45 {
			reward = -0.7
		} else {
			reward = -1.0 // Bad loss
		}
	}
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
