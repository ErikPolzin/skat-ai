package agent

import (
	"math"
	"math/rand"
	"skat/game"
)

// MCTSAgent uses Monte Carlo Tree Search to select moves
type MCTSAgent struct {
	name           string
	simulations    int     // Number of simulations per move
	explorationC   float64 // UCB1 exploration constant
	deterministicC float64 // Determinization count
}

func NewMCTSAgent(name string, simulations int) *MCTSAgent {
	return &MCTSAgent{
		name:           name,
		simulations:    simulations,
		explorationC:   1.41, // sqrt(2) - standard UCB1 value
		deterministicC: 10,   // Number of determinizations for imperfect info
	}
}

// SetSimulations updates the simulation count
func (m *MCTSAgent) SetSimulations(sims int) {
	m.simulations = sims
}

// MCTSNode represents a node in the search tree
type MCTSNode struct {
	state        *game.GameState
	parent       *MCTSNode
	children     []*MCTSNode
	move         *game.Card // Move that led to this node
	visits       int
	totalReward  float64
	untriedMoves []game.Card
	playerID     game.GamePosition // Which player's perspective
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

func (m *MCTSAgent) SelectMove(state *game.GameState, validMoves []game.Card) game.Card {
	if len(validMoves) == 1 {
		return validMoves[0]
	}

	// Information Set MCTS (IS-MCTS) for imperfect information
	// Run multiple determinizations
	moveCounts := make(map[game.Card]int)
	moveScores := make(map[game.Card]float64)

	for d := 0; d < int(m.deterministicC); d++ {
		// Create a determinization (assign unknown cards randomly)
		detState := m.determinize(state)

		// Run MCTS on this determinization
		root := newMCTSNode(detState, nil, nil, state.CurrentPlayer)

		for i := 0; i < m.simulations; i++ {
			// MCTS phases: Select, Expand, Simulate, Backpropagate
			node := m.selectNode(root)
			reward := m.simulate(node)
			m.backpropagate(node, reward)
		}

		// Get best move from this determinization
		bestChild := m.bestChild(root, 0) // exploitation only
		if bestChild != nil && bestChild.move != nil {
			moveCounts[*bestChild.move]++
			moveScores[*bestChild.move] += float64(bestChild.visits)
		}
	}

	// Select move that was chosen most often across determinizations
	var bestMove game.Card
	bestCount := 0
	for move, count := range moveCounts {
		if count > bestCount {
			bestCount = count
			bestMove = move
		}
	}

	// Fallback to random if something went wrong
	if bestCount == 0 {
		return validMoves[rand.Intn(len(validMoves))]
	}

	return bestMove
}

// selectNode uses UCB1 to traverse tree until leaf
func (m *MCTSAgent) selectNode(node *MCTSNode) *MCTSNode {
	for len(node.untriedMoves) == 0 && len(node.children) > 0 {
		node = m.bestChild(node, m.explorationC)
	}

	// If we have untried moves, expand
	if len(node.untriedMoves) > 0 {
		return m.expand(node)
	}

	return node
}

// expand adds a new child node
func (m *MCTSAgent) expand(node *MCTSNode) *MCTSNode {
	// Pick random untried move
	moveIdx := rand.Intn(len(node.untriedMoves))
	move := node.untriedMoves[moveIdx]

	// Remove from untried
	node.untriedMoves = append(node.untriedMoves[:moveIdx], node.untriedMoves[moveIdx+1:]...)

	// Apply move to create child state
	childState := node.state.Clone()
	childState.PlayCard("", move)

	child := newMCTSNode(childState, node, &move, node.playerID)
	node.children = append(node.children, child)

	return child
}

// simulate plays out the game randomly from this node
func (m *MCTSAgent) simulate(node *MCTSNode) float64 {
	state := node.state.Clone()

	// Play out randomly until game ends
	maxMoves := 50 // Safety limit
	moves := 0
	for state.Phase == game.PhasePlaying && moves < maxMoves {
		validMoves := state.GetValidMoves()
		if len(validMoves) == 0 {
			break
		}
		move := validMoves[rand.Intn(len(validMoves))]
		state.PlayCard("", move)
		moves++
	}

	// Return reward from our player's perspective
	return m.evaluateTerminalState(state, node.playerID)
}

// evaluateTerminalState calculates reward for a finished game
func (m *MCTSAgent) evaluateTerminalState(state *game.GameState, playerID game.GamePosition) float64 {
	// Declarer needs 61+ points to win
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

// backpropagate updates statistics up the tree
func (m *MCTSAgent) backpropagate(node *MCTSNode, reward float64) {
	for node != nil {
		node.visits++
		node.totalReward += reward
		node = node.parent
	}
}

// bestChild selects best child using UCB1 formula
func (m *MCTSAgent) bestChild(node *MCTSNode, c float64) *MCTSNode {
	if len(node.children) == 0 {
		return nil
	}

	var best *MCTSNode
	bestValue := math.Inf(-1)

	for _, child := range node.children {
		var ucb1 float64

		if child.visits == 0 {
			ucb1 = math.Inf(1) // Prioritize unvisited nodes
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

// determinize creates a possible world consistent with what we know
func (m *MCTSAgent) determinize(state *game.GameState) *game.GameState {
	det := state.Clone()
	currentPlayerID := state.CurrentPlayer

	// Collect all cards we can see
	seenCards := make(map[game.Card]bool)

	// Our own hand
	for _, card := range state.Players[currentPlayerID].Hand {
		seenCards[card] = true
	}

	// Cards already played
	for _, trick := range state.CardsPlayed {
		for _, card := range trick {
			seenCards[card] = true
		}
	}

	// Current trick
	for _, card := range state.Trick {
		seenCards[card] = true
	}

	// Collect all unseen cards (full deck minus seen cards)
	allCards := game.NewDeck()
	unseenCards := make([]game.Card, 0)
	for _, card := range allCards {
		if !seenCards[card] {
			unseenCards = append(unseenCards, card)
		}
	}

	// Shuffle unseen cards
	rand.Shuffle(len(unseenCards), func(i, j int) {
		unseenCards[i], unseenCards[j] = unseenCards[j], unseenCards[i]
	})

	// Distribute to opponent hands and skat
	idx := 0
	for p := 0; p < 3; p++ {
		if p == int(currentPlayerID) {
			continue // Keep our own hand
		}

		// Calculate how many cards this opponent should have
		// Start with 10, subtract cards in tricks taken, subtract if they played in current trick
		expectedCards := len(state.Players[p].Hand)

		// Make sure we have a sane value
		if expectedCards < 0 {
			expectedCards = 0
		}
		if expectedCards > len(unseenCards)-idx {
			expectedCards = len(unseenCards) - idx
		}

		// Assign random unseen cards
		det.Players[p].Hand = make([]game.Card, 0, expectedCards)
		for i := 0; i < expectedCards && idx < len(unseenCards); i++ {
			det.Players[p].Hand = append(det.Players[p].Hand, unseenCards[idx])
			idx++
		}
	}

	// Remaining cards go to skat (if not seen)
	if idx < len(unseenCards) && idx+1 < len(unseenCards) {
		det.Skat[0] = unseenCards[idx]
		det.Skat[1] = unseenCards[idx+1]
	}

	return det
}

func (m *MCTSAgent) Bid(state *game.GameState, currentBid int) int {
	// Use simple heuristic for bidding
	// Count high-value cards and jacks
	player := state.Players[state.CurrentPlayer]
	strength := 0

	for _, card := range player.Hand {
		if card.Rank == game.Jack {
			strength += 3
		} else if card.Value() >= 10 {
			strength += 1
		}
	}

	// Bid based on hand strength
	if strength >= 8 && currentBid < 30 {
		return currentBid + 1
	}
	return 0
}

func (m *MCTSAgent) ChooseGame(state *game.GameState) (game.GameMode, game.Suit) {
	// Count suits and jacks
	player := state.Players[state.CurrentPlayer]
	suitCounts := make(map[game.Suit]int)
	jacks := 0

	for _, card := range player.Hand {
		if card.Rank == game.Jack {
			jacks++
		}
		suitCounts[card.Suit]++
	}

	// If we have 3+ jacks, play Grand
	if jacks >= 3 {
		return game.ModeGrand, game.Clubs // Trump suit doesn't matter for Grand
	}

	// Otherwise pick suit with most cards
	var bestSuit game.Suit
	maxCount := 0
	for suit, count := range suitCounts {
		if count > maxCount {
			maxCount = count
			bestSuit = suit
		}
	}

	return game.ModeSuit, bestSuit
}

func (m *MCTSAgent) Name() string {
	return m.name
}
