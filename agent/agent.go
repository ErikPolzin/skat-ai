package agent

import (
	"math"
	"math/rand"
	"skat/game"
	"sync"
)

// Agent represents a player that can make decisions
type Agent interface {
	// SelectMove chooses a card to play from valid moves
	SelectMove(state *game.GameState, validMoves []game.Card) game.Card

	// Bid decides whether to bid and at what value
	Bid(state *game.GameState, currentBid int) int

	// ChooseGame selects the game mode after winning the bid
	ChooseGame(state *game.GameState) (game.GameMode, game.Suit)

	// Name returns the agent's identifier
	Name() string
}

// SkatAgent combines MCTS for card playing with Q-learning for bidding
type SkatAgent struct {
	name string

	// MCTS parameters for card playing
	simulations    int
	explorationC   float64
	deterministicC float64

	// Q-learning parameters for bidding
	qTable  map[int]map[int]float64
	alpha   float64 // Learning rate
	gamma   float64 // Discount factor
	Epsilon float64 // Exploration rate (exported)

	// Track current episode (exported for trainer access)
	CurrentHandScore int
	CurrentBid       int
}

func NewSkatAgent(name string, simulations int) *SkatAgent {
	return &SkatAgent{
		name:           name,
		simulations:    simulations,
		explorationC:   1.41,
		deterministicC: 10,
		qTable:         make(map[int]map[int]float64),
		alpha:          0.1,
		gamma:          0.9,
		Epsilon:        0.15,
	}
}

func (sa *SkatAgent) Name() string {
	return sa.name
}

func (sa *SkatAgent) SetSimulations(sims int) {
	sa.simulations = sims
}

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

// Bid uses Q-learning to make bidding decisions
func (sa *SkatAgent) Bid(state *game.GameState, currentBid int) int {
	handScore := sa.evaluateHand(state.Players[state.CurrentPlayer].Hand)
	sa.CurrentHandScore = handScore

	if sa.qTable[handScore] == nil {
		sa.qTable[handScore] = make(map[int]float64)
	}

	possibleBids := []int{0}
	for bid := currentBid + 1; bid <= 60; bid++ {
		possibleBids = append(possibleBids, bid)
	}

	var selectedBid int
	if rand.Float64() < sa.Epsilon {
		selectedBid = possibleBids[rand.Intn(len(possibleBids))]
	} else {
		bestBid := 0
		bestQ := sa.getQ(handScore, 0)

		for _, bid := range possibleBids {
			q := sa.getQ(handScore, bid)
			if q > bestQ {
				bestQ = q
				bestBid = bid
			}
		}

		selectedBid = bestBid
	}

	sa.CurrentBid = selectedBid
	return selectedBid
}

// ChooseGame selects the best game mode based on hand composition
func (sa *SkatAgent) ChooseGame(state *game.GameState) (game.GameMode, game.Suit) {
	hand := state.Players[state.CurrentPlayer].Hand
	jacks := 0
	jackSuits := make(map[game.Suit]bool)
	suitCounts := make(map[game.Suit]int)
	suitPoints := make(map[game.Suit]int)

	for _, card := range hand {
		if card.Rank == game.Jack {
			jacks++
			jackSuits[card.Suit] = true
		}
		suitCounts[card.Suit]++
		suitPoints[card.Suit] += card.Value()
	}

	if jacks >= 3 {
		return game.ModeGrand, game.Clubs
	}

	if jackSuits[game.Clubs] && jackSuits[game.Spades] {
		return game.ModeGrand, game.Clubs
	}

	bestSuit := game.Clubs
	bestScore := 0

	for suit := game.Clubs; suit <= game.Diamonds; suit++ {
		score := suitCounts[suit]*10 + suitPoints[suit]
		if score > bestScore {
			bestScore = score
			bestSuit = suit
		}
	}

	if suitCounts[bestSuit] >= 5 {
		return game.ModeSuit, bestSuit
	}

	return game.ModeGrand, game.Clubs
}

// OnGameEnd updates Q-values based on game outcome
func (sa *SkatAgent) OnGameEnd(wonBid bool, becameDeclarer bool, wonGame bool, gameValue int, pointsScored int) {
	reward := 0.0

	if !becameDeclarer {
		if wonBid {
			reward = -0.05
		} else {
			if !wonGame && pointsScored < 40 {
				regretPenalty := -0.3 * (float64(sa.CurrentHandScore) / 100.0)
				reward = regretPenalty
			} else if !wonGame && pointsScored < 50 {
				regretPenalty := -0.15 * (float64(sa.CurrentHandScore) / 100.0)
				reward = regretPenalty
			} else {
				reward = 0.05
			}
		}
	} else {
		if wonGame {
			safetyMargin := float64(pointsScored-61) / 60.0
			reward = 1.0 + safetyMargin*0.5

			if sa.CurrentHandScore >= 70 {
				reward += 0.2
			}
		} else {
			if pointsScored >= 55 {
				reward = -0.2
				if sa.CurrentHandScore >= 70 {
					reward += 0.1
				}
			} else if pointsScored >= 45 {
				reward = -0.5
			} else {
				reward = -1.0
			}

			if sa.CurrentBid > 30 && pointsScored < 40 {
				reward -= 0.5
			}
		}
	}

	oldQ := sa.getQ(sa.CurrentHandScore, sa.CurrentBid)
	newQ := oldQ + sa.alpha*(reward-oldQ)
	sa.setQ(sa.CurrentHandScore, sa.CurrentBid, newQ)
}

// DecayEpsilon reduces exploration over time
func (sa *SkatAgent) DecayEpsilon(minEpsilon float64) {
	sa.Epsilon = math.Max(minEpsilon, sa.Epsilon*0.995)
}

// GetQTableSize returns the number of states learned
func (sa *SkatAgent) GetQTableSize() int {
	total := 0
	for _, bids := range sa.qTable {
		total += len(bids)
	}
	return total
}

// GetBestAction returns the best action and its Q-value for a given hand score
func (sa *SkatAgent) GetBestAction(handScore, currentBid int) (int, float64) {
	if sa.qTable[handScore] == nil {
		return 0, 0.0
	}

	bestBid := 0
	bestQ := sa.getQ(handScore, 0)

	for bid := currentBid + 1; bid <= 60; bid++ {
		q := sa.getQ(handScore, bid)
		if q > bestQ {
			bestQ = q
			bestBid = bid
		}
	}

	return bestBid, bestQ
}

func (sa *SkatAgent) SetQ(handScore, bid int, value float64) {
	sa.setQ(handScore, bid, value)
}

// evaluateHand returns a score representing hand strength
func (sa *SkatAgent) evaluateHand(hand []game.Card) int {
	score := 0
	jacks := 0
	jackSuits := make(map[game.Suit]bool)
	aces := 0
	tens := 0
	suitCounts := make(map[game.Suit]int)

	for _, card := range hand {
		if card.Rank == game.Jack {
			jacks++
			jackSuits[card.Suit] = true
		}
		if card.Rank == game.Ace {
			aces++
		}
		if card.Rank == game.Ten {
			tens++
		}
		suitCounts[card.Suit]++
		score += card.Value()
	}

	score += jacks * 15

	if jackSuits[game.Clubs] {
		score += 10
		if jackSuits[game.Spades] {
			score += 8
			if jackSuits[game.Hearts] {
				score += 6
			}
		}
	}

	for _, count := range suitCounts {
		if count >= 5 {
			score += (count - 4) * 3
		}
	}

	if score > 100 {
		score = 100
	}

	return score
}

func (sa *SkatAgent) getQ(handScore, bid int) float64 {
	if sa.qTable[handScore] == nil {
		return 0.0
	}
	return sa.qTable[handScore][bid]
}

func (sa *SkatAgent) setQ(handScore, bid int, value float64) {
	if sa.qTable[handScore] == nil {
		sa.qTable[handScore] = make(map[int]float64)
	}
	sa.qTable[handScore][bid] = value
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
	childState.PlayCard("", move)

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
		state.PlayCard("", move)
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

var (
	untrainedWarningShown = make(map[int]bool)
	untrainedWarningMutex sync.Mutex
)

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
