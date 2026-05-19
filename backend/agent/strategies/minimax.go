package strategies

import (
	"math"
	"skat/game"
	"sync"
)

// TranspositionEntry stores cached evaluation results
type TranspositionEntry struct {
	depth int
	value float64
	alpha float64
	beta  float64
}

// PerfectInfoMinimaxStrategy implements minimax search with perfect information
// This is suitable for generating optimal training data where all hands are known
type PerfectInfoMinimaxStrategy struct {
	maxDepth          int
	transTable        map[uint64]*TranspositionEntry
	transMutex        sync.RWMutex
	useMoveOrdering   bool
	useTransTable     bool
	useLateMoveRed    bool
	lateMoveThreshold int
	lateMoveReduction int
}

// PerfectInfoMinimaxVsHeuristicStrategy searches the root player's choices with
// perfect information, while predicting every other player with the heuristic
// card-play policy. This keeps the branching factor small enough for full-game
// depth searches against heuristic opponents.
type PerfectInfoMinimaxVsHeuristicStrategy struct {
	*PerfectInfoMinimaxStrategy
	heuristicMoveCache map[uint64]game.Card
}

// NewPerfectInfoMinimaxStrategy creates a new perfect-info minimax strategy
func NewPerfectInfoMinimaxStrategy() *PerfectInfoMinimaxStrategy {
	return &PerfectInfoMinimaxStrategy{
		maxDepth:          30, // Search full game tree
		transTable:        make(map[uint64]*TranspositionEntry),
		useMoveOrdering:   true,
		useTransTable:     true,
		useLateMoveRed:    true,
		lateMoveThreshold: 3, // Start reducing after 3rd move
		lateMoveReduction: 2, // Reduce depth by 2
	}
}

// NewPerfectInfoMinimaxStrategyWithDepth creates a strategy with custom depth
func NewPerfectInfoMinimaxStrategyWithDepth(maxDepth int) *PerfectInfoMinimaxStrategy {
	return &PerfectInfoMinimaxStrategy{
		maxDepth:          maxDepth,
		transTable:        make(map[uint64]*TranspositionEntry),
		useMoveOrdering:   true,
		useTransTable:     true,
		useLateMoveRed:    true,
		lateMoveThreshold: 2,
		lateMoveReduction: 3,
	}
}

// NewPerfectInfoMinimaxVsHeuristicStrategy creates a full-depth minimax strategy
// that predicts non-root players with the heuristic card-play strategy.
func NewPerfectInfoMinimaxVsHeuristicStrategy() *PerfectInfoMinimaxVsHeuristicStrategy {
	return NewPerfectInfoMinimaxVsHeuristicStrategyWithDepth(30)
}

// NewPerfectInfoMinimaxVsHeuristicStrategyWithDepth creates a strategy with a
// custom maximum depth. A depth of 30 can simulate the whole card-play phase.
func NewPerfectInfoMinimaxVsHeuristicStrategyWithDepth(maxDepth int) *PerfectInfoMinimaxVsHeuristicStrategy {
	return &PerfectInfoMinimaxVsHeuristicStrategy{
		heuristicMoveCache: make(map[uint64]game.Card),
		PerfectInfoMinimaxStrategy: &PerfectInfoMinimaxStrategy{
			maxDepth:          maxDepth,
			transTable:        make(map[uint64]*TranspositionEntry),
			useMoveOrdering:   true,
			useTransTable:     true,
			useLateMoveRed:    false,
			lateMoveThreshold: 0,
			lateMoveReduction: 0,
		},
	}
}

func (m *PerfectInfoMinimaxStrategy) GetName() string {
	return "PerfectInfoMinimax"
}

func (m *PerfectInfoMinimaxVsHeuristicStrategy) GetName() string {
	return "PerfectInfoMinimaxVsHeuristic"
}

func (m *PerfectInfoMinimaxStrategy) SelectMove(state *game.GameState, validMoves []game.Card) game.Card {
	if len(validMoves) == 1 {
		return validMoves[0]
	}

	// Clear transposition table for new move selection
	if m.useTransTable {
		m.transMutex.Lock()
		m.transTable = make(map[uint64]*TranspositionEntry)
		m.transMutex.Unlock()
	}

	currentPlayer := state.CurrentPlayer
	isDeclarer := state.Declarer != nil && currentPlayer == *state.Declarer

	// Order moves by card value for better pruning
	if m.useMoveOrdering {
		m.orderMoves(state, validMoves, isDeclarer)
	}

	var bestMove game.Card
	var bestValue float64

	if isDeclarer {
		bestValue = math.Inf(-1) // Maximize for declarer
	} else {
		bestValue = math.Inf(1) // Minimize for defenders
	}

	alpha, beta := math.Inf(-1), math.Inf(1)

	for _, move := range validMoves {
		// Clone state and apply move
		nextState := state.Clone()
		nextState.PlayCard(move)

		// Resolve trick if complete
		if len(nextState.Trick) == 3 {
			nextState.ResolveTrick()
		}

		// Evaluate this move
		value := m.minimax(nextState, m.maxDepth-1, alpha, beta)

		// Declarer maximizes, defenders minimize
		if isDeclarer {
			if value > bestValue {
				bestValue = value
				bestMove = move
			}
			alpha = math.Max(alpha, value)
		} else {
			if value < bestValue {
				bestValue = value
				bestMove = move
			}
			beta = math.Min(beta, value)
		}
	}

	return bestMove
}

func (m *PerfectInfoMinimaxVsHeuristicStrategy) SelectMove(state *game.GameState, validMoves []game.Card) game.Card {
	if len(validMoves) == 1 {
		return validMoves[0]
	}

	if m.useTransTable {
		m.transMutex.Lock()
		m.transTable = make(map[uint64]*TranspositionEntry)
		m.heuristicMoveCache = make(map[uint64]game.Card)
		m.transMutex.Unlock()
	}

	rootPlayer := state.CurrentPlayer
	rootIsDeclarer := state.Declarer != nil && rootPlayer == *state.Declarer

	if m.useMoveOrdering {
		m.orderMoves(state, validMoves, rootIsDeclarer)
	}

	bestMove := validMoves[0]
	bestValue := math.Inf(-1)
	if !rootIsDeclarer {
		bestValue = math.Inf(1)
	}

	for _, move := range validMoves {
		nextState := state.Clone()
		m.playAndResolve(nextState, move)

		value := m.minimaxVsHeuristic(nextState, m.maxDepth-1, math.Inf(-1), math.Inf(1), rootPlayer)
		if rootIsDeclarer {
			if value > bestValue {
				bestValue = value
				bestMove = move
			}
		} else if value < bestValue {
			bestValue = value
			bestMove = move
		}
	}

	return bestMove
}

// minimax performs alpha-beta pruning minimax search
func (m *PerfectInfoMinimaxStrategy) minimax(state *game.GameState, depth int, alpha, beta float64) float64 {
	// Terminal conditions
	if state.Phase != game.PhasePlaying || depth == 0 {
		return m.evaluate(state)
	}

	// Check transposition table
	if m.useTransTable {
		hash := m.hashState(state)
		m.transMutex.RLock()
		entry, found := m.transTable[hash]
		m.transMutex.RUnlock()

		if found && entry.depth >= depth {
			// Use cached value if it's valid for this alpha-beta window
			if entry.value <= entry.alpha {
				if entry.alpha >= beta {
					return entry.alpha
				}
			} else if entry.value >= entry.beta {
				if entry.beta <= alpha {
					return entry.beta
				}
			} else {
				return entry.value
			}
		}
	}

	validMoves := state.GetValidMoves()
	if len(validMoves) == 0 {
		return m.evaluate(state)
	}

	// Order moves for better pruning
	if m.useMoveOrdering {
		isDeclarer := state.Declarer != nil && state.CurrentPlayer == *state.Declarer
		m.orderMoves(state, validMoves, isDeclarer)
	}

	isDeclarer := state.Declarer != nil && state.CurrentPlayer == *state.Declarer

	var value float64
	if isDeclarer {
		// Maximizing player (declarer)
		maxValue := math.Inf(-1)
		for i, move := range validMoves {
			nextState := state.Clone()
			nextState.PlayCard(move)

			if len(nextState.Trick) == 3 {
				nextState.ResolveTrick()
			}

			// Late move reduction: search less promising moves at reduced depth
			searchDepth := depth - 1
			if m.useLateMoveRed && i >= m.lateMoveThreshold && depth >= m.lateMoveReduction+1 {
				searchDepth = depth - 1 - m.lateMoveReduction
			}

			value = m.minimax(nextState, searchDepth, alpha, beta)
			maxValue = math.Max(maxValue, value)
			alpha = math.Max(alpha, value)

			if beta <= alpha {
				break // Beta cutoff
			}
		}
		value = maxValue
	} else {
		// Minimizing player (defenders)
		minValue := math.Inf(1)
		for i, move := range validMoves {
			nextState := state.Clone()
			nextState.PlayCard(move)

			if len(nextState.Trick) == 3 {
				nextState.ResolveTrick()
			}

			// Late move reduction
			searchDepth := depth - 1
			if m.useLateMoveRed && i >= m.lateMoveThreshold && depth >= m.lateMoveReduction+1 {
				searchDepth = depth - 1 - m.lateMoveReduction
			}

			value = m.minimax(nextState, searchDepth, alpha, beta)
			minValue = math.Min(minValue, value)
			beta = math.Min(beta, value)

			if beta <= alpha {
				break // Alpha cutoff
			}
		}
		value = minValue
	}

	// Store in transposition table
	if m.useTransTable {
		hash := m.hashState(state)
		m.transMutex.Lock()
		m.transTable[hash] = &TranspositionEntry{
			depth: depth,
			value: value,
			alpha: alpha,
			beta:  beta,
		}
		m.transMutex.Unlock()
	}

	return value
}

func (m *PerfectInfoMinimaxVsHeuristicStrategy) minimaxVsHeuristic(state *game.GameState, depth int, alpha, beta float64, rootPlayer game.GamePosition) float64 {
	if state.Phase != game.PhasePlaying || depth == 0 {
		return m.evaluate(state)
	}

	validMoves := state.GetValidMoves()
	if len(validMoves) == 0 {
		return m.evaluate(state)
	}

	if state.CurrentPlayer != rootPlayer {
		nextState := state.Clone()
		move := m.predictHeuristicMove(nextState, validMoves)
		m.playAndResolve(nextState, move)
		return m.minimaxVsHeuristic(nextState, depth-1, alpha, beta, rootPlayer)
	}

	if m.useTransTable {
		hash := m.hashStateForRoot(state, rootPlayer)
		m.transMutex.RLock()
		entry, found := m.transTable[hash]
		m.transMutex.RUnlock()
		if found && entry.depth >= depth {
			return entry.value
		}
	}

	rootIsDeclarer := state.Declarer != nil && rootPlayer == *state.Declarer
	if m.useMoveOrdering {
		m.orderMoves(state, validMoves, rootIsDeclarer)
	}

	var value float64
	if rootIsDeclarer {
		value = math.Inf(-1)
		for _, move := range validMoves {
			nextState := state.Clone()
			m.playAndResolve(nextState, move)

			childValue := m.minimaxVsHeuristic(nextState, depth-1, alpha, beta, rootPlayer)
			value = math.Max(value, childValue)
			alpha = math.Max(alpha, childValue)
			if beta <= alpha {
				break
			}
		}
	} else {
		value = math.Inf(1)
		for _, move := range validMoves {
			nextState := state.Clone()
			m.playAndResolve(nextState, move)

			childValue := m.minimaxVsHeuristic(nextState, depth-1, alpha, beta, rootPlayer)
			value = math.Min(value, childValue)
			beta = math.Min(beta, childValue)
			if beta <= alpha {
				break
			}
		}
	}

	if m.useTransTable {
		hash := m.hashStateForRoot(state, rootPlayer)
		m.transMutex.Lock()
		m.transTable[hash] = &TranspositionEntry{
			depth: depth,
			value: value,
			alpha: alpha,
			beta:  beta,
		}
		m.transMutex.Unlock()
	}

	return value
}

func (m *PerfectInfoMinimaxStrategy) playAndResolve(state *game.GameState, move game.Card) {
	state.PlayCard(move)
	if len(state.Trick) == 3 {
		state.ResolveTrick()
	}
}

func (m *PerfectInfoMinimaxVsHeuristicStrategy) predictHeuristicMove(state *game.GameState, validMoves []game.Card) game.Card {
	hash := m.hashStateForRoot(state, state.CurrentPlayer)

	m.transMutex.RLock()
	move, found := m.heuristicMoveCache[hash]
	m.transMutex.RUnlock()
	if found {
		return move
	}

	heuristic := heuristicStrategyForState(state)
	moves := append([]game.Card{}, validMoves...)
	move = heuristic.SelectMove(state, moves)

	m.transMutex.Lock()
	m.heuristicMoveCache[hash] = move
	m.transMutex.Unlock()

	return move
}

func heuristicStrategyForState(state *game.GameState) *HeuristicCardPlayStrategy {
	heuristic := NewHeuristicCardPlayStrategy()
	for _, trick := range state.CardsPlayed {
		for _, card := range trick {
			heuristic.cardsPlayed[card] = true
		}
	}
	return heuristic
}

// orderMoves sorts moves to improve alpha-beta pruning efficiency
// Uses heuristic-based ordering to prioritize moves likely to be good
func (m *PerfectInfoMinimaxStrategy) orderMoves(state *game.GameState, moves []game.Card, isDeclarer bool) {
	// Use heuristic-based move ordering for better pruning
	heuristicOrder(state, moves, isDeclarer)
}

// hashState creates a hash of the game state for transposition table
func (m *PerfectInfoMinimaxStrategy) hashState(state *game.GameState) uint64 {
	var hash uint64 = 0

	// Hash player hands
	for p := 0; p < 3; p++ {
		for _, card := range state.Players[p].Hand {
			// Simple hash combining suit and rank
			cardHash := uint64(card.Suit)*13 + uint64(card.Rank)
			hash = hash*31 + cardHash
		}
	}

	// Hash current trick
	for _, card := range state.Trick {
		cardHash := uint64(card.Suit)*13 + uint64(card.Rank)
		hash = hash*31 + cardHash
	}

	for _, trick := range state.CardsPlayed {
		hash = hash*31 + 17
		for _, card := range trick {
			cardHash := uint64(card.Suit)*13 + uint64(card.Rank)
			hash = hash*31 + cardHash
		}
	}

	// Hash current player
	hash = hash*31 + uint64(state.CurrentPlayer)

	// Hash declarer score
	for _, score := range state.PlayerScores {
		hash = hash*31 + uint64(score)
	}
	hash = hash*31 + uint64(state.TrickStarter)
	hash = hash*31 + uint64(state.TrumpSuit)
	hash = hash*31 + hashGameMode(state.Mode)

	return hash
}

func (m *PerfectInfoMinimaxStrategy) hashStateForRoot(state *game.GameState, rootPlayer game.GamePosition) uint64 {
	hash := m.hashState(state)
	hash = hash*31 + uint64(rootPlayer)
	return hash
}

func hashGameMode(mode game.GameMode) uint64 {
	var hash uint64
	for _, r := range string(mode) {
		hash = hash*31 + uint64(r)
	}
	return hash
}

// evaluate returns a heuristic score for the current state
// Positive values favor the declarer, negative values favor defenders
func (m *PerfectInfoMinimaxStrategy) evaluate(state *game.GameState) float64 {
	if state.Declarer == nil {
		return 0.0
	}
	if state.Phase == game.PhaseComplete {
		return m.evaluateTerminal(state)
	}

	declarer := *state.Declarer

	// Material score (points already won + projected points)
	materialScore := m.evaluateMaterial(state, declarer)

	// Positional score (hand strength, trick control, etc.)
	positionalScore := m.evaluatePosition(state, declarer)

	// Weighted combination
	// Material dominates endgame, position matters in opening/midgame
	tricksRemaining := 0
	for p := 0; p < 3; p++ {
		tricksRemaining += len(state.Players[p].Hand)
	}
	tricksRemaining = tricksRemaining / 3 // Total tricks remaining

	materialWeight := 1.0
	positionalWeight := float64(tricksRemaining) / 10.0 // Decreases as game progresses

	return materialWeight*materialScore + positionalWeight*positionalScore
}

func (m *PerfectInfoMinimaxStrategy) evaluateTerminal(state *game.GameState) float64 {
	declarerWon, _, _ := state.GetGameResult()
	if state.Overbid {
		declarerWon = false
	}

	margin := float64(state.DeclarerCardScore() - state.OpponentCardScore())
	if state.Mode == game.ModeNull {
		if declarerWon {
			return 1000.0
		}
		return -1000.0
	}
	if declarerWon {
		return 1000.0 + margin
	}
	return -1000.0 + margin
}

// evaluateMaterial calculates material advantage (points)
func (m *PerfectInfoMinimaxStrategy) evaluateMaterial(state *game.GameState, declarer game.GamePosition) float64 {
	// Start with current score
	score := float64(state.DeclarerCardScore())

	// Add remaining card values in hands
	for p := 0; p < 3; p++ {
		pos := game.GamePosition(p)
		for _, card := range state.Players[p].Hand {
			cardValue := float64(card.Value())
			if pos == declarer {
				score += cardValue
			} else {
				score -= cardValue
			}
		}
	}

	// Add cards in the current trick
	if len(state.Trick) > 0 {
		trickValue := 0
		for _, card := range state.Trick {
			trickValue += card.Value()
		}

		// Find the winning card so far
		winner := game.Dealer
		winCard := state.Trick[0]
		for i := game.Listener; i < game.GamePosition(len(state.Trick)); i++ {
			if state.CardBeats(state.Trick[i], winCard) {
				winner = i
				winCard = state.Trick[i]
			}
		}

		actualWinner := (state.TrickStarter + winner) % 3

		if actualWinner == declarer {
			score += float64(trickValue)
		} else {
			score -= float64(trickValue)
		}
	}

	return score
}

// evaluatePosition calculates positional advantage (hand strength, control)
func (m *PerfectInfoMinimaxStrategy) evaluatePosition(state *game.GameState, declarer game.GamePosition) float64 {
	score := 0.0

	// Evaluate trump control
	trumpControl := m.evaluateTrumpControl(state, declarer)
	score += trumpControl * 20.0 // Trump control is very important

	// Evaluate high card control (Aces and Tens)
	highCardControl := m.evaluateHighCardControl(state, declarer)
	score += highCardControl * 15.0

	// Evaluate suit length advantages
	suitControl := m.evaluateSuitControl(state, declarer)
	score += suitControl * 10.0

	return score
}

// evaluateTrumpControl returns -1 to +1 (negative favors defenders)
func (m *PerfectInfoMinimaxStrategy) evaluateTrumpControl(state *game.GameState, declarer game.GamePosition) float64 {
	declarerTrumps := 0
	defenderTrumps := 0

	for p := 0; p < 3; p++ {
		pos := game.GamePosition(p)
		for _, card := range state.Players[p].Hand {
			isTrump := card.Rank == game.Jack || (state.Mode == game.ModeSuit && card.Suit == state.TrumpSuit)
			if isTrump {
				if pos == declarer {
					declarerTrumps++
					// Weight by card strength (Jacks more valuable)
					if card.Rank == game.Jack {
						if card.Suit == game.Clubs {
							declarerTrumps += 2 // J♣ is strongest
						} else if card.Suit == game.Spades {
							declarerTrumps += 1
						}
					}
				} else {
					defenderTrumps++
					if card.Rank == game.Jack {
						if card.Suit == game.Clubs {
							defenderTrumps += 2
						} else if card.Suit == game.Spades {
							defenderTrumps += 1
						}
					}
				}
			}
		}
	}

	totalTrumps := declarerTrumps + defenderTrumps
	if totalTrumps == 0 {
		return 0.0
	}

	// Normalize to -1..+1 range
	return (float64(declarerTrumps) - float64(defenderTrumps)) / float64(totalTrumps+4)
}

// evaluateHighCardControl returns -1 to +1 (negative favors defenders)
func (m *PerfectInfoMinimaxStrategy) evaluateHighCardControl(state *game.GameState, declarer game.GamePosition) float64 {
	declarerHighCards := 0
	defenderHighCards := 0

	for p := 0; p < 3; p++ {
		pos := game.GamePosition(p)
		for _, card := range state.Players[p].Hand {
			// Count Aces and Tens (high-value cards)
			if card.Rank == game.Ace {
				if pos == declarer {
					declarerHighCards += 2
				} else {
					defenderHighCards += 2
				}
			} else if card.Rank == game.Ten {
				if pos == declarer {
					declarerHighCards += 1
				} else {
					defenderHighCards += 1
				}
			}
		}
	}

	total := declarerHighCards + defenderHighCards
	if total == 0 {
		return 0.0
	}

	return (float64(declarerHighCards) - float64(defenderHighCards)) / float64(total)
}

// evaluateSuitControl returns -1 to +1 (negative favors defenders)
func (m *PerfectInfoMinimaxStrategy) evaluateSuitControl(state *game.GameState, declarer game.GamePosition) float64 {
	score := 0.0

	for suit := game.Clubs; suit <= game.Diamonds; suit++ {
		declarerCards := 0
		defenderCards := 0

		for p := 0; p < 3; p++ {
			pos := game.GamePosition(p)
			for _, card := range state.Players[p].Hand {
				if card.Suit == suit && card.Rank != game.Jack {
					if pos == declarer {
						declarerCards++
					} else {
						defenderCards++
					}
				}
			}
		}

		// Long suits are valuable (can force opponents, set up tricks)
		// For declarer: long suits good for control
		// For defenders: long suits good for forcing declarer to use trumps
		if declarerCards >= 3 {
			score += 0.2 // Declarer has length advantage
		}
		if defenderCards >= 4 {
			score -= 0.3 // Defenders have strong suit to pressure declarer
		}

		// Voids are also valuable (can trump in)
		if declarerCards == 0 && state.Mode == game.ModeSuit {
			// Declarer void in side suit = can trump
			score += 0.3
		}
	}

	return score
}
