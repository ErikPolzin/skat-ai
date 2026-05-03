package strategies

import (
	"skat/game"
)

// HeuristicBiddingStrategy uses hand strength heuristics to make bidding decisions
type HeuristicBiddingStrategy struct{}

func (h *HeuristicBiddingStrategy) GetName() string {
	return "HeuristicBidding"
}

func (h *HeuristicBiddingStrategy) ShouldBid(gs *game.GameState, hand []game.Card, currentBid int) bool {
	cards := game.Cards(hand)

	// Use the game choice strategy to determine which game we'd actually play
	// This ensures bidding is based on realistic game choice, not theoretical maximum
	gameChoiceStrategy := &HeuristicGameChoiceStrategy{}

	// Get the next bid value to estimate what we'd need to declare
	nextBid := gs.GetNextBidValue()
	if nextBid == 0 {
		return false
	}

	// Determine which game we would choose at this bid level
	mode, suit := gameChoiceStrategy.ChooseGame(hand, nextBid)

	// Get the game value for our chosen game
	gameValue := cards.GameValue(mode, suit)

	// Bid if our chosen game can meet the bid with small safety margin
	// Only need slight buffer to avoid overbids
	safetyMargin := 1.05 // Just 5% safety margin
	return float64(gameValue) >= float64(nextBid)*safetyMargin
}

// HeuristicGameChoiceStrategy chooses game based on hand strength heuristics
type HeuristicGameChoiceStrategy struct{}

func (h *HeuristicGameChoiceStrategy) GetName() string {
	return "HeuristicGameChoice"
}

func (h *HeuristicGameChoiceStrategy) ChooseGame(hand []game.Card, bidValue int) (game.GameMode, game.Suit) {
	cards := game.Cards(hand)

	// Evaluate each possible game with refined scoring
	type gameOption struct {
		mode  game.GameMode
		suit  game.Suit
		value int
		score float64 // refined heuristic score
	}

	var options []gameOption

	// Check Grand - evaluate strength
	grandValue := cards.GameValue(game.ModeGrand, game.NoSuit)
	if grandValue >= bidValue {
		score := h.evaluateGrandStrength(cards)
		options = append(options, gameOption{game.ModeGrand, game.NoSuit, grandValue, score})
	}

	// Check all suit games
	for suit := game.Clubs; suit <= game.Diamonds; suit++ {
		suitValue := cards.GameValue(game.ModeSuit, suit)
		if suitValue >= bidValue {
			score := h.evaluateSuitStrength(cards, suit)
			options = append(options, gameOption{game.ModeSuit, suit, suitValue, score})
		}
	}

	// No viable options - return highest value game regardless
	if len(options) == 0 {
		bestValue := 0
		bestMode := game.ModeGrand
		bestSuit := game.NoSuit

		grandValue := cards.GameValue(game.ModeGrand, game.NoSuit)
		if grandValue > bestValue {
			bestValue = grandValue
			bestMode = game.ModeGrand
			bestSuit = game.NoSuit
		}

		for suit := game.Clubs; suit <= game.Diamonds; suit++ {
			suitValue := cards.GameValue(game.ModeSuit, suit)
			if suitValue > bestValue {
				bestValue = suitValue
				bestMode = game.ModeSuit
				bestSuit = suit
			}
		}

		return bestMode, bestSuit
	}

	// Select game with best heuristic score
	best := options[0]
	for _, opt := range options[1:] {
		if opt.score > best.score {
			best = opt
		}
	}

	return best.mode, best.suit
}

func (h *HeuristicGameChoiceStrategy) ChooseSkatDiscard(hand []game.Card, mode game.GameMode, trumpSuit game.Suit) (game.Card, game.Card) {
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

	// Grand is the hardest game type - only choose with excellent hands
	// Need 3+ jacks OR 4 jacks for viability
	if jackCount < 3 {
		return -150.0 // Massive penalty - Grand nearly impossible with <3 jacks
	}

	// Base score starts low - Grand must prove itself
	score = 0.0

	// Bonus for jacks
	score += float64(jackCount * 12)

	// Count Aces and estimate tricks
	aceCount := 0
	tenCount := 0
	for _, card := range cards {
		if card.Rank == game.Ace {
			aceCount++
			score += 18
		}
		if card.Rank == game.Ten {
			tenCount++
		}
	}

	// Grand requires MANY winners - 6+ jacks+aces to be safe
	totalWinners := jackCount + aceCount
	if totalWinners < 6 {
		score -= float64((6 - totalWinners) * 30) // Heavy penalty
	} else if totalWinners == 7 {
		score += 40 // Excellent
	} else if totalWinners == 8 {
		score += 60 // Perfect Grand hand
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

	return score
}

// evaluateSuitStrength scores a hand for playing a specific suit
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

			// Bonus for high trumps
			if card.Rank == game.Jack {
				switch card.Suit {
				case game.Clubs:
					score += 12
					hasTopTrumps = true
				case game.Spades:
					score += 10
					hasTopTrumps = true
				case game.Hearts:
					score += 8
				case game.Diamonds:
					score += 6
				}
			} else if card.Rank == game.Ace {
				score += 10
				hasTopTrumps = true
			} else if card.Rank == game.Ten {
				score += 8
			}
		}
	}

	// Trump length is critical - need at least 5 for safety
	if trumpCount < 5 {
		score -= float64((5 - trumpCount) * 10) // Penalty for short trumps
	}

	// Strong bonus for trump length with better scaling
	score += float64(trumpCount * trumpCount * 4)
	score += float64(trumpPoints) * 0.8

	// Bonus for having top trump control
	if hasTopTrumps {
		score += 15
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

	return score
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
		if h.isTrump(gs, card) {
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

	if isDefender {
		return h.selectDefenderMove(gs, validMoves)
	}
	return h.selectDeclarerMove(gs, validMoves)
}

func (h *HeuristicCardPlayStrategy) selectDeclarerMove(gs *game.GameState, validMoves []game.Card) game.Card {
	trick := gs.Trick

	// Leading the trick
	if len(trick) == 0 {
		// Strategy: Cash Aces before drawing trumps
		// This prevents defenders from trumping our high-value winners

		// First, check for unprotected Aces in side suits
		for _, move := range validMoves {
			if move.Rank == game.Ace && !h.isTrump(gs, move) {
				// Lead the Ace to cash it
				return move
			}
		}

		// Next, check for protected Ace-10 combinations (Ace with 10)
		for suit := game.Clubs; suit <= game.Diamonds; suit++ {
			if h.isTrump(gs, game.Card{Suit: suit, Rank: game.Ace}) {
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
			if h.isTrump(gs, move) {
				trumpCount++
				if !hasTrump || h.cardStrongerThan(gs, move, highestTrump) {
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
				if !h.isTrump(gs, move) {
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
					if validMoves[i].Suit == shortestSuit && !h.isTrump(gs, validMoves[i]) {
						return validMoves[i]
					}
				}
			}
		}

		// Default declarer strategy: lead from SHORT side suits
		// This allows declarer to ruff later rounds of that suit
		suitLengths := make(map[game.Suit]int)
		for _, move := range validMoves {
			if !h.isTrump(gs, move) {
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
			if !h.isTrump(gs, validMoves[i]) {
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
			if h.isTrump(gs, move) {
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
				if validMoves[i].Suit == longestSuit && !h.isTrump(gs, validMoves[i]) {
					return validMoves[i]
				}
			}
		}

		// Strategy 3: Lead any Ace we have (cash winners)
		for _, move := range validMoves {
			if move.Rank == game.Ace && !h.isTrump(gs, move) {
				return move
			}
		}

		// Strategy 4: Lead low card from side suit to find partner's strength
		for _, move := range validMoves {
			if !h.isTrump(gs, move) && move.Value() == 0 {
				return move
			}
		}

		// Strategy 5: Only lead trump if we have nothing else or very strong trumps
		if trumpCount >= 4 {
			// We have strong trump control - lead trump to attack declarer
			for i := len(validMoves) - 1; i >= 0; i-- {
				if h.isTrump(gs, validMoves[i]) {
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

func (h *HeuristicCardPlayStrategy) isTrump(gs *game.GameState, card game.Card) bool {
	if card.Rank == game.Jack {
		return true
	}
	if gs.Mode == game.ModeSuit && card.Suit == gs.TrumpSuit {
		return true
	}
	return false
}

func (h *HeuristicCardPlayStrategy) cardStrongerThan(gs *game.GameState, card1, card2 game.Card) bool {
	return gs.CardBeats(card1, card2)
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
