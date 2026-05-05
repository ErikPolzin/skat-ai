package strategies

import (
	"math"
	"skat/game"
)

// PerfectInfoMinimaxStrategy implements minimax search with perfect information
// This is suitable for generating optimal training data where all hands are known
type PerfectInfoMinimaxStrategy struct {
	maxDepth int
}

// NewPerfectInfoMinimaxStrategy creates a new perfect-info minimax strategy
func NewPerfectInfoMinimaxStrategy() *PerfectInfoMinimaxStrategy {
	return &PerfectInfoMinimaxStrategy{
		maxDepth: 10, // Search up to 10 cards ahead (adjustable)
	}
}

// NewPerfectInfoMinimaxStrategyWithDepth creates a strategy with custom depth
func NewPerfectInfoMinimaxStrategyWithDepth(maxDepth int) *PerfectInfoMinimaxStrategy {
	return &PerfectInfoMinimaxStrategy{
		maxDepth: maxDepth,
	}
}

func (m *PerfectInfoMinimaxStrategy) GetName() string {
	return "PerfectInfoMinimax"
}

func (m *PerfectInfoMinimaxStrategy) SelectMove(state *game.GameState, validMoves []game.Card) game.Card {
	if len(validMoves) == 1 {
		return validMoves[0]
	}

	currentPlayer := state.CurrentPlayer
	isDeclarer := state.Declarer != nil && currentPlayer == *state.Declarer

	var bestMove game.Card
	var bestValue float64

	if isDeclarer {
		bestValue = math.Inf(-1) // Maximize for declarer
	} else {
		bestValue = math.Inf(1) // Minimize for defenders
	}

	for _, move := range validMoves {
		// Clone state and apply move
		nextState := state.Clone()
		nextState.PlayCard(move)

		// Resolve trick if complete
		if len(nextState.Trick) == 3 {
			nextState.ResolveTrick()
		}

		// Evaluate this move
		value := m.minimax(nextState, m.maxDepth-1, math.Inf(-1), math.Inf(1))

		// Declarer maximizes, defenders minimize
		if isDeclarer {
			if value > bestValue {
				bestValue = value
				bestMove = move
			}
		} else {
			if value < bestValue {
				bestValue = value
				bestMove = move
			}
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

	validMoves := state.GetValidMoves()
	if len(validMoves) == 0 {
		return m.evaluate(state)
	}

	isDeclarer := state.Declarer != nil && state.CurrentPlayer == *state.Declarer

	if isDeclarer {
		// Maximizing player (declarer)
		maxValue := math.Inf(-1)
		for _, move := range validMoves {
			nextState := state.Clone()
			nextState.PlayCard(move)

			if len(nextState.Trick) == 3 {
				nextState.ResolveTrick()
			}

			value := m.minimax(nextState, depth-1, alpha, beta)
			maxValue = math.Max(maxValue, value)
			alpha = math.Max(alpha, value)

			if beta <= alpha {
				break // Beta cutoff
			}
		}
		return maxValue
	} else {
		// Minimizing player (defenders)
		minValue := math.Inf(1)
		for _, move := range validMoves {
			nextState := state.Clone()
			nextState.PlayCard(move)

			if len(nextState.Trick) == 3 {
				nextState.ResolveTrick()
			}

			value := m.minimax(nextState, depth-1, alpha, beta)
			minValue = math.Min(minValue, value)
			beta = math.Min(beta, value)

			if beta <= alpha {
				break // Alpha cutoff
			}
		}
		return minValue
	}
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
