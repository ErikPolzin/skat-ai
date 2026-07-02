package strategies

import (
	"cmp"
	"math"
	"skat/game"
	"slices"
)

type heuristicContractWinProbabilityEstimator struct{}

// NewHeuristicContractWinProbabilityEstimator returns the legacy hand-written
// strength estimator. It is useful as a baseline or fallback probability model.
func NewHeuristicContractWinProbabilityEstimator() ContractWinProbabilityEstimator {
	return heuristicContractWinProbabilityEstimator{}
}

func (m heuristicContractWinProbabilityEstimator) EstimateWinProbability(hand game.Cards, mode game.GameMode, suit game.Suit, playedHand, announcedSchneider, announcedSchwarz bool) float64 {
	var probability float64
	switch mode {
	case game.ModeGrand:
		probability = m.evaluateGrandStrength(hand)
	case game.ModeSuit:
		probability = m.evaluateSuitStrength(hand, suit)
	case game.ModeNull:
		probability = m.evaluateNullStrength(hand)
	default:
		return 0
	}
	// The heuristic has no learned declaration-conditioned model. Keep it as a
	// conservative fallback; neural estimators learn these probabilities directly.
	if playedHand {
		probability *= 0.8
	}
	if announcedSchneider {
		probability = clampProbability((probability - 0.45) / 0.55)
	}
	if announcedSchwarz {
		probability = clampProbability((probability - 0.75) / 0.25)
	}
	return probability
}

func clampProbability(value float64) float64 { return min(1, max(0, value)) }

// HeuristicBiddingStrategy uses hand strength heuristics to make bidding decisions
type HeuristicBiddingStrategy struct {
	evaluator *ContractEvaluator
}

func NewHeuristicBiddingStrategy() *HeuristicBiddingStrategy {
	return &HeuristicBiddingStrategy{evaluator: NewContractEvaluator()}
}

func NewHeuristicBiddingStrategyWithConfig(config ContractEvaluatorConfig) *HeuristicBiddingStrategy {
	return &HeuristicBiddingStrategy{evaluator: NewContractEvaluatorWithConfig(config)}
}

func NewHeuristicBiddingStrategyWithEstimator(config ContractEvaluatorConfig, estimator ContractWinProbabilityEstimator) *HeuristicBiddingStrategy {
	return &HeuristicBiddingStrategy{evaluator: NewContractEvaluatorWithEstimator(config, estimator)}
}

func (h *HeuristicBiddingStrategy) GetName() string {
	return "HeuristicBidding"
}

func (h *HeuristicBiddingStrategy) ShouldBid(gs *game.GameState, hand game.Cards, currentBid int) bool {
	nextBid := gs.GetNextBidValue()
	if nextBid == 0 {
		return false
	}
	next, ok := h.evaluator.Best(hand, nextBid)
	if !ok {
		return false
	}

	// Do not raise beyond the value of the best current contract merely because
	// a weaker fallback also clears the generic acceptance threshold.
	current, currentOK := h.evaluator.Best(hand, currentBid)
	return !currentOK || next.ExpectedValue >= current.ExpectedValue
}

// HeuristicGameChoiceStrategy chooses game based on hand strength heuristics
type HeuristicGameChoiceStrategy struct {
	evaluator *ContractEvaluator
}

func NewHeuristicGameChoiceStrategy() *HeuristicGameChoiceStrategy {
	return &HeuristicGameChoiceStrategy{evaluator: NewContractEvaluator()}
}

func NewHeuristicGameChoiceStrategyWithConfig(config ContractEvaluatorConfig) *HeuristicGameChoiceStrategy {
	return &HeuristicGameChoiceStrategy{evaluator: NewContractEvaluatorWithConfig(config)}
}

func NewHeuristicGameChoiceStrategyWithEstimator(config ContractEvaluatorConfig, estimator ContractWinProbabilityEstimator) *HeuristicGameChoiceStrategy {
	return &HeuristicGameChoiceStrategy{evaluator: NewContractEvaluatorWithEstimator(config, estimator)}
}

func (h *HeuristicGameChoiceStrategy) GetName() string {
	return "HeuristicGameChoice"
}

func (h *HeuristicGameChoiceStrategy) ChooseGame(hand game.Cards, bidValue int) GameChoice {
	best, _ := h.evaluator.Best(hand, bidValue)
	return best.ToGameChoice()
}

func (h *HeuristicGameChoiceStrategy) ChooseGameAndSkatDiscard(hand game.Cards, bidValue int) GameChoice {
	if len(hand) != 12 {
		panic("ChooseGameAndSkatDiscard requires the 12-card post-skat hand")
	}
	best, _ := h.evaluator.BestWithDiscard(hand, bidValue, h.chooseSkatDiscard)
	return best.ToGameChoice()
}

// ChooseGameAndSkatDiscardForContract supports datasets that intentionally
// force a contract while still applying the heuristic discard for that exact
// contract. Normal play uses ChooseGameAndSkatDiscard instead.
func (h *HeuristicGameChoiceStrategy) ChooseGameAndSkatDiscardForContract(hand game.Cards, mode game.GameMode, suit game.Suit) GameChoice {
	first, second := h.chooseSkatDiscard(hand, mode, suit)
	return GameChoice{Mode: mode, TrumpSuit: suit, Discard: [2]game.Card{first, second}}
}

func (h *HeuristicGameChoiceStrategy) chooseSkatDiscard(hand game.Cards, mode game.GameMode, trumpSuit game.Suit) (game.Card, game.Card) {
	// Special handling for Null games
	if mode == game.ModeNull {
		return h.chooseNullSkatDiscard(hand)
	}

	// Refined heuristic based on research:
	// 1. Never discard trumps
	// 2. Never discard Aces unless necessary
	// 3. Prefer discarding unprotected high cards (10s without Ace)
	// 4. Prefer discarding from long suits to create voids

	var nonTrump []game.Card
	suitCounts := make(map[game.Suit]int)

	// Separate trumps from non-trumps and count suits
	for _, card := range hand {
		if !card.IsTrump(mode, trumpSuit) {
			nonTrump = append(nonTrump, card)
			suitCounts[card.Suit]++
		}
	}

	// Score each non-trump card for discarding (higher score = better to discard)
	type cardScore struct {
		card  game.Card
		score float64
	}

	var scoredCards []cardScore
	for _, card := range nonTrump {
		score := h.evaluateDiscardScore(card, nonTrump, suitCounts)
		scoredCards = append(scoredCards, cardScore{card, score})
	}

	// Sort by discard score (descending - highest scores first)
	slices.SortStableFunc(scoredCards, func(a, b cardScore) int {
		return cmp.Compare(b.score, a.score)
	})

	// Discard top two scored cards if we have enough non-trumps
	if len(scoredCards) >= 2 {
		return scoredCards[0].card, scoredCards[1].card
	}

	// Not enough non-trumps - fallback to lowest value cards
	game.Cards(hand).SortByValue()
	return hand[0], hand[1]
}

// evaluateGrandStrength scores a hand for playing Grand
// Returns normalized probability (0-1) of winning the Grand game
func (m heuristicContractWinProbabilityEstimator) evaluateGrandStrength(cards game.Cards) float64 {
	score := 0.0

	// Count jacks (trumps in Grand)
	jackCount := 0
	for _, card := range cards {
		if card.Rank == game.Jack {
			jackCount++
			// Higher value for higher jacks
			switch card.Suit {
			case game.Clubs:
				score += 15
			case game.Spades:
				score += 12
			case game.Hearts:
				score += 9
			case game.Diamonds:
				score += 6
			}
		}
	}

	// Grand requires good trump control
	// With fewer than 3 jacks, Grand is very difficult
	if jackCount < 3 {
		score = -150.0 // Massive penalty - Grand nearly impossible with <3 jacks
	} else {
		// 3 jacks is viable for Grand with good side suits
		// Base score starts low - Grand must prove itself
		score = 0.0

		// Bonus for jacks - very high value since they're critical for Grand
		score += float64(jackCount * 30) // Increased from 25

		// Count Aces and estimate tricks
		aceCount := 0
		tenCount := 0
		for _, card := range cards {
			if card.Rank == game.Ace {
				aceCount++
				score += 30 // Increased from 25
			}
			if card.Rank == game.Ten {
				tenCount++
			}
		}

		// Grand requires solid winners - 6+ jacks+aces
		totalWinners := jackCount + aceCount
		if totalWinners < 6 {
			score -= float64((6 - totalWinners) * 15) // Further reduced to allow more Grands
		} else if totalWinners == 7 {
			score += 60 // Excellent
		} else if totalWinners == 8 {
			score += 100 // Perfect Grand hand
		}

		// Bonus for having Ace-10 combinations (protected tens)
		aceTenPairs := 0
		for suit := game.Clubs; suit <= game.Diamonds; suit++ {
			hasAce, hasTen := false, false
			for _, card := range cards {
				if card.Suit == suit {
					if card.Rank == game.Ace {
						hasAce = true
					}
					if card.Rank == game.Ten {
						hasTen = true
					}
				}
			}
			if hasAce && hasTen {
				score += 20
				aceTenPairs++
			}
		}

		// Grand needs balanced distribution
		if aceTenPairs >= 3 {
			score += 30 // Excellent for Grand
		} else if aceTenPairs < 2 {
			score -= 20 // Risky - unprotected tens or missing aces
		}
	}

	// Normalize to 0-1 probability using sigmoid
	// Typical Grand scores range from -150 (impossible) to +200 (excellent)
	// Center sigmoid at score=50 (reasonable Grand), temperature=100 for calibration
	return sigmoid(score, 50.0, 100.0)
}

// evaluateSuitStrength scores a hand for playing a specific suit
// Returns normalized probability (0-1) of winning the suit game
func (m heuristicContractWinProbabilityEstimator) evaluateSuitStrength(cards game.Cards, trumpSuit game.Suit) float64 {
	score := 0.0

	// Count trumps (Jacks + trump suit)
	trumpCount := 0
	trumpPoints := 0
	hasTopTrumps := false

	for _, card := range cards {
		if card.IsTrump(game.ModeSuit, trumpSuit) {
			trumpCount++
			trumpPoints += card.Value()

			// Simplified trump bonus - don't double-count
			if card.Rank == game.Jack && (card.Suit == game.Clubs || card.Suit == game.Spades) {
				hasTopTrumps = true
			} else if card.Rank == game.Ace {
				hasTopTrumps = true
			}
		}
	}

	// Trump length is critical - need at least 5 for safety
	if trumpCount < 5 {
		score -= float64((5 - trumpCount) * 20) // Significant penalty for short trumps
	}

	// Primary trump scoring - count and quality
	score += float64(trumpCount*trumpCount) * 1.2 // Reduced to balance with Grand
	score += float64(trumpPoints) * 0.2           // Reduced further

	// Bonus for having top trump control
	if hasTopTrumps {
		score += 20 // Control bonus
	}

	// Evaluate side suits
	sideAces := 0
	for suit := game.Clubs; suit <= game.Diamonds; suit++ {
		if suit == trumpSuit {
			continue
		}

		suitCards := []game.Card{}
		for _, card := range cards {
			if card.Suit == suit && card.Rank != game.Jack {
				suitCards = append(suitCards, card)
			}
		}

		// Bonus for Ace-10 combinations in side suits
		hasAce, hasTen := false, false
		for _, card := range suitCards {
			if card.Rank == game.Ace {
				hasAce = true
				sideAces++
			}
			if card.Rank == game.Ten {
				hasTen = true
			}
		}
		if hasAce && hasTen {
			score += 15 // Very valuable
		} else if hasAce {
			score += 8
		}

		// Bonus for void suits (can trump in)
		if len(suitCards) == 0 {
			score += 18
		}
		// Good bonus for short suits (easier to trump)
		if len(suitCards) == 1 {
			score += 10
		}
		// Small bonus for doubleton
		if len(suitCards) == 2 {
			score += 4
		}
	}

	// Bonus for side suit aces (tricks outside trumps)
	score += float64(sideAces * 5)

	// Normalize to 0-1 probability using sigmoid
	// Typical suit scores range from -100 (weak) to +200 (excellent)
	// Center sigmoid at score=60 (reasonable suit game), temperature=100 for calibration
	return sigmoid(score, 60.0, 100.0)
}

// chooseNullSkatDiscard selects two cards to discard for Null games
// In Null, we want to keep low cards and discard high cards
func (h *HeuristicGameChoiceStrategy) chooseNullSkatDiscard(hand []game.Card) (game.Card, game.Card) {
	// Score each card for discarding in Null (higher score = better to discard)
	type cardScore struct {
		card  game.Card
		score float64
	}

	var scoredCards []cardScore
	var suitCounts [5]int
	for _, card := range hand {
		suitCounts[card.Suit]++
		score := 0.0
		switch card.Rank {
		case game.Ace:
			score += 100.0
		case game.King:
			score += 80.0
		case game.Queen:
			score += 70.0
		case game.Jack:
			score += 60.0
		case game.Ten:
			score += 50.0
		case game.Nine:
			score -= 30.0
		case game.Eight:
			score -= 40.0
		case game.Seven:
			score -= 50.0
		}
		scoredCards = append(scoredCards, cardScore{card, score})
	}

	if len(scoredCards) >= 2 {
		bestI, bestJ, bestScore := 0, 1, math.Inf(-1)
		for i := 0; i < len(scoredCards)-1; i++ {
			for j := i + 1; j < len(scoredCards); j++ {
				score := scoredCards[i].score + scoredCards[j].score
				if scoredCards[i].card.Suit == scoredCards[j].card.Suit {
					switch suitCounts[scoredCards[i].card.Suit] - 2 {
					case 0:
						score += 60 // Create a void for unloading later winners.
					case 1:
						score += 30 // Leave a disposable singleton.
					default:
						score += 10 // At least shorten one dangerous suit.
					}
				}
				if score > bestScore {
					bestI, bestJ, bestScore = i, j, score
				}
			}
		}
		return scoredCards[bestI].card, scoredCards[bestJ].card
	}

	// Fallback (shouldn't happen)
	game.Cards(hand).SortByValue()
	return hand[len(hand)-1], hand[len(hand)-2]
}

// evaluateDiscardScore scores how good a card is to discard (higher = better to discard)
func (h *HeuristicGameChoiceStrategy) evaluateDiscardScore(card game.Card, nonTrumpCards []game.Card, suitCounts map[game.Suit]int) float64 {
	score := 0.0

	// Never want to discard Aces (negative score)
	if card.Rank == game.Ace {
		return -100.0
	}

	// Check if we have the Ace of this suit
	hasAce := false
	for _, c := range nonTrumpCards {
		if c.Suit == card.Suit && c.Rank == game.Ace {
			hasAce = true
			break
		}
	}

	// Unprotected 10s are good to discard
	if card.Rank == game.Ten && !hasAce {
		score += 30
	}

	// Protected 10s (have Ace) are bad to discard
	if card.Rank == game.Ten && hasAce {
		score -= 20
	}

	// Prefer discarding from SHORTER suits to create voids faster
	// Invert the logic: fewer cards in suit = higher discard priority
	if suitCounts[card.Suit] <= 2 {
		score += 20 // High priority to create voids
	} else if suitCounts[card.Suit] == 3 {
		score += 10
	} else {
		// Longer suits are kept for flexibility
		score -= float64((suitCounts[card.Suit] - 3) * 5)
	}

	// Low value cards are generally good to discard
	if card.Value() == 0 {
		score += 15
	}

	// Kings and Queens in the middle
	if card.Rank == game.King {
		score += 8
	}
	if card.Rank == game.Queen {
		score += 10
	}

	return score
}

// HeuristicCardPlayStrategy uses rule-based heuristics for card play
type HeuristicCardPlayStrategy struct {
	// Card tracking for inference
	cardsPlayed map[game.Card]bool
}

func NewHeuristicCardPlayStrategy() *HeuristicCardPlayStrategy {
	return &HeuristicCardPlayStrategy{
		cardsPlayed: make(map[game.Card]bool),
	}
}

func (h *HeuristicCardPlayStrategy) GetName() string {
	return "HeuristicCardPlay"
}

func (h *HeuristicCardPlayStrategy) Clone() *HeuristicCardPlayStrategy {
	clone := NewHeuristicCardPlayStrategy()
	for card, played := range h.cardsPlayed {
		clone.cardsPlayed[card] = played
	}
	return clone
}

// OnTrickComplete tracks cards that have been played
func (h *HeuristicCardPlayStrategy) OnTrickComplete(trick []game.Card) {
	if h.cardsPlayed == nil {
		h.cardsPlayed = make(map[game.Card]bool)
	}
	for _, card := range trick {
		h.cardsPlayed[card] = true
	}
}

// Reset clears tracking (call at start of new game)
func (h *HeuristicCardPlayStrategy) Reset() {
	h.cardsPlayed = make(map[game.Card]bool)
}

// countRemainingTrumps counts how many trumps haven't been played yet
func (h *HeuristicCardPlayStrategy) countRemainingTrumps(gs *game.GameState, myHand []game.Card) int {
	if h.cardsPlayed == nil {
		h.cardsPlayed = make(map[game.Card]bool)
	}

	myTrumps := make(map[game.Card]bool)
	for _, card := range myHand {
		if gs.TrumpValue(card) > 0 {
			myTrumps[card] = true
		}
	}

	remaining := 0
	// Check all possible trumps
	if gs.Mode == game.ModeGrand {
		// Only jacks are trump
		for suit := game.Clubs; suit <= game.Diamonds; suit++ {
			card := game.Card{Suit: suit, Rank: game.Jack}
			if !h.cardsPlayed[card] && !myTrumps[card] {
				remaining++
			}
		}
	} else if gs.Mode == game.ModeSuit {
		// Jacks + trump suit
		for suit := game.Clubs; suit <= game.Diamonds; suit++ {
			card := game.Card{Suit: suit, Rank: game.Jack}
			if !h.cardsPlayed[card] && !myTrumps[card] {
				remaining++
			}
		}
		// Trump suit cards (excluding jacks)
		for rank := game.Seven; rank <= game.Ace; rank++ {
			if rank == game.Jack {
				continue
			}
			card := game.Card{Suit: gs.TrumpSuit, Rank: rank}
			if !h.cardsPlayed[card] && !myTrumps[card] {
				remaining++
			}
		}
	}

	return remaining
}

func (h *HeuristicCardPlayStrategy) SelectMove(gs *game.GameState, validMoves []game.Card) game.Card {
	if len(validMoves) == 1 {
		return validMoves[0]
	}

	// Sort moves by value (low to high)
	game.Cards(validMoves).SortByValue()

	currentPlayer := gs.CurrentPlayer
	isDefender := gs.Declarer == nil || currentPlayer != *gs.Declarer

	// Handle Null games (declarer tries to lose all tricks)
	if gs.Mode == game.ModeNull {
		if isDefender {
			return h.selectNullDefenderMove(gs, validMoves)
		}
		return h.selectNullDeclarerMove(gs, validMoves)
	}

	if isDefender {
		return h.selectDefenderMove(gs, validMoves)
	}
	return h.selectDeclarerMove(gs, validMoves)
}

// selectNullDeclarerMove handles card play for declarer in Null games
// Goal: Lose every trick by playing cards that won't win
func (h *HeuristicCardPlayStrategy) selectNullDeclarerMove(gs *game.GameState, validMoves []game.Card) game.Card {
	trick := gs.Trick

	// Sort moves by Null card strength (in Null: A > K > Q > J > 10 > 9 > 8 > 7)
	game.Cards(validMoves).SortByNullRank()

	// Leading the trick
	if len(trick) == 0 {
		return validMoves[0]
	}

	// Following in trick - play card that loses but as high as possible without winning
	leadSuit := trick[0].Suit

	// Find highest card in current trick
	highestCard := trick[0]
	for _, card := range trick[1:] {
		if card.BeatsInNull(highestCard, leadSuit) {
			highestCard = card
		}
	}

	// Try to play highest card that still loses
	for i := len(validMoves) - 1; i >= 0; i-- {
		if !validMoves[i].BeatsInNull(highestCard, leadSuit) {
			return validMoves[i]
		}
	}

	// Must win - play lowest card to minimize damage
	return validMoves[0]
}

// selectNullDefenderMove handles card play for defenders in Null games
// Goal: Force declarer to win tricks
func (h *HeuristicCardPlayStrategy) selectNullDefenderMove(gs *game.GameState, validMoves []game.Card) game.Card {
	trick := gs.Trick

	// Sort moves by Null card strength
	game.Cards(validMoves).SortByNullRank()

	// Leading the trick
	if len(trick) == 0 {
		return nullDefenderPressureLead(validMoves)
	}

	// Following in trick
	leadSuit := trick[0].Suit

	// Check if declarer has played yet
	declarerPlayed := false
	declarerCard := game.Card{}

	for i, card := range trick {
		pos := (gs.TrickStarter + game.GamePosition(i)) % 3
		if gs.Declarer != nil && pos == *gs.Declarer {
			declarerPlayed = true
			declarerCard = card
			break
		}
	}

	if declarerPlayed {
		// If the declarer is already losing, safely unload our strongest card.
		// If they are winning, play the highest card that stays underneath them
		// instead of overtaking and rescuing them.
		declarerWinning := true
		for _, card := range trick {
			if card != declarerCard && card.BeatsInNull(declarerCard, leadSuit) {
				declarerWinning = false
				break
			}
		}

		if !declarerWinning {
			return validMoves[len(validMoves)-1]
		}
		for i := len(validMoves) - 1; i >= 0; i-- {
			if !validMoves[i].BeatsInNull(declarerCard, leadSuit) {
				return validMoves[i]
			}
		}
		return validMoves[0]
	}

	// Before the declarer plays, a low card applies pressure; a high card merely
	// gives them more room to duck under the trick.
	return validMoves[0]
}

// nullDefenderPressureLead leads low from the defender's longest suit. It uses
// only the defender's own cards, preserving imperfect-information play.
func nullDefenderPressureLead(moves []game.Card) game.Card {
	var suitCounts [5]int
	for _, move := range moves {
		suitCounts[move.Suit]++
	}
	best := moves[0]
	bestScore := suitCounts[best.Suit]*10 - best.NullRank()
	for _, move := range moves {
		score := suitCounts[move.Suit]*10 - move.NullRank()
		if score > bestScore {
			best, bestScore = move, score
		}
	}
	return best
}

func (h *HeuristicCardPlayStrategy) selectDeclarerMove(gs *game.GameState, validMoves []game.Card) game.Card {
	trick := gs.Trick

	// Leading the trick
	if len(trick) == 0 {
		// Strategy: Cash Aces before drawing trumps
		// This prevents defenders from trumping our high-value winners

		// First, check for unprotected Aces in side suits
		for _, move := range validMoves {
			if move.Rank == game.Ace && gs.TrumpValue(move) == 0 {
				// Lead the Ace to cash it
				return move
			}
		}

		// Next, check for protected Ace-10 combinations (Ace with 10)
		for suit := game.Clubs; suit <= game.Diamonds; suit++ {
			if gs.TrumpValue(game.Card{Suit: suit, Rank: game.Ace}) > 0 {
				continue // Skip trump suit
			}

			hasAce, hasTen := false, false
			var aceCard game.Card

			for _, move := range validMoves {
				if move.Suit == suit && move.Rank == game.Ace {
					hasAce = true
					aceCard = move
				}
				if move.Suit == suit && move.Rank == game.Ten {
					hasTen = true
				}
			}

			// If we have Ace-10, lead Ace first
			if hasAce && hasTen {
				return aceCard
			}
		}

		// Now consider drawing trumps if we have strong trump control
		trumpCount := 0
		var highestTrump game.Card
		hasTrump := false

		for _, move := range validMoves {
			if gs.TrumpValue(move) > 0 {
				trumpCount++
				if !hasTrump || gs.CardBeats(move, highestTrump) {
					highestTrump = move
					hasTrump = true
				}
			}
		}

		// Use card tracking to decide if we should draw trumps
		remainingOpponentTrumps := h.countRemainingTrumps(gs, validMoves)

		// Draw trumps if:
		// 1. We have good trump control (3+ trumps), AND
		// 2. Opponents still have trumps that could ruff our winners
		if trumpCount >= 3 && hasTrump && remainingOpponentTrumps > 0 {
			return highestTrump
		}

		// If opponents are out of trumps (or very few), focus on cashing winners
		// Lead from SHORT suits to set up ruffs or cash winners before they're gone
		if remainingOpponentTrumps <= 1 {
			// Opponents have few/no trumps - cash our winners from short suits
			suitLengths := make(map[game.Suit]int)
			for _, move := range validMoves {
				if gs.TrumpValue(move) == 0 {
					suitLengths[move.Suit]++
				}
			}

			// Find shortest suit with high cards (to cash winners)
			shortestSuit := game.NoSuit
			minLength := 10
			for suit, length := range suitLengths {
				if length < minLength && length > 0 {
					minLength = length
					shortestSuit = suit
				}
			}

			// Lead high card from shortest suit
			if shortestSuit != game.NoSuit {
				for i := len(validMoves) - 1; i >= 0; i-- {
					if validMoves[i].Suit == shortestSuit && gs.TrumpValue(validMoves[i]) == 0 {
						return validMoves[i]
					}
				}
			}
		}

		// Default declarer strategy: lead from SHORT side suits
		// This allows declarer to ruff later rounds of that suit
		suitLengths := make(map[game.Suit]int)
		for _, move := range validMoves {
			if gs.TrumpValue(move) == 0 {
				suitLengths[move.Suit]++
			}
		}

		shortestSuit := game.NoSuit
		minLength := 10
		for suit, length := range suitLengths {
			if length < minLength && length > 0 {
				minLength = length
				shortestSuit = suit
			}
		}

		// Lead from shortest suit (high card first to cash winners)
		if shortestSuit != game.NoSuit {
			for i := len(validMoves) - 1; i >= 0; i-- {
				if validMoves[i].Suit == shortestSuit {
					return validMoves[i]
				}
			}
		}

		// Fallback: lead highest non-trump card
		for i := len(validMoves) - 1; i >= 0; i-- {
			if gs.TrumpValue(validMoves[i]) == 0 {
				return validMoves[i]
			}
		}

		// All cards are trump - lead highest
		return validMoves[len(validMoves)-1]
	}

	// Following in trick - try to win with lowest winning card
	for _, move := range validMoves {
		if wouldWinTrick(gs, move, trick) {
			return move
		}
	}

	// Can't win - play lowest card
	return validMoves[0]
}

func (h *HeuristicCardPlayStrategy) selectDefenderMove(gs *game.GameState, validMoves []game.Card) game.Card {
	trick := gs.Trick

	// Leading the trick
	if len(trick) == 0 {
		// Defender strategy: Attack declarer's weak suits, NOT trumps
		// Leading trumps as a defender is usually wrong - it helps declarer draw trumps

		// Count our holdings by suit
		suitCounts := make(map[game.Suit]int)
		trumpCount := 0
		hasAce := make(map[game.Suit]bool)

		for _, move := range validMoves {
			if gs.TrumpValue(move) > 0 {
				trumpCount++
			} else {
				suitCounts[move.Suit]++
				if move.Rank == game.Ace {
					hasAce[move.Suit] = true
				}
			}
		}

		// Strategy 1: Lead Ace from short suit (2 cards or less)
		// This cashes the Ace before declarer can trump it
		for suit := game.Clubs; suit <= game.Diamonds; suit++ {
			if hasAce[suit] && suitCounts[suit] <= 2 {
				// Find and lead the Ace
				for _, move := range validMoves {
					if move.Suit == suit && move.Rank == game.Ace {
						return move
					}
				}
			}
		}

		// Strategy 2: Lead from longest non-trump suit
		// Forces declarer to use trumps or lose control
		longestSuit := game.NoSuit
		maxLength := 0
		for suit, length := range suitCounts {
			if length > maxLength {
				maxLength = length
				longestSuit = suit
			}
		}

		if longestSuit != game.NoSuit && maxLength >= 3 {
			// Lead high card from long suit to force declarer
			for i := len(validMoves) - 1; i >= 0; i-- {
				if validMoves[i].Suit == longestSuit && gs.TrumpValue(validMoves[i]) == 0 {
					return validMoves[i]
				}
			}
		}

		// Strategy 3: Lead any Ace we have (cash winners)
		for _, move := range validMoves {
			if move.Rank == game.Ace && gs.TrumpValue(move) == 0 {
				return move
			}
		}

		// Strategy 4: Lead low card from side suit to find partner's strength
		for _, move := range validMoves {
			if gs.TrumpValue(move) == 0 && move.Value() == 0 {
				return move
			}
		}

		// Strategy 5: Only lead trump if we have nothing else or very strong trumps
		if trumpCount >= 4 {
			// We have strong trump control - lead trump to attack declarer
			for i := len(validMoves) - 1; i >= 0; i-- {
				if gs.TrumpValue(validMoves[i]) > 0 {
					return validMoves[i]
				}
			}
		}

		// Fallback: lead lowest card
		return validMoves[0]
	}

	// Following in trick
	// Check if partner is winning (in 3rd position)
	if len(trick) == 2 {
		winner := trick.TrickWinner(gs.TrickStarter, gs.Mode, gs.TrumpSuit)
		partner := getDefenderPartner(gs)
		if winner == partner {
			// Partner winning - play lowest card (don't waste high cards)
			return validMoves[0]
		}
	}

	// Try to beat the trick with LOWEST winning card (efficient)
	for _, move := range validMoves {
		if wouldWinTrick(gs, move, trick) {
			return move // validMoves is sorted low to high, so first winner is lowest
		}
	}

	// Can't win - discard highest useless card if partner might win
	// or lowest card if declarer is winning
	if len(trick) == 1 {
		// Second to play - discard low to signal weakness
		return validMoves[0]
	}

	// Third to play and can't win - discard high cards we don't need
	for i := len(validMoves) - 1; i >= 0; i-- {
		if validMoves[i].Value() == 0 {
			// Prefer discarding worthless cards from high to low
			return validMoves[i]
		}
	}

	// All cards have value - discard lowest
	return validMoves[0]
}

// heuristicOrder orders moves by the sequence the heuristic agent would play them
// Optimized for minimax: avoids allocations and uses in-place sorting
func heuristicOrder(gs *game.GameState, moves []game.Card, isDeclarer bool) {
	if len(moves) <= 1 {
		return
	}
	if gs.Mode == game.ModeNull {
		scoreNullMoves(gs, moves, isDeclarer)
		return
	}

	trick := gs.Trick
	isTrumpCache := make([]bool, len(moves))

	// Pre-compute trump status for all moves (avoid repeated checks)
	for i, move := range moves {
		isTrumpCache[i] = move.IsTrump(gs.Mode, gs.TrumpSuit)
	}

	// Compute scores for sorting
	scores := make([]float64, len(moves))

	if isDeclarer {
		scoreDeclarerMoves(gs, moves, trick, isTrumpCache, scores)
	} else {
		scoreDefenderMoves(gs, moves, trick, isTrumpCache, scores)
	}

	type scoredMove struct {
		card  game.Card
		score float64
	}
	scoredMoves := make([]scoredMove, len(moves))
	for i := range moves {
		scoredMoves[i] = scoredMove{card: moves[i], score: scores[i]}
	}
	slices.SortStableFunc(scoredMoves, func(a, b scoredMove) int {
		return cmp.Compare(b.score, a.score)
	})
	for i := range moves {
		moves[i] = scoredMoves[i].card
	}
}

// scoreNullMoves orders moves according to the inverted objective of a Null
// game. The generic point-game ordering prioritizes taking tricks for both
// sides, which is exactly backwards for the declarer and often backwards for
// defenders trying to leave the declarer on lead.
func scoreNullMoves(gs *game.GameState, moves []game.Card, isDeclarer bool) {
	type scoredMove struct {
		card  game.Card
		score int
	}
	scored := make([]scoredMove, len(moves))
	for i, move := range moves {
		next := gs.Clone()
		_, _ = next.PlayCard(move)
		winner := next.Trick.TrickWinner(next.TrickStarter, next.Mode, next.TrumpSuit)
		declarerWinning := next.Declarer != nil && winner == *next.Declarer

		score := -move.NullRank()
		if isDeclarer {
			if declarerWinning {
				score -= 1000
			} else {
				// Safely unload the highest card that still loses the trick.
				score = 1000 + move.NullRank()
			}
		} else if declarerWinning {
			// Search forcing continuations before lines that rescue the declarer.
			score += 1000
		}
		scored[i] = scoredMove{card: move, score: score}
	}
	slices.SortStableFunc(scored, func(a, b scoredMove) int {
		return cmp.Compare(b.score, a.score)
	})
	for i := range moves {
		moves[i] = scored[i].card
	}
}

func scoreDeclarerMoves(gs *game.GameState, moves []game.Card, trick game.Cards, isTrumpCache []bool, scores []float64) {
	if len(trick) == 0 {
		// Pre-compute suit counts and trump count once
		var suitCounts [5]int // NoSuit, Clubs, Spades, Hearts, Diamonds
		trumpCount := 0

		for i, m := range moves {
			if isTrumpCache[i] {
				trumpCount++
			} else {
				suitCounts[m.Suit]++
			}
		}

		// Score each move
		for i, move := range moves {
			score := 0.0

			// 1. Aces in side suits (highest priority)
			if move.Rank == game.Ace && !isTrumpCache[i] {
				score += 100.0

				// 2. Check for Ace-10 combination
				for _, m := range moves {
					if m.Suit == move.Suit && m.Rank == game.Ten {
						score += 90.0
						break
					}
				}
			}

			// 3. Drawing trumps with strong control
			if isTrumpCache[i] && trumpCount >= 3 {
				score += 50.0 + float64(move.Value())
			}

			// 4. Cards from short suits
			if !isTrumpCache[i] && suitCounts[move.Suit] <= 2 {
				score += 40.0 + float64(move.Value())*0.5
			}

			// 5. High cards are generally preferred
			score += float64(move.Value()) * 0.3

			scores[i] = score
		}
	} else {
		// Following - try to win with lowest winning card
		for i, move := range moves {
			if wouldWinTrick(gs, move, trick) {
				scores[i] = 100.0 - float64(move.Value())
			} else {
				scores[i] = 10.0 - float64(move.Value())*0.5
			}
		}
	}
}

func scoreDefenderMoves(gs *game.GameState, moves []game.Card, trick game.Cards, isTrumpCache []bool, scores []float64) {
	if len(trick) == 0 {
		// Pre-compute suit counts and trump count once
		var suitCounts [5]int // NoSuit, Clubs, Spades, Hearts, Diamonds
		trumpCount := 0
		maxSuitLength := 0

		for i, m := range moves {
			if isTrumpCache[i] {
				trumpCount++
			} else {
				suitCounts[m.Suit]++
				if suitCounts[m.Suit] > maxSuitLength {
					maxSuitLength = suitCounts[m.Suit]
				}
			}
		}

		// Score each move
		for i, move := range moves {
			score := 0.0

			// 1. Ace from short suit (highest priority)
			if move.Rank == game.Ace && !isTrumpCache[i] && suitCounts[move.Suit] <= 2 {
				score += 100.0
			}

			// 2. Cards from longest suit
			if !isTrumpCache[i] && suitCounts[move.Suit] == maxSuitLength && maxSuitLength >= 3 {
				score += 80.0 + float64(move.Value())*0.5
			}

			// 3. Any Ace (cash winners)
			if move.Rank == game.Ace && !isTrumpCache[i] {
				score += 70.0
			}

			// 4. Low cards from side suits (find partner's strength)
			if !isTrumpCache[i] && move.Value() == 0 {
				score += 50.0
			}

			// 5. Trump only if very strong
			if isTrumpCache[i] && trumpCount >= 4 {
				score += 40.0 + float64(move.Value())*0.3
			}

			scores[i] = score
		}
	} else {
		// Pre-check if partner is winning (in 3rd position)
		partnerWinning := false
		if len(trick) == 2 {
			winner := trick.TrickWinner(gs.TrickStarter, gs.Mode, gs.TrumpSuit)
			partner := getDefenderPartner(gs)
			partnerWinning = (winner == partner)
		}

		// Score each move
		for i, move := range moves {
			if partnerWinning {
				// Partner winning - play lowest card
				scores[i] = 100.0 - float64(move.Value())*2
			} else if wouldWinTrick(gs, move, trick) {
				// Try to beat with lowest winning card
				scores[i] = 100.0 - float64(move.Value())
			} else {
				// Can't win - discard appropriately
				if move.Value() == 0 {
					scores[i] = 50.0
				} else {
					scores[i] = 10.0 - float64(move.Value())*0.5
				}
			}
		}
	}
}

func wouldWinTrick(gs *game.GameState, card game.Card, trick game.Cards) bool {
	for _, trickCard := range trick {
		if !gs.CardBeats(card, trickCard) {
			return false
		}
	}
	return true
}

func getDefenderPartner(gs *game.GameState) game.GamePosition {
	currentPlayer := gs.CurrentPlayer
	for pos := game.Dealer; pos <= game.Speaker; pos++ {
		if pos != currentPlayer && (gs.Declarer == nil || pos != *gs.Declarer) {
			return pos
		}
	}
	return game.Dealer
}

// evaluateNullStrength scores a hand for playing Null
// Returns normalized probability (0-1) of winning the Null game
// In Null, declarer must lose every trick, so low cards and weak holdings are best
func (m heuristicContractWinProbabilityEstimator) evaluateNullStrength(cards game.Cards) float64 {
	score := 0.0

	// In Null games, there are no trumps - suits follow standard order (A, K, Q, J, 10, 9, 8, 7)
	// Declarer needs to LOSE all tricks, so we want:
	// - Low cards (7, 8, 9) are excellent
	// - No high cards (A, K, Q) that might win
	// - Balanced distribution to avoid being forced to win

	// Count cards by rank
	sevenCount := 0
	eightCount := 0
	nineCount := 0
	highCards := 0 // Aces, Kings, Queens
	jacks := 0
	tens := 0

	for _, card := range cards {
		switch card.Rank {
		case game.Seven:
			sevenCount++
		case game.Eight:
			eightCount++
		case game.Nine:
			nineCount++
		case game.Ten:
			tens++
		case game.Jack:
			jacks++
		case game.Queen, game.King, game.Ace:
			highCards++
		}
	}

	// Strong bonus for low cards (want many 7s, 8s, 9s)
	score += float64(sevenCount) * 55.0 // Sevens are best
	score += float64(eightCount) * 45.0
	score += float64(nineCount) * 35.0

	// Heavy penalty for high cards (very risky in Null)
	score -= float64(highCards) * 55.0 // Strong penalty for face cards

	// Moderate penalty for Jacks and Tens
	score -= float64(jacks) * 35.0
	score -= float64(tens) * 40.0

	// Check suit distribution - want balanced or short suits
	suitCounts := make(map[game.Suit]int)
	for _, card := range cards {
		suitCounts[card.Suit]++
	}

	// Penalty for long suits (hard to avoid winning)
	for _, count := range suitCounts {
		if count >= 4 {
			score -= 20.0
		} else if count == 3 {
			score -= 8.0
		}
	}

	// Bonus for having escape cards (multiple low cards in each suit)
	for suit := game.Clubs; suit <= game.Diamonds; suit++ {
		lowCardsInSuit := 0
		for _, card := range cards {
			if card.Suit == suit && (card.Rank == game.Seven || card.Rank == game.Eight || card.Rank == game.Nine) {
				lowCardsInSuit++
			}
		}
		if lowCardsInSuit >= 2 {
			score += 22.0 // Good escape options
		}
	}

	baseProbability := sigmoid(score, 210.0, 115.0)
	return evaluateNullSuitSafety(cards, baseProbability)
}

// evaluateNullSuitSafety accounts for the shape within each suit. Rank counts
// alone miss the central Null distinction between a low, escapable holding and
// a high card stranded above gaps. The coefficients were fitted on 16,000 of
// 20,000 simulated heuristic Null games; on the 4,000 held-out games,
// suit-shape features improved AUC from 0.756 to 0.887.
func evaluateNullSuitSafety(cards game.Cards, baseProbability float64) float64 {
	var bySuit [5][]int
	for _, card := range cards {
		bySuit[card.Suit] = append(bySuit[card.Suit], card.NullRank()-1)
	}

	totalGaps := 0
	longestSuit := 0
	voids := 0
	singletons := 0
	singletonRank := 0
	highestLowestCard := 0
	for suit := game.Clubs; suit <= game.Diamonds; suit++ {
		ranks := bySuit[suit]
		if len(ranks) == 0 {
			voids++
			continue
		}
		slices.Sort(ranks)
		if len(ranks) > longestSuit {
			longestSuit = len(ranks)
		}
		if ranks[0] > highestLowestCard {
			highestLowestCard = ranks[0]
		}
		if len(ranks) == 1 {
			singletons++
			singletonRank += ranks[0]
		}
		for position, rank := range ranks {
			totalGaps += rank - position
		}
	}

	baseLogit := math.Log(baseProbability / (1 - baseProbability))
	// The intercept is calibrated on naturally selected games using the atomic
	// contract-and-discard flow; the remaining coefficients come from the
	// held-out suit-shape fit described above.
	logit := 2.900405 +
		0.29092*baseLogit -
		0.12722*float64(totalGaps) -
		0.22012*float64(longestSuit) +
		1.46256*float64(voids) +
		0.75291*float64(singletons) -
		0.13444*float64(singletonRank) -
		0.82697*float64(highestLowestCard)
	return 1 / (1 + math.Exp(-logit))
}

// sigmoid converts a raw score to a probability using a sigmoid function
// center: the score value that maps to 0.5 probability
// temperature: controls the steepness (higher = more gradual)
func sigmoid(score, center, temperature float64) float64 {
	return 1.0 / (1.0 + math.Exp(-(score-center)/temperature))
}
