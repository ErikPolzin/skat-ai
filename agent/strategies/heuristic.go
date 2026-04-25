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

	// Find game type with highest value that meets bid
	bestValue := 0
	bestMode := game.ModeGrand
	bestSuit := game.NoSuit

	// Check Grand
	grandValue := cards.GameValue(game.ModeGrand, game.NoSuit)
	if grandValue >= bidValue && grandValue > bestValue {
		bestValue = grandValue
		bestMode = game.ModeGrand
		bestSuit = game.NoSuit
	}

	// Check all suit games
	for suit := game.Clubs; suit <= game.Diamonds; suit++ {
		suitValue := cards.GameValue(game.ModeSuit, suit)
		if suitValue >= bidValue && suitValue > bestValue {
			bestValue = suitValue
			bestMode = game.ModeSuit
			bestSuit = suit
		}
	}

	return bestMode, bestSuit
}

func (h *HeuristicGameChoiceStrategy) ChooseSkatDiscard(hand []game.Card, mode game.GameMode, trumpSuit game.Suit) (game.Card, game.Card) {
	// Simple heuristic: discard lowest non-trump cards
	var nonTrump []game.Card

	for _, card := range hand {
		isTrump := card.Rank == game.Jack ||
			(mode == game.ModeSuit && card.Suit == trumpSuit)

		if !isTrump {
			nonTrump = append(nonTrump, card)
		}
	}

	// Sort non-trump by value (ascending)
	sortByValue(nonTrump)

	// If we have enough non-trump, discard two lowest
	if len(nonTrump) >= 2 {
		return nonTrump[0], nonTrump[1]
	}

	// Otherwise, discard lowest overall cards
	sortByValue(hand)
	return hand[0], hand[1]
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
	isDefender := currentPlayer != gs.Declarer

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
		if pos != currentPlayer && pos != gs.Declarer {
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
