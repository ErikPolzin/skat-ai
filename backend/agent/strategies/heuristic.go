package strategies

import (
	"math"
	"skat/game"
)

// HeuristicBiddingStrategy uses hand strength heuristics to make bidding decisions
type HeuristicBiddingStrategy struct {
	evaluator *ContractEvaluator
}

func NewHeuristicBiddingStrategy() *HeuristicBiddingStrategy {
	gameChoiceStrat := NewHeuristicGameChoiceStrategy()
	return &HeuristicBiddingStrategy{evaluator: NewContractEvaluator(gameChoiceStrat)}
}

func NewHeuristicBiddingStrategyWithConfig(config ContractEvaluatorConfig) *HeuristicBiddingStrategy {
	gameChoiceStrat := NewHeuristicGameChoiceStrategy()
	return &HeuristicBiddingStrategy{evaluator: NewContractEvaluatorWithConfig(gameChoiceStrat, config)}
}

func (h *HeuristicBiddingStrategy) GetName() string {
	return "HeuristicBidding"
}

func (h *HeuristicBiddingStrategy) ShouldBid(gs *game.GameState, hand []game.Card, currentBid int) bool {
	nextBid := gs.GetNextBidValue()
	if nextBid == 0 {
		return false
	}
	_, ok := h.evaluator.Best(hand, nextBid)
	return ok
}

// HeuristicGameChoiceStrategy chooses game based on hand strength heuristics
type HeuristicGameChoiceStrategy struct {
	evaluator *ContractEvaluator
}

func NewHeuristicGameChoiceStrategy() *HeuristicGameChoiceStrategy {
	strat := &HeuristicGameChoiceStrategy{}
	strat.evaluator = NewContractEvaluator(strat)
	return strat
}

func NewHeuristicGameChoiceStrategyWithConfig(config ContractEvaluatorConfig) *HeuristicGameChoiceStrategy {
	strat := &HeuristicGameChoiceStrategy{}
	strat.evaluator = NewContractEvaluatorWithConfig(strat, config)
	return strat
}

func (h *HeuristicGameChoiceStrategy) GetName() string {
	return "HeuristicGameChoice"
}

func (h *HeuristicGameChoiceStrategy) ChooseGame(hand []game.Card, bidValue int) (game.GameMode, game.Suit) {
	best, _ := h.evaluator.Best(hand, bidValue)
	return best.Mode, best.TrumpSuit
}

func (h *HeuristicGameChoiceStrategy) ChooseSkatDiscard(hand []game.Card, mode game.GameMode, trumpSuit game.Suit) (game.Card, game.Card) {
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
		isTrump := card.Rank == game.Jack ||
			(mode == game.ModeSuit && card.Suit == trumpSuit)

		if !isTrump {
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
	for i := 0; i < len(scoredCards); i++ {
		for j := i + 1; j < len(scoredCards); j++ {
			if scoredCards[i].score < scoredCards[j].score {
				scoredCards[i], scoredCards[j] = scoredCards[j], scoredCards[i]
			}
		}
	}

	// Discard top two scored cards if we have enough non-trumps
	if len(scoredCards) >= 2 {
		return scoredCards[0].card, scoredCards[1].card
	}

	// Not enough non-trumps - fallback to lowest value cards
	sortByValue(hand)
	return hand[0], hand[1]
}

// evaluateGrandStrength scores a hand for playing Grand
// Returns normalized probability (0-1) of winning the Grand game
func (h *HeuristicGameChoiceStrategy) evaluateGrandStrength(cards game.Cards) float64 {
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
func (h *HeuristicGameChoiceStrategy) evaluateSuitStrength(cards game.Cards, trumpSuit game.Suit) float64 {
	score := 0.0

	// Count trumps (Jacks + trump suit)
	trumpCount := 0
	trumpPoints := 0
	hasTopTrumps := false

	for _, card := range cards {
		isTrump := card.Rank == game.Jack || card.Suit == trumpSuit
		if isTrump {
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
	for _, card := range hand {
		score := 0.0

		// High cards are best to discard (we want to keep low cards)
		switch card.Rank {
		case game.Ace:
			score += 100.0 // Aces are worst in Null - must discard
		case game.King:
			score += 80.0
		case game.Queen:
			score += 70.0
		case game.Jack:
			score += 60.0
		case game.Ten:
			score += 50.0
		case game.Nine:
			score -= 30.0 // Keep 9s
		case game.Eight:
			score -= 40.0 // Keep 8s
		case game.Seven:
			score -= 50.0 // Keep 7s - best cards in Null
		}

		scoredCards = append(scoredCards, cardScore{card, score})
	}

	// Sort by discard score (descending)
	for i := 0; i < len(scoredCards); i++ {
		for j := i + 1; j < len(scoredCards); j++ {
			if scoredCards[i].score < scoredCards[j].score {
				scoredCards[i], scoredCards[j] = scoredCards[j], scoredCards[i]
			}
		}
	}

	// Return top two cards to discard
	if len(scoredCards) >= 2 {
		return scoredCards[0].card, scoredCards[1].card
	}

	// Fallback (shouldn't happen)
	sortByValue(hand)
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
	sortByValue(validMoves)

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
	game.SortByNullRank(validMoves)

	// Leading the trick
	if len(trick) == 0 {
		// Lead lowest card to avoid winning
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
	game.SortByNullRank(validMoves)

	// Leading the trick
	if len(trick) == 0 {
		// Lead highest card to try to force declarer to win
		return validMoves[len(validMoves)-1]
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
		// Declarer already played - check if they're winning
		declarerWinning := true
		for _, card := range trick {
			if card != declarerCard && card.BeatsInNull(declarerCard, leadSuit) {
				declarerWinning = false
				break
			}
		}

		if declarerWinning {
			// Declarer is winning - try to take the trick with lowest winning card
			for _, move := range validMoves {
				if move.BeatsInNull(declarerCard, leadSuit) {
					return move
				}
			}
			// Can't beat - play lowest
			return validMoves[0]
		} else {
			// Declarer is losing - play lowest card to not interfere
			return validMoves[0]
		}
	} else {
		// Declarer hasn't played yet
		// Check if partner is already winning with a high card
		// Find highest card currently in trick
		highestInTrick := trick[0]
		for _, card := range trick[1:] {
			if card.BeatsInNull(highestInTrick, leadSuit) {
				highestInTrick = card
			}
		}

		// Check if any of our cards can beat partner's high card
		// If not, play low to let partner force declarer
		canBeatPartner := false
		for _, move := range validMoves {
			if move.BeatsInNull(highestInTrick, leadSuit) {
				canBeatPartner = true
				break
			}
		}

		if canBeatPartner {
			// We can beat partner - play our highest to force declarer even more
			return validMoves[len(validMoves)-1]
		} else {
			// Can't beat partner's high card - play lowest to let partner win and force declarer
			return validMoves[0]
		}
	}
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
		if h.wouldWinTrick(gs, move, trick) {
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
		winner := h.getTrickWinner(gs, trick)
		partner := h.getDefenderPartner(gs)
		if winner == partner {
			// Partner winning - play lowest card (don't waste high cards)
			return validMoves[0]
		}
	}

	// Try to beat the trick with LOWEST winning card (efficient)
	for _, move := range validMoves {
		if h.wouldWinTrick(gs, move, trick) {
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

func (h *HeuristicCardPlayStrategy) wouldWinTrick(gs *game.GameState, card game.Card, trick []game.Card) bool {
	for _, trickCard := range trick {
		if !gs.CardBeats(card, trickCard) {
			return false
		}
	}
	return true
}

func (h *HeuristicCardPlayStrategy) getTrickWinner(gs *game.GameState, trick []game.Card) game.GamePosition {
	if len(trick) == 0 {
		return gs.CurrentPlayer
	}

	winner := gs.TrickStarter
	winningCard := trick[0]

	for i := 1; i < len(trick); i++ {
		if gs.CardBeats(trick[i], winningCard) {
			winner = (gs.TrickStarter + game.GamePosition(i)) % 3
			winningCard = trick[i]
		}
	}

	return winner
}

func (h *HeuristicCardPlayStrategy) getDefenderPartner(gs *game.GameState) game.GamePosition {
	currentPlayer := gs.CurrentPlayer
	for pos := game.Dealer; pos <= game.Speaker; pos++ {
		if pos != currentPlayer && (gs.Declarer == nil || pos != *gs.Declarer) {
			return pos
		}
	}
	return game.Dealer
}

// Helper function to sort cards by value
func sortByValue(cards []game.Card) {
	// Simple bubble sort by card value
	for i := 0; i < len(cards); i++ {
		for j := i + 1; j < len(cards); j++ {
			if cards[i].Value() > cards[j].Value() {
				cards[i], cards[j] = cards[j], cards[i]
			}
		}
	}
}

// heuristicOrder orders moves by the sequence the heuristic agent would play them
// Optimized for minimax: avoids allocations and uses in-place sorting
func heuristicOrder(gs *game.GameState, moves []game.Card, isDeclarer bool) {
	if len(moves) <= 1 {
		return
	}

	trick := gs.Trick
	isTrumpCache := make([]bool, len(moves))

	// Pre-compute trump status for all moves (avoid repeated checks)
	for i, move := range moves {
		isTrumpCache[i] = move.Rank == game.Jack || (gs.Mode == game.ModeSuit && move.Suit == gs.TrumpSuit)
	}

	// Compute scores for sorting
	scores := make([]float64, len(moves))

	if isDeclarer {
		scoreDeclarerMoves(gs, moves, trick, isTrumpCache, scores)
	} else {
		scoreDefenderMoves(gs, moves, trick, isTrumpCache, scores)
	}

	// Insertion sort (efficient for small arrays, which is typical in card games)
	for i := 1; i < len(moves); i++ {
		move := moves[i]
		score := scores[i]
		j := i - 1

		// Move elements with lower scores to the right
		for j >= 0 && scores[j] < score {
			moves[j+1] = moves[j]
			scores[j+1] = scores[j]
			j--
		}
		moves[j+1] = move
		scores[j+1] = score
	}
}

func scoreDeclarerMoves(gs *game.GameState, moves []game.Card, trick []game.Card, isTrumpCache []bool, scores []float64) {
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

func scoreDefenderMoves(gs *game.GameState, moves []game.Card, trick []game.Card, isTrumpCache []bool, scores []float64) {
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
			winner := getTrickWinner(gs, trick)
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

// Helper functions for move ordering (optimized versions without strategy object)
func wouldWinTrick(gs *game.GameState, card game.Card, trick []game.Card) bool {
	for _, trickCard := range trick {
		if !gs.CardBeats(card, trickCard) {
			return false
		}
	}
	return true
}

func getTrickWinner(gs *game.GameState, trick []game.Card) game.GamePosition {
	if len(trick) == 0 {
		return gs.CurrentPlayer
	}

	winner := gs.TrickStarter
	winningCard := trick[0]

	for i := 1; i < len(trick); i++ {
		if gs.CardBeats(trick[i], winningCard) {
			winner = (gs.TrickStarter + game.GamePosition(i)) % 3
			winningCard = trick[i]
		}
	}

	return winner
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
func (h *HeuristicGameChoiceStrategy) evaluateNullStrength(cards game.Cards) float64 {
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

	// Normalize to 0-1 probability using sigmoid
	// Typical Null scores range from -250 (impossible - many high cards) to +300 (excellent - all low cards)
	// Shift center down to -15 to make good null hands competitive but not overwhelming
	// Temperature=75 for moderate calibration
	return sigmoid(score, -15.0, 75.0)
}

// sigmoid converts a raw score to a probability using a sigmoid function
// center: the score value that maps to 0.5 probability
// temperature: controls the steepness (higher = more gradual)
func sigmoid(score, center, temperature float64) float64 {
	return 1.0 / (1.0 + math.Exp(-(score-center)/temperature))
}
