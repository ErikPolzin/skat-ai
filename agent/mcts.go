package agent

import (
	"math"
	"math/rand"
	"skat/game"
	"sync"
)

var (
	untrainedWarningShown = make(map[int]bool)
	untrainedWarningMutex sync.Mutex
)

// SelectMove uses MCTS to choose the best card to play
func (sa *SkatAgent) SelectMove(state *game.GameState, validMoves []game.Card) game.Card {
	if len(validMoves) == 1 {
		return validMoves[0]
	}

	moveCounts := make(map[game.Card]int)
	moveScores := make(map[game.Card]float64)

	for d := 0; d < int(sa.deterministicC); d++ {
		detState := sa.determinize(state)
		root := newMCTSNode(detState, nil, nil, state.CurrentPlayer)

		for i := 0; i < sa.simulations; i++ {
			node := sa.selectNode(root)
			reward := sa.simulate(node)
			sa.backpropagate(node, reward)
		}

		bestChild := sa.bestChild(root, 0)
		if bestChild != nil && bestChild.move != nil {
			moveCounts[*bestChild.move]++
			moveScores[*bestChild.move] += float64(bestChild.visits)
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

// MCTS helper methods
func (sa *SkatAgent) selectNode(node *MCTSNode) *MCTSNode {
	for len(node.untriedMoves) == 0 && len(node.children) > 0 {
		node = sa.bestChild(node, sa.explorationC)
	}

	if len(node.untriedMoves) > 0 {
		return sa.expand(node)
	}

	return node
}

func (sa *SkatAgent) expand(node *MCTSNode) *MCTSNode {
	moveIdx := rand.Intn(len(node.untriedMoves))
	move := node.untriedMoves[moveIdx]

	node.untriedMoves = append(node.untriedMoves[:moveIdx], node.untriedMoves[moveIdx+1:]...)

	childState := node.state.Clone()
	childState.PlayCard(move)

	// Resolve trick if complete
	if len(childState.Trick) == 3 {
		childState.ResolveTrick()
	}

	child := newMCTSNode(childState, node, &move, node.playerID)
	node.children = append(node.children, child)

	return child
}

func (sa *SkatAgent) simulate(node *MCTSNode) float64 {
	state := node.state.Clone()

	maxMoves := 50
	moves := 0
	for state.Phase == game.PhasePlaying && moves < maxMoves {
		validMoves := state.GetValidMoves()
		if len(validMoves) == 0 {
			break
		}
		move := validMoves[rand.Intn(len(validMoves))]
		state.PlayCard(move)

		// Resolve trick if complete
		if len(state.Trick) == 3 {
			state.ResolveTrick()
		}

		moves++
	}

	return sa.evaluateTerminalState(state, node.playerID)
}

func (sa *SkatAgent) evaluateTerminalState(state *game.GameState, playerID game.GamePosition) float64 {
	declarerWon := state.DeclarerScore >= 61

	if playerID == state.Declarer {
		if declarerWon {
			return 1.0
		}
		return 0.0
	} else {
		if declarerWon {
			return 0.0
		}
		return 1.0
	}
}

func (sa *SkatAgent) backpropagate(node *MCTSNode, reward float64) {
	for node != nil {
		node.visits++
		node.totalReward += reward
		node = node.parent
	}
}

func (sa *SkatAgent) bestChild(node *MCTSNode, c float64) *MCTSNode {
	if len(node.children) == 0 {
		return nil
	}

	var best *MCTSNode
	bestValue := math.Inf(-1)

	for _, child := range node.children {
		var ucb1 float64

		if child.visits == 0 {
			ucb1 = math.Inf(1)
		} else {
			exploit := child.totalReward / float64(child.visits)
			explore := c * math.Sqrt(math.Log(float64(node.visits))/float64(child.visits))
			ucb1 = exploit + explore
		}

		if ucb1 > bestValue {
			bestValue = ucb1
			best = child
		}
	}

	return best
}

func (sa *SkatAgent) determinize(state *game.GameState) *game.GameState {
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

// MCTSNode represents a node in the search tree
type MCTSNode struct {
	state        *game.GameState
	parent       *MCTSNode
	children     []*MCTSNode
	move         *game.Card
	visits       int
	totalReward  float64
	untriedMoves []game.Card
	playerID     game.GamePosition
}

func newMCTSNode(state *game.GameState, parent *MCTSNode, move *game.Card, playerID game.GamePosition) *MCTSNode {
	validMoves := state.GetValidMoves()
	return &MCTSNode{
		state:        state,
		parent:       parent,
		move:         move,
		visits:       0,
		totalReward:  0,
		untriedMoves: validMoves,
		playerID:     playerID,
	}
}
