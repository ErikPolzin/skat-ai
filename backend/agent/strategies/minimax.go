package strategies

import (
	"math"
	"skat/game"
	"sync"
)

// TranspositionEntry stores cached evaluation results
type TranspositionEntry struct {
	depth    int
	value    float64
	nodeType int
	bestMove game.Card
	hasBest  bool
}

const (
	transExact = iota
	transLower
	transUpper
)

const (
	minimaxWinProbIntercept = 1.202908

	DefaultMinimaxBaseDepth             = 12
	DefaultMinimaxDepthIncreasePerTrick = 2
)

const (
	minimaxFeaturePointMargin = iota
	minimaxFeatureRemainingMaterial
	minimaxFeatureHandPotential
	minimaxFeatureCurrentTrick
	minimaxFeatureTrumpControl
	minimaxFeatureHighCardControl
	minimaxFeatureSuitControl
	minimaxFeatureWinnerControl
	minimaxFeatureDefenderCoordination
	minimaxFeatureTricksRemaining
	minimaxFeatureDeclarerTurn
	minimaxFeatureModeGrand
	minimaxFeatureModeNull
	minimaxFeatureCount
)

type minimaxEvaluationFeature struct {
	name        string
	coefficient float64
}

var minimaxEvaluationFeatures = []minimaxEvaluationFeature{
	{name: "point_margin", coefficient: 4.569097},
	{name: "remaining_material", coefficient: 0.525141},
	{name: "hand_potential", coefficient: 1.262587},
	{name: "current_trick", coefficient: 1.020750},
	{name: "trump_control", coefficient: 3.308783},
	{name: "high_card_control", coefficient: 0.562981},
	{name: "suit_control", coefficient: -0.102827},
	{name: "winner_control", coefficient: 0.062937},
	{name: "defender_coordination", coefficient: 0.347723},
	{name: "tricks_remaining", coefficient: 0.408757},
	{name: "declarer_turn", coefficient: -0.007902},
	{name: "mode_grand", coefficient: -0.083206},
	{name: "mode_null", coefficient: 2.410103},
}

// MinimaxEvaluationFeatures contains scaled inputs for the calibrated win model.
type MinimaxEvaluationFeatures struct {
	Names  []string
	Values []float64
}

// PerfectInfoMinimaxStrategy implements minimax search with perfect information
// This is suitable for generating optimal training data where all hands are known
type PerfectInfoMinimaxStrategy struct {
	baseDepth             int
	maxDepth              int
	depthIncreasePerTrick int
	transTable            map[uint64]*TranspositionEntry
	transMutex            sync.RWMutex
	useMoveOrdering       bool
	useTransTable         bool
	useLateMoveRed        bool
	lateMoveThreshold     int
	lateMoveReduction     int
	handWinPredictor      *HandWinPredictor
	handWinMinSamples     uint64
}

// MinimaxSearchConfig controls search depth and optional tree-pruning aids.
// Late-move reduction is approximate: later ordered moves are searched fewer plies.
type MinimaxSearchConfig struct {
	BaseDepth             int
	MaxDepth              int
	DepthIncreasePerTrick int
	UseMoveOrdering       bool
	UseTransTable         bool
	UseLateMoveRed        bool
	LateMoveThreshold     int
	LateMoveReduction     int
	HandWinPredictor      *HandWinPredictor
	HandWinMinSamples     uint64
}

// DefaultMinimaxSearchConfig returns the strongest settings from the
// reproducible minimax-vs-heuristic evaluation sweep.
func DefaultMinimaxSearchConfig(depth int) MinimaxSearchConfig {
	return MinimaxSearchConfig{
		BaseDepth:             depth,
		MaxDepth:              30,
		DepthIncreasePerTrick: DefaultMinimaxDepthIncreasePerTrick,
		UseMoveOrdering:       true,
		UseTransTable:         true,
		UseLateMoveRed:        true,
		LateMoveThreshold:     2,
		LateMoveReduction:     4,
		HandWinMinSamples:     8,
	}
}

// NewPerfectInfoMinimaxStrategyWithDepth creates a strategy with custom depth
func NewPerfectInfoMinimaxStrategyWithDepth(depth int) *PerfectInfoMinimaxStrategy {
	return NewPerfectInfoMinimaxStrategyWithConfig(DefaultMinimaxSearchConfig(depth))
}

// NewPerfectInfoMinimaxStrategyWithConfig creates a strategy with explicit
// search and pruning settings.
func NewPerfectInfoMinimaxStrategyWithConfig(config MinimaxSearchConfig) *PerfectInfoMinimaxStrategy {
	if config.HandWinPredictor != nil && config.HandWinMinSamples == 0 {
		config.HandWinMinSamples = 8
	}
	return &PerfectInfoMinimaxStrategy{
		baseDepth:             config.BaseDepth,
		depthIncreasePerTrick: config.DepthIncreasePerTrick,
		maxDepth:              config.MaxDepth,
		transTable:            make(map[uint64]*TranspositionEntry),
		useMoveOrdering:       config.UseMoveOrdering,
		useTransTable:         config.UseTransTable,
		useLateMoveRed:        config.UseLateMoveRed,
		lateMoveThreshold:     config.LateMoveThreshold,
		lateMoveReduction:     config.LateMoveReduction,
		handWinPredictor:      config.HandWinPredictor,
		handWinMinSamples:     config.HandWinMinSamples,
	}
}

func (m *PerfectInfoMinimaxStrategy) GetName() string {
	return "PerfectInfoMinimax"
}

func (m *PerfectInfoMinimaxStrategy) Clone() *PerfectInfoMinimaxStrategy {
	return NewPerfectInfoMinimaxStrategyWithConfig(MinimaxSearchConfig{
		BaseDepth: m.baseDepth, MaxDepth: m.maxDepth,
		DepthIncreasePerTrick: m.depthIncreasePerTrick,
		UseMoveOrdering:       m.useMoveOrdering, UseTransTable: m.useTransTable,
		UseLateMoveRed: m.useLateMoveRed, LateMoveThreshold: m.lateMoveThreshold,
		LateMoveReduction: m.lateMoveReduction, HandWinPredictor: m.handWinPredictor,
		HandWinMinSamples: m.handWinMinSamples,
	})
}

func (d *PerfectInfoMinimaxStrategy) SearchDepth(state *game.GameState) int {
	depth := d.baseDepth + len(state.CardsPlayed)*d.depthIncreasePerTrick
	remaining := 0
	for _, player := range state.Players {
		if player != nil {
			remaining += len(player.Hand)
		}
	}
	depth = min(depth, d.maxDepth, remaining)
	return max(1, depth)
}

// ScoreMoves evaluates every legal root move independently. Normal-game scores
// are from the declarer's perspective (higher is better for the declarer), while
// Ramsch scores are from the current player's perspective (higher is better).
// Independent full-window root searches make these values suitable for soft
// supervised card-play targets.
func (m *PerfectInfoMinimaxStrategy) ScoreMoves(state *game.GameState, validMoves []game.Card) []float64 {
	scores := make([]float64, len(validMoves))
	root := state.CurrentPlayer
	currentDepth := m.SearchDepth(state)

	for i, move := range validMoves {
		if m.useTransTable {
			m.transMutex.Lock()
			m.transTable = make(map[uint64]*TranspositionEntry)
			m.transMutex.Unlock()
		}

		next := state.Clone()
		m.playAndResolve(next, move)
		if state.Mode == game.ModeRamsch {
			scores[i] = m.minimaxRamsch(next, currentDepth-1, math.Inf(-1), math.Inf(1), root)
		} else {
			scores[i] = m.minimax(next, currentDepth-1, math.Inf(-1), math.Inf(1))
		}
	}

	return scores
}

func (m *PerfectInfoMinimaxStrategy) SelectMove(state *game.GameState, validMoves []game.Card) game.Card {
	if len(validMoves) == 1 {
		return validMoves[0]
	}
	currentDepth := m.SearchDepth(state)

	// Clear transposition table for new move selection
	if m.useTransTable {
		m.transMutex.Lock()
		m.transTable = make(map[uint64]*TranspositionEntry)
		m.transMutex.Unlock()
	}

	currentPlayer := state.CurrentPlayer
	if state.Mode == game.ModeRamsch {
		return m.selectRamschMove(state, validMoves, currentPlayer, currentDepth)
	}
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
		m.playAndResolve(nextState, move)

		// Evaluate this move
		value := m.minimax(nextState, currentDepth-1, alpha, beta)

		// Declarer maximizes, defenders minimize
		if isDeclarer {
			if value > bestValue {
				bestValue = value
				bestMove = move
			}
			alpha = math.Max(alpha, value)
			// Win probabilities cannot exceed one, so no remaining move can
			// improve on a proven declarer win.
			if bestValue >= 1.0 {
				break
			}
		} else {
			if value < bestValue {
				bestValue = value
				bestMove = move
			}
			beta = math.Min(beta, value)
			// Defenders minimize declarer win probability. Zero is therefore
			// a proven defender win and cannot be improved upon.
			if bestValue <= 0.0 {
				break
			}
		}
	}

	return bestMove
}

func (m *PerfectInfoMinimaxStrategy) selectRamschMove(state *game.GameState, validMoves []game.Card, root game.GamePosition, depth int) game.Card {
	bestMove, bestValue := validMoves[0], math.Inf(-1)
	for _, move := range validMoves {
		next := state.Clone()
		m.playAndResolve(next, move)
		value := m.minimaxRamsch(next, depth-1, math.Inf(-1), math.Inf(1), root)
		if value > bestValue {
			bestMove, bestValue = move, value
		}
	}
	return bestMove
}

func (m *PerfectInfoMinimaxStrategy) minimaxRamsch(state *game.GameState, depth int, alpha, beta float64, root game.GamePosition) float64 {
	if state.Phase != game.PhasePlaying || (depth <= 0 && len(state.Trick) == 0) {
		score := float64(-state.PlayerScores[root])
		if state.Phase == game.PhaseComplete {
			minScore := state.PlayerScores[0]
			for _, points := range state.PlayerScores[1:] {
				if points < minScore {
					minScore = points
				}
			}
			if state.PlayerScores[root] == minScore {
				score += 1000
			} else {
				score -= 1000
			}
		}
		return score
	}
	moves := state.GetValidMoves()
	maximizing := state.CurrentPlayer == root
	value := math.Inf(1)
	if maximizing {
		value = math.Inf(-1)
	}
	for _, move := range moves {
		next := state.Clone()
		m.playAndResolve(next, move)
		child := m.minimaxRamsch(next, depth-1, alpha, beta, root)
		if maximizing {
			value = math.Max(value, child)
			alpha = math.Max(alpha, value)
		} else {
			value = math.Min(value, child)
			beta = math.Min(beta, value)
		}
		if beta <= alpha {
			break
		}
	}
	return value
}

func (m *PerfectInfoMinimaxStrategy) playAndResolve(state *game.GameState, move game.Card) {
	state.PlayCard(move)
	if len(state.Trick) == 3 {
		state.ResolveTrick()
	}
}

// minimax performs alpha-beta pruning minimax search
func (m *PerfectInfoMinimaxStrategy) minimax(state *game.GameState, depth int, alpha, beta float64) float64 {
	// Do not evaluate halfway through a trick. Extending the leaf by at most two
	// plies prevents the current winner from being credited before every player
	// has had the chance to beat it.
	if state.Phase != game.PhasePlaying || (depth <= 0 && len(state.Trick) == 0) {
		return m.evaluate(state)
	}

	originalAlpha, originalBeta := alpha, beta
	var cachedBest game.Card
	hasCachedBest := false

	// Check transposition table
	if m.useTransTable {
		hash := m.hashState(state)
		m.transMutex.RLock()
		entry, found := m.transTable[hash]
		m.transMutex.RUnlock()

		if found && entry.depth >= depth {
			switch entry.nodeType {
			case transExact:
				return entry.value
			case transLower:
				alpha = math.Max(alpha, entry.value)
			case transUpper:
				beta = math.Min(beta, entry.value)
			}
			if beta <= alpha {
				return entry.value
			}
			if entry.hasBest {
				cachedBest, hasCachedBest = entry.bestMove, true
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
	if hasCachedBest {
		prioritizeMove(validMoves, cachedBest)
	}

	var value float64
	var bestMove game.Card
	hasBest := false
	provenResult := false
	if isDeclarer {
		// Maximizing player (declarer)
		maxValue := math.Inf(-1)
		for i, move := range validMoves {
			nextState := state.Clone()
			m.playAndResolve(nextState, move)

			// Late move reduction: search less promising moves at reduced depth
			searchDepth := depth - 1
			if m.useLateMoveRed && state.Mode != game.ModeNull && i >= m.lateMoveThreshold && depth >= m.lateMoveReduction+1 {
				searchDepth = depth - 1 - m.lateMoveReduction
			}

			value = m.minimax(nextState, searchDepth, alpha, beta)
			if value > maxValue {
				maxValue = value
				bestMove = move
				hasBest = true
			}
			alpha = math.Max(alpha, value)

			if maxValue >= 1.0 {
				provenResult = true
				break // No sibling can improve on a proven declarer win.
			}
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
			m.playAndResolve(nextState, move)

			// Late move reduction
			searchDepth := depth - 1
			if m.useLateMoveRed && state.Mode != game.ModeNull && i >= m.lateMoveThreshold && depth >= m.lateMoveReduction+1 {
				searchDepth = depth - 1 - m.lateMoveReduction
			}

			value = m.minimax(nextState, searchDepth, alpha, beta)
			if value < minValue {
				minValue = value
				bestMove = move
				hasBest = true
			}
			beta = math.Min(beta, value)

			if minValue <= 0.0 {
				provenResult = true
				break // No sibling can improve on a proven defender win.
			}
			if beta <= alpha {
				break // Alpha cutoff
			}
		}
		value = minValue
	}

	// Store in transposition table
	if m.useTransTable {
		nodeType := transExact
		if provenResult {
			nodeType = transExact
		} else if value <= originalAlpha {
			nodeType = transUpper
		} else if value >= originalBeta {
			nodeType = transLower
		}
		hash := m.hashState(state)
		m.transMutex.Lock()
		m.transTable[hash] = &TranspositionEntry{
			depth:    depth,
			value:    value,
			nodeType: nodeType,
			bestMove: bestMove,
			hasBest:  hasBest,
		}
		m.transMutex.Unlock()
	}

	return value
}

// orderMoves sorts moves to improve alpha-beta pruning efficiency
// Uses heuristic-based ordering to prioritize moves likely to be good
func (m *PerfectInfoMinimaxStrategy) orderMoves(state *game.GameState, moves []game.Card, isDeclarer bool) {
	// Use heuristic-based move ordering for better pruning
	heuristicOrder(state, moves, isDeclarer)
}

func prioritizeMove(moves []game.Card, move game.Card) {
	for i := range moves {
		if moves[i] == move {
			if i > 0 {
				copy(moves[1:i+1], moves[0:i])
				moves[0] = move
			}
			return
		}
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

func hashGameMode(mode game.GameMode) uint64 {
	var hash uint64
	for _, r := range string(mode) {
		hash = hash*31 + uint64(r)
	}
	return hash
}

// EvaluateState returns estimated declarer win probability for this state.
func (m *PerfectInfoMinimaxStrategy) EvaluateState(state *game.GameState) float64 {
	return m.EvaluateStateEstimate(state).WinProbability
}

// MinimaxWinEstimate reports win probability and empirical confidence error.
type MinimaxWinEstimate struct {
	WinProbability       float64
	Error                float64
	Samples              uint64
	PredictorLevel       int
	FromHandWinPredictor bool
}

func (m *PerfectInfoMinimaxStrategy) EvaluateStateEstimate(state *game.GameState) MinimaxWinEstimate {
	if state.Declarer == nil {
		return MinimaxWinEstimate{WinProbability: 0.5, Error: 0.5}
	}
	if state.Phase == game.PhaseComplete {
		return MinimaxWinEstimate{WinProbability: m.evaluateTerminalProbability(state)}
	}
	if state.Mode == game.ModeNull {
		probability := m.evaluateNullStateProbability(state)
		return MinimaxWinEstimate{WinProbability: probability, Error: math.Sqrt(probability * (1 - probability))}
	}
	if m.handWinPredictor != nil {
		if estimate, ok := m.handWinPredictor.Lookup(state, m.handWinMinSamples); ok {
			return MinimaxWinEstimate{
				WinProbability: estimate.WinProbability, Error: estimate.Error,
				Samples: estimate.Samples, PredictorLevel: estimate.Level, FromHandWinPredictor: true,
			}
		}
	}
	probability := calibratedMinimaxWinProbability(m.evaluateFeatureValues(state))
	return MinimaxWinEstimate{WinProbability: probability, Error: math.Sqrt(probability * (1 - probability))}
}

// evaluateNullStateProbability avoids applying point-game calibration to Null.
// Card points, trump control, and a positive high-card balance all have either
// no meaning or the opposite meaning when taking a single trick loses. The
// hand estimator supplies a null-specific risk model; surviving later into the
// game adds a modest bonus without overwhelming the remaining hand shape.
func (m *PerfectInfoMinimaxStrategy) evaluateNullStateProbability(state *game.GameState) float64 {
	declarerHand := state.Players[*state.Declarer].Hand
	probability := NewHeuristicContractWinProbabilityEstimator().EstimateWinProbability(
		declarerHand, game.ModeNull, game.NoSuit,
	)
	progress := 1.0 - float64(len(declarerHand))/10.0
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}
	return probability + (1-probability)*0.35*progress
}

// EvaluateFeatures exposes the calibrated model inputs for data-driven fitting.
func (m *PerfectInfoMinimaxStrategy) EvaluateFeatures(state *game.GameState) MinimaxEvaluationFeatures {
	return MinimaxEvaluationFeatures{
		Names:  minimaxEvaluationFeatureNames(),
		Values: m.evaluateFeatureValues(state),
	}
}

// evaluate returns declarer win probability.
func (m *PerfectInfoMinimaxStrategy) evaluate(state *game.GameState) float64 {
	return m.EvaluateStateEstimate(state).WinProbability
}

func calibratedMinimaxWinProbability(features []float64) float64 {
	logit := minimaxWinProbIntercept
	for i, value := range features {
		if i >= len(minimaxEvaluationFeatures) {
			break
		}
		logit += minimaxEvaluationFeatures[i].coefficient * value
	}
	return minimaxSigmoid(logit)
}

func minimaxEvaluationFeatureNames() []string {
	names := make([]string, len(minimaxEvaluationFeatures))
	for i, feature := range minimaxEvaluationFeatures {
		names[i] = feature.name
	}
	return names
}

func minimaxSigmoid(x float64) float64 {
	if x >= 0 {
		z := math.Exp(-x)
		return 1 / (1 + z)
	}
	z := math.Exp(x)
	return z / (1 + z)
}

func (m *PerfectInfoMinimaxStrategy) evaluateTerminalProbability(state *game.GameState) float64 {
	declarerWon, _, _ := state.GetGameResult()
	if state.Overbid {
		declarerWon = false
	}
	if declarerWon {
		return 1.0
	}
	return 0.0
}

func (m *PerfectInfoMinimaxStrategy) evaluateFeatureValues(state *game.GameState) []float64 {
	values := make([]float64, minimaxFeatureCount)
	if state.Declarer == nil {
		return values
	}
	declarer := *state.Declarer
	pointMargin, remainingMaterial, handPotential, currentTrick := m.evaluateMaterialParts(state, declarer)
	tricksRemaining := 0
	for p := 0; p < 3; p++ {
		tricksRemaining += len(state.Players[p].Hand)
	}
	tricksRemaining /= 3

	values[minimaxFeaturePointMargin] = pointMargin / 120.0
	values[minimaxFeatureRemainingMaterial] = remainingMaterial / 120.0
	values[minimaxFeatureHandPotential] = handPotential / 120.0
	values[minimaxFeatureCurrentTrick] = currentTrick / 30.0
	values[minimaxFeatureTrumpControl] = m.evaluateTrumpControl(state, declarer)
	values[minimaxFeatureHighCardControl] = m.evaluateHighCardControl(state, declarer)
	values[minimaxFeatureSuitControl] = m.evaluateSuitControl(state, declarer)
	if state.CurrentPlayer == declarer {
		values[minimaxFeatureWinnerControl] = m.evaluateWinnerControl(state, declarer)
		values[minimaxFeatureDeclarerTurn] = 1
	} else {
		values[minimaxFeatureDefenderCoordination] = m.evaluateDefenderCoordination(state, declarer)
		values[minimaxFeatureDeclarerTurn] = -1
	}
	values[minimaxFeatureTricksRemaining] = float64(tricksRemaining) / 10.0
	if state.Mode == game.ModeGrand {
		values[minimaxFeatureModeGrand] = 1
	}
	if state.Mode == game.ModeNull {
		values[minimaxFeatureModeNull] = 1
	}
	return values
}

func (m *PerfectInfoMinimaxStrategy) evaluateMaterialParts(state *game.GameState, declarer game.GamePosition) (pointMargin, remainingMaterial, handPotential, currentTrick float64) {
	// Start with the actual point margin. Using only the declarer's score made
	// points already captured by defenders disappear from cutoff evaluation.
	pointMargin = float64(state.DeclarerCardScore() - state.OpponentCardScore())

	// Add remaining card values in hands
	for p := 0; p < 3; p++ {
		pos := game.GamePosition(p)
		for _, card := range state.Players[p].Hand {
			cardValue := float64(card.Value())
			if pos == declarer {
				remainingMaterial += cardValue
			} else {
				remainingMaterial -= cardValue
			}
		}
	}

	handPotential = m.evaluateHandPotential(state, declarer)

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
			currentTrick += float64(trickValue)
		} else {
			currentTrick -= float64(trickValue)
		}
	}

	return pointMargin, remainingMaterial, handPotential, currentTrick
}

func (m *PerfectInfoMinimaxStrategy) evaluateHandPotential(state *game.GameState, declarer game.GamePosition) float64 {
	if state.Mode == game.ModeNull {
		return m.evaluateNullHandPotential(state, declarer)
	}

	score := 0.0
	for pos := game.Dealer; pos <= game.Speaker; pos++ {
		player := state.Players[pos]
		if player == nil {
			continue
		}
		side := -1.0
		if pos == declarer {
			side = 1.0
		}
		for _, card := range player.Hand {
			value := float64(card.Value())
			strongerAgainst := m.countStrongerCardsInOtherHands(state, pos, card)
			weakerPartnerPoints := m.availablePartnerPointSupport(state, declarer, pos, card)
			if strongerAgainst == 0 {
				score += side * (value + 3.0 + weakerPartnerPoints*0.25)
			} else if value >= 10 {
				score -= side * (value * 0.35 * float64(strongerAgainst))
			}
		}
	}
	return score
}

func (m *PerfectInfoMinimaxStrategy) evaluateNullHandPotential(state *game.GameState, declarer game.GamePosition) float64 {
	score := 0.0
	for pos := game.Dealer; pos <= game.Speaker; pos++ {
		player := state.Players[pos]
		if player == nil {
			continue
		}
		side := -1.0
		if pos == declarer {
			side = 1.0
		}
		for _, card := range player.Hand {
			strongerAgainst := m.countStrongerCardsInOtherHands(state, pos, card)
			if strongerAgainst == 0 {
				score -= side * 8.0
			}
			if card.NullRank() >= game.Queen.NullRank() {
				score -= side * 2.0
			}
		}
	}
	return score
}

func (m *PerfectInfoMinimaxStrategy) evaluateWinnerControl(state *game.GameState, declarer game.GamePosition) float64 {
	declarerWinners, defenderWinners := 0.0, 0.0
	for pos := game.Dealer; pos <= game.Speaker; pos++ {
		player := state.Players[pos]
		if player == nil {
			continue
		}
		for _, card := range player.Hand {
			if m.countStrongerCardsInOtherHands(state, pos, card) != 0 {
				continue
			}
			weight := 1.0 + float64(card.Value())/10.0
			if state.TrumpValue(card) > 0 {
				weight += float64(state.TrumpValue(card)) / 8.0
			}
			if pos == declarer {
				declarerWinners += weight
			} else {
				defenderWinners += weight
			}
		}
	}
	total := declarerWinners + defenderWinners
	if total == 0 {
		return 0
	}
	return (declarerWinners - defenderWinners) / total
}

func (m *PerfectInfoMinimaxStrategy) evaluateDefenderCoordination(state *game.GameState, declarer game.GamePosition) float64 {
	if state.Mode == game.ModeNull {
		return 0
	}
	score := 0.0
	for suit := game.Clubs; suit <= game.Diamonds; suit++ {
		if state.Mode == game.ModeSuit && suit == state.TrumpSuit {
			continue
		}
		declarerCount := m.countEffectiveSuit(state, declarer, suit)
		defenderVoidCount := 0
		defenderLongCount := 0
		for pos := game.Dealer; pos <= game.Speaker; pos++ {
			if pos == declarer {
				continue
			}
			count := m.countEffectiveSuit(state, pos, suit)
			if count == 0 {
				defenderVoidCount++
			}
			if count >= 3 {
				defenderLongCount++
			}
		}
		if declarerCount >= 3 && defenderVoidCount > 0 {
			score -= 0.25 * float64(defenderVoidCount)
		}
		if declarerCount == 0 {
			score += 0.2
		}
		if defenderLongCount > 0 {
			score -= 0.15 * float64(defenderLongCount)
		}
	}
	return score
}

func (m *PerfectInfoMinimaxStrategy) countStrongerCardsInOtherHands(state *game.GameState, owner game.GamePosition, card game.Card) int {
	count := 0
	for pos := game.Dealer; pos <= game.Speaker; pos++ {
		if pos == owner || state.Players[pos] == nil {
			continue
		}
		for _, other := range state.Players[pos].Hand {
			if other.EffectiveSuit(state.Mode, state.TrumpSuit) == card.EffectiveSuit(state.Mode, state.TrumpSuit) && state.CardBeats(other, card) {
				count++
			}
		}
	}
	return count
}

func (m *PerfectInfoMinimaxStrategy) availablePartnerPointSupport(state *game.GameState, declarer, owner game.GamePosition, card game.Card) float64 {
	if owner == declarer {
		return 0
	}
	points := 0.0
	for pos := game.Dealer; pos <= game.Speaker; pos++ {
		if pos == declarer || pos == owner || state.Players[pos] == nil {
			continue
		}
		for _, partnerCard := range state.Players[pos].Hand {
			if partnerCard.EffectiveSuit(state.Mode, state.TrumpSuit) == card.EffectiveSuit(state.Mode, state.TrumpSuit) && !state.CardBeats(partnerCard, card) {
				points += float64(partnerCard.Value())
			}
		}
	}
	return points
}

func (m *PerfectInfoMinimaxStrategy) countEffectiveSuit(state *game.GameState, pos game.GamePosition, suit game.Suit) int {
	if state.Players[pos] == nil {
		return 0
	}
	count := 0
	for _, card := range state.Players[pos].Hand {
		if card.EffectiveSuit(state.Mode, state.TrumpSuit) == suit {
			count++
		}
	}
	return count
}

// evaluateTrumpControl returns -1 to +1 (negative favors defenders)
func (m *PerfectInfoMinimaxStrategy) evaluateTrumpControl(state *game.GameState, declarer game.GamePosition) float64 {
	declarerTrumps := 0
	defenderTrumps := 0

	for p := 0; p < 3; p++ {
		pos := game.GamePosition(p)
		for _, card := range state.Players[p].Hand {
			if card.IsTrump(state.Mode, state.TrumpSuit) {
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
