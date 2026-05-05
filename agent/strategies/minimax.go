package strategies

import (
	"math"
	"skat/game"
	"sort"
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
	maxDepth         int
	transTable       map[uint64]*TranspositionEntry
	transMutex       sync.RWMutex
	useMoveOrdering  bool
	useTransTable    bool
	useLateMoveRed   bool
	lateMoveThreshold int
	lateMoveReduction int
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
		lateMoveThreshold: 3,
		lateMoveReduction: 2,
	}
}

func (m *PerfectInfoMinimaxStrategy) GetName() string {
	return "PerfectInfoMinimax"
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
		m.orderMoves(validMoves, isDeclarer)
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
		m.orderMoves(validMoves, isDeclarer)
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

// orderMoves sorts moves to improve alpha-beta pruning efficiency
// High-value cards are searched first for declarers, low-value for defenders
func (m *PerfectInfoMinimaxStrategy) orderMoves(moves []game.Card, isDeclarer bool) {
	if isDeclarer {
		// Declarer: try high-value cards first
		sort.Slice(moves, func(i, j int) bool {
			return moves[i].Value() > moves[j].Value()
		})
	} else {
		// Defenders: try low-value cards first (to avoid giving points)
		sort.Slice(moves, func(i, j int) bool {
			return moves[i].Value() < moves[j].Value()
		})
	}
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

	// Hash current player
	hash = hash*31 + uint64(state.CurrentPlayer)

	// Hash declarer score
	hash = hash*31 + uint64(state.DeclarerScore)

	return hash
}

// evaluate returns a heuristic score for the current state
// Positive values favor the declarer, negative values favor defenders
func (m *PerfectInfoMinimaxStrategy) evaluate(state *game.GameState) float64 {
	if state.Declarer == nil {
		return 0.0
	}

	// Start with current score
	score := float64(state.DeclarerScore)

	// Add remaining card values in hands
	declarer := *state.Declarer
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

	// Add cards in the current trick (not yet scored)
	for _, card := range state.Trick {
		score += float64(card.Value())
	}

	return score
}
