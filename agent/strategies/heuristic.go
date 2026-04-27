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

	// Calculate maximum achievable game value across all game types
	maxValue := 0

	// Check Grand
	grandValue := cards.GameValue(game.ModeGrand, game.NoSuit)
	if grandValue > maxValue {
		maxValue = grandValue
	}

	// Check all suit games
	for suit := game.Clubs; suit <= game.Diamonds; suit++ {
		suitValue := cards.GameValue(game.ModeSuit, suit)
		if suitValue > maxValue {
			maxValue = suitValue
		}
	}

	// Bid if we can meet the current bid with some safety margin
	// Use 1.2x margin to account for uncertainty
	safetyMargin := 1.2
	return float64(maxValue) >= float64(currentBid)*safetyMargin
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
				score += 20
			case game.Spades:
				score += 15
			case game.Hearts:
				score += 10
			case game.Diamonds:
				score += 5
			}
		}
	}

	// Strong bonus for having multiple jacks
	score += float64(jackCount * jackCount * 5)

	// Count Aces (strong in Grand)
	aceCount := 0
	for _, card := range cards {
		if card.Rank == game.Ace {
			aceCount++
			score += 10
		}
	}

	// Bonus for having Ace-10 combinations
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
			score += 8
		}
	}

	return score
}

// evaluateSuitStrength scores a hand for playing a specific suit
func (h *HeuristicGameChoiceStrategy) evaluateSuitStrength(cards game.Cards, trumpSuit game.Suit) float64 {
	score := 0.0

	// Count trumps (Jacks + trump suit)
	trumpCount := 0
	trumpPoints := 0

	for _, card := range cards {
		isTrump := card.Rank == game.Jack || card.Suit == trumpSuit
		if isTrump {
			trumpCount++
			trumpPoints += card.Value()

			// Bonus for high trumps
			if card.Rank == game.Jack {
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
			} else if card.Rank == game.Ace {
				score += 8
			} else if card.Rank == game.Ten {
				score += 5
			}
		}
	}

	// Strong bonus for trump length (need control)
	score += float64(trumpCount * trumpCount * 3)
	score += float64(trumpPoints)

	// Evaluate side suits
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
			}
			if card.Rank == game.Ten {
				hasTen = true
			}
		}
		if hasAce && hasTen {
			score += 10
		} else if hasAce {
			score += 5
		}

		// Bonus for void suits (can trump in)
		if len(suitCards) == 0 {
			score += 12
		}
		// Small bonus for short suits
		if len(suitCards) == 1 {
			score += 6
		}
	}

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

	// Prefer discarding from longer suits (helps create voids)
	score += float64(suitCounts[card.Suit] * 5)

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
type HeuristicCardPlayStrategy struct{}

func (h *HeuristicCardPlayStrategy) GetName() string {
	return "HeuristicCardPlay"
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
		// Lead with trump to draw out defenders' trumps
		for i := len(validMoves) - 1; i >= 0; i-- {
			if h.isTrump(gs, validMoves[i]) {
				return validMoves[i]
			}
		}
		// Otherwise lead high value cards
		for i := len(validMoves) - 1; i >= 0; i-- {
			if validMoves[i].Value() >= 10 {
				return validMoves[i]
			}
		}
		// Lead highest card
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
		// Lead trump if we have it
		for i := len(validMoves) - 1; i >= 0; i-- {
			if h.isTrump(gs, validMoves[i]) {
				return validMoves[i]
			}
		}
		// Lead Ace if we have one
		for _, move := range validMoves {
			if move.Rank == game.Ace {
				return move
			}
		}
		// Otherwise lead low card
		return validMoves[0]
	}

	// Check if partner is winning (in 3rd position)
	if len(trick) == 2 {
		winner := h.getTrickWinner(gs, trick)
		partner := h.getDefenderPartner(gs)
		if winner == partner {
			// Partner winning - play lowest card
			return validMoves[0]
		}
	}

	// Try to beat the trick
	for i := len(validMoves) - 1; i >= 0; i-- {
		if h.wouldWinTrick(gs, validMoves[i], trick) {
			return validMoves[i]
		}
	}

	// Can't win - play lowest card
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
