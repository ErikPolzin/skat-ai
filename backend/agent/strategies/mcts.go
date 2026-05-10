package strategies

import (
	"math"
	"math/rand"
	"skat/game"
)

// MCTSNode represents a node in the search tree
type MCTSNode struct {
	State        *game.GameState
	Parent       *MCTSNode
	Children     []*MCTSNode
	Move         *game.Card
	Visits       int
	TotalReward  float64
	UntriedMoves []game.Card
	PlayerID     game.GamePosition
}

// NewMCTSNode creates a new MCTS node
func NewMCTSNode(state *game.GameState, parent *MCTSNode, move *game.Card, playerID game.GamePosition) *MCTSNode {
	validMoves := state.GetValidMoves()
	return &MCTSNode{
		State:        state,
		Parent:       parent,
		Move:         move,
		Visits:       0,
		TotalReward:  0,
		UntriedMoves: validMoves,
		PlayerID:     playerID,
	}
}

// MCTSCardPlayStrategy uses Monte Carlo Tree Search
type MCTSCardPlayStrategy struct {
	simulations    int
	explorationC   float64
	deterministicC int
}

// NewMCTSCardPlayStrategy creates a new MCTS strategy with default parameters
func NewMCTSCardPlayStrategy() *MCTSCardPlayStrategy {
	return &MCTSCardPlayStrategy{
		simulations:    1000,
		explorationC:   1.41,
		deterministicC: 50,
	}
}

// NewMCTSCardPlayStrategyWithParams creates a new MCTS strategy with custom parameters
func NewMCTSCardPlayStrategyWithParams(simulations int, explorationC float64, deterministicC int) *MCTSCardPlayStrategy {
	return &MCTSCardPlayStrategy{
		simulations:    simulations,
		explorationC:   explorationC,
		deterministicC: deterministicC,
	}
}

func (m *MCTSCardPlayStrategy) GetName() string {
	return "MCTSCardPlay"
}

func (m *MCTSCardPlayStrategy) SelectMove(state *game.GameState, validMoves []game.Card) game.Card {
	if len(validMoves) == 1 {
		return validMoves[0]
	}

	moveCounts := make(map[game.Card]int)
	moveScores := make(map[game.Card]float64)

	for d := 0; d < m.deterministicC; d++ {
		detState := m.determinize(state)
		root := NewMCTSNode(detState, nil, nil, state.CurrentPlayer)

		for i := 0; i < m.simulations; i++ {
			node := m.selectNode(root)
			reward := m.simulate(node)
			m.backpropagate(node, reward)
		}

		bestChild := m.bestChild(root, 0)
		if bestChild != nil && bestChild.Move != nil {
			moveCounts[*bestChild.Move]++
			moveScores[*bestChild.Move] += float64(bestChild.Visits)
		}
	}

	var bestMove game.Card
	bestCount := 0
	for move, count := range moveCounts {
		if count > bestCount {
			bestCount = count
			bestMove = move
		}
	}

	if bestCount == 0 {
		return validMoves[rand.Intn(len(validMoves))]
	}

	return bestMove
}

// MCTS algorithm implementation

func (m *MCTSCardPlayStrategy) selectNode(node *MCTSNode) *MCTSNode {
	for len(node.UntriedMoves) == 0 && len(node.Children) > 0 {
		node = m.bestChild(node, m.explorationC)
	}

	if len(node.UntriedMoves) > 0 {
		return m.expand(node)
	}

	return node
}

func (m *MCTSCardPlayStrategy) expand(node *MCTSNode) *MCTSNode {
	moveIdx := rand.Intn(len(node.UntriedMoves))
	move := node.UntriedMoves[moveIdx]

	node.UntriedMoves = append(node.UntriedMoves[:moveIdx], node.UntriedMoves[moveIdx+1:]...)

	childState := node.State.Clone()
	childState.PlayCard(move)

	// Resolve trick if complete
	if len(childState.Trick) == 3 {
		childState.ResolveTrick()
	}

	child := NewMCTSNode(childState, node, &move, node.PlayerID)
	node.Children = append(node.Children, child)

	return child
}

func (m *MCTSCardPlayStrategy) simulate(node *MCTSNode) float64 {
	state := node.State.Clone()

	maxMoves := 50
	moves := 0
	for state.Phase == game.PhasePlaying && moves < maxMoves {
		validMoves := state.GetValidMoves()
		if len(validMoves) == 0 {
			break
		}

		move := m.selectRolloutMove(state, validMoves, node.PlayerID)
		state.PlayCard(move)

		if len(state.Trick) == 3 {
			state.ResolveTrick()
		}

		moves++
	}

	return m.evaluateTerminalState(state, node.PlayerID)
}

func (m *MCTSCardPlayStrategy) backpropagate(node *MCTSNode, reward float64) {
	currentReward := reward
	for node != nil {
		node.Visits++
		node.TotalReward += currentReward
		currentReward = -currentReward // Flip perspective for alternating players
		node = node.Parent
	}
}

func (m *MCTSCardPlayStrategy) bestChild(node *MCTSNode, c float64) *MCTSNode {
	if len(node.Children) == 0 {
		return nil
	}

	var best *MCTSNode
	bestValue := math.Inf(-1)

	for _, child := range node.Children {
		var ucb1 float64

		if child.Visits == 0 {
			ucb1 = math.Inf(1)
		} else {
			exploit := child.TotalReward / float64(child.Visits)
			explore := c * math.Sqrt(math.Log(float64(node.Visits))/float64(child.Visits))
			ucb1 = exploit + explore
		}

		if ucb1 > bestValue {
			bestValue = ucb1
			best = child
		}
	}

	return best
}

func (m *MCTSCardPlayStrategy) determinize(state *game.GameState) *game.GameState {
	det := state.Clone()
	currentPlayerID := state.CurrentPlayer

	seenCards := make(map[game.Card]bool)

	for _, card := range state.Players[currentPlayerID].Hand {
		seenCards[card] = true
	}

	for _, trick := range state.CardsPlayed {
		for _, card := range trick {
			seenCards[card] = true
		}
	}

	for _, card := range state.Trick {
		seenCards[card] = true
	}

	allCards := game.NewDeck()
	unseenCards := make([]game.Card, 0)
	for _, card := range allCards {
		if !seenCards[card] {
			unseenCards = append(unseenCards, card)
		}
	}

	rand.Shuffle(len(unseenCards), func(i, j int) {
		unseenCards[i], unseenCards[j] = unseenCards[j], unseenCards[i]
	})

	idx := 0
	for p := 0; p < 3; p++ {
		if p == int(currentPlayerID) {
			continue
		}

		expectedCards := len(state.Players[p].Hand)
		if expectedCards < 0 {
			expectedCards = 0
		}
		if expectedCards > len(unseenCards)-idx {
			expectedCards = len(unseenCards) - idx
		}

		det.Players[p].Hand = make([]game.Card, 0, expectedCards)
		for i := 0; i < expectedCards && idx < len(unseenCards); i++ {
			det.Players[p].Hand = append(det.Players[p].Hand, unseenCards[idx])
			idx++
		}
	}

	if idx < len(unseenCards) && idx+1 < len(unseenCards) {
		det.Skat[0] = unseenCards[idx]
		det.Skat[1] = unseenCards[idx+1]
	}

	return det
}

// Rollout heuristics

func (m *MCTSCardPlayStrategy) selectRolloutMove(state *game.GameState, validMoves []game.Card, rootPlayerID game.GamePosition) game.Card {
	if len(validMoves) == 1 {
		return validMoves[0]
	}

	currentPlayer := state.CurrentPlayer
	isDefender := state.Declarer == nil || currentPlayer != *state.Declarer

	if isDefender {
		return m.selectDefenderRolloutMove(state, validMoves)
	}

	return m.selectDeclarerRolloutMove(state, validMoves)
}

func (m *MCTSCardPlayStrategy) selectDefenderRolloutMove(state *game.GameState, validMoves []game.Card) game.Card {
	trick := state.Trick

	if len(trick) == 0 {
		// Lead with trump or high card
		for i := len(validMoves) - 1; i >= 0; i-- {
			move := validMoves[i]
			if m.isTrump(state, move) {
				return move
			}
		}
		for _, move := range validMoves {
			if move.Rank == game.Ace {
				return move
			}
		}
		return validMoves[len(validMoves)-1]
	}

	if len(trick) == 2 {
		trickWinner := m.getTrickWinner(state)
		partnerPos := m.getDefenderPartner(state)

		if trickWinner == partnerPos {
			return validMoves[0]
		}
	}

	currentWinner := m.getTrickWinner(state)

	if state.Declarer != nil && currentWinner == *state.Declarer {
		for i := len(validMoves) - 1; i >= 0; i-- {
			move := validMoves[i]
			wouldWin := true
			for _, trickCard := range trick {
				if !state.CardBeats(move, trickCard) {
					wouldWin = false
					break
				}
			}
			if wouldWin {
				return move
			}
		}
	}

	return validMoves[0]
}

func (m *MCTSCardPlayStrategy) selectDeclarerRolloutMove(state *game.GameState, validMoves []game.Card) game.Card {
	trick := state.Trick

	if len(trick) == 0 {
		for _, move := range validMoves {
			if move.Rank == game.Ace || move.Rank == game.Ten {
				return move
			}
		}
		return validMoves[len(validMoves)-1]
	}

	for _, move := range validMoves {
		wouldWin := true
		for _, trickCard := range trick {
			if !state.CardBeats(move, trickCard) {
				wouldWin = false
				break
			}
		}
		if wouldWin {
			return move
		}
	}

	return validMoves[0]
}

// Helper functions

func (m *MCTSCardPlayStrategy) getDefenderPartner(state *game.GameState) game.GamePosition {
	currentPlayer := state.CurrentPlayer
	for pos := game.Dealer; pos <= game.Speaker; pos++ {
		if pos != currentPlayer && (state.Declarer == nil || pos != *state.Declarer) {
			return pos
		}
	}
	return game.Dealer
}

func (m *MCTSCardPlayStrategy) getTrickWinner(state *game.GameState) game.GamePosition {
	if len(state.Trick) == 0 {
		return state.CurrentPlayer
	}

	winner := state.TrickStarter
	winningCard := state.Trick[0]

	for i := 1; i < len(state.Trick); i++ {
		if state.CardBeats(state.Trick[i], winningCard) {
			winner = (state.TrickStarter + game.GamePosition(i)) % 3
			winningCard = state.Trick[i]
		}
	}

	return winner
}

func (m *MCTSCardPlayStrategy) isTrump(state *game.GameState, card game.Card) bool {
	if card.Rank == game.Jack {
		return true
	}
	if state.Mode == game.ModeSuit && card.Suit == state.TrumpSuit {
		return true
	}
	return false
}

func (m *MCTSCardPlayStrategy) evaluateTerminalState(state *game.GameState, playerID game.GamePosition) float64 {
	isPlayerDeclarer := state.Declarer != nil && playerID == *state.Declarer
	declarerPoints := float64(state.DeclarerScore)

	// Normalize to [-1, 1] range centered at 61 (winning threshold)
	// Score of 61 -> 0.0, score of 120 -> ~1.0, score of 0 -> ~-1.0
	normalizedScore := (declarerPoints - 61.0) / 60.0

	// Clamp to [-1, 1]
	if normalizedScore > 1.0 {
		normalizedScore = 1.0
	}
	if normalizedScore < -1.0 {
		normalizedScore = -1.0
	}

	if isPlayerDeclarer {
		return normalizedScore
	}
	return -normalizedScore
}
