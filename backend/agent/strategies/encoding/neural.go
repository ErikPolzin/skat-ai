package encoding

import (
	"skat/game"
)

// NeuralCardPlayEncoding represents the DQN network input for card play
// Total: 114 active features (simplified, removed redundant heuristics)
type NeuralCardPlayEncoding struct {
	// Card presence (96 features)
	MyHand      [32]float32 // Binary: cards in my hand
	TrickCards  [32]float32 // Binary: cards in current trick
	PlayedCards [32]float32 // Binary: all cards played so far (cumulative)

	// Game context (13 features - simplified)
	GameMode        [5]float32 // One-hot: [Grand, Clubs, Spades, Hearts, Diamonds]
	TrickPosition   [3]float32 // [leading, second, third]
	Scores          [2]float32 // [declarer_score/120, opponent_score/120]
	TricksRemaining float32    // tricks_left / 10
	TrumpSituation  [2]float32 // [my_trump_count/11, estimated_opponent_trumps/11]

	// Trick analysis (5 features - let network learn the rest)
	LeadSuitInHand [4]float32 // Count of lead suit cards in hand per suit (normalized)
	TrumpInTrick   float32    // Number of trump cards in trick

	// Valid moves mask (32 features) - kept separate from state
	ValidMovesMask [32]float32 // Binary: 1 if card is playable, 0 otherwise
}

// ToStateArray converts encoding to state array (114 features, no valid mask)
func (e *NeuralCardPlayEncoding) ToSlice() [114]float32 {
	result := [114]float32{}
	idx := 0

	// Card presence (96)
	copy(result[idx:idx+32], e.MyHand[:])
	idx += 32
	copy(result[idx:idx+32], e.TrickCards[:])
	idx += 32
	copy(result[idx:idx+32], e.PlayedCards[:])
	idx += 32

	// Game context (13 features)
	copy(result[idx:idx+5], e.GameMode[:])
	idx += 5
	copy(result[idx:idx+3], e.TrickPosition[:])
	idx += 3
	copy(result[idx:idx+2], e.Scores[:])
	idx += 2
	result[idx] = e.TricksRemaining
	idx++
	copy(result[idx:idx+2], e.TrumpSituation[:])
	idx += 2

	// Trick analysis (5 features)
	copy(result[idx:idx+4], e.LeadSuitInHand[:])
	idx += 4
	result[idx] = e.TrumpInTrick
	idx++

	// Total: 96 + 13 + 5 = 114 features

	return result
}

// GetValidMask returns the valid moves mask
func (e *NeuralCardPlayEncoding) GetValidMask() [32]float32 {
	return e.ValidMovesMask
}

// ToNetworkInput returns the complete network input (114 state + 32 mask = 146)
func (e *NeuralCardPlayEncoding) ToNetworkInput() [146]float32 {
	result := [146]float32{}
	state := e.ToSlice()
	copy(result[0:114], state[:])
	copy(result[114:146], e.ValidMovesMask[:])
	return result
}

func EncodeNeuralCardPlay(gs *game.GameState, myPosition game.GamePosition, validMoves []game.Card) NeuralCardPlayEncoding {
	var encoding NeuralCardPlayEncoding

	myHand := gs.Players[myPosition].Hand

	// 1. Card Presence (96 features)
	for _, card := range myHand {
		encoding.MyHand[CardToIndex(card)] = 1.0
	}

	for _, card := range gs.Trick {
		encoding.TrickCards[CardToIndex(card)] = 1.0
	}

	for _, trick := range gs.CardsPlayed {
		for _, card := range trick {
			encoding.PlayedCards[CardToIndex(card)] = 1.0
		}
	}

	// 2. Game Context (13 features)

	// Game mode one-hot
	switch gs.Mode {
	case game.ModeGrand:
		encoding.GameMode[0] = 1.0
	case game.ModeSuit:
		switch gs.TrumpSuit {
		case game.Clubs:
			encoding.GameMode[1] = 1.0
		case game.Spades:
			encoding.GameMode[2] = 1.0
		case game.Hearts:
			encoding.GameMode[3] = 1.0
		case game.Diamonds:
			encoding.GameMode[4] = 1.0
		}
	}

	// Trick position
	trickPos := len(gs.Trick)
	if trickPos < 3 {
		encoding.TrickPosition[trickPos] = 1.0
	}

	// Scores (normalized to 0-1, max score is 120)
	encoding.Scores[0] = float32(gs.DeclarerScore) / 120.0
	encoding.Scores[1] = float32(gs.OpponentScore) / 120.0

	// Tricks remaining (each player has 10 cards, so 10 tricks total)
	cardsInHand := len(myHand)
	encoding.TricksRemaining = float32(cardsInHand) / 10.0

	// Trump situation
	myTrumpCount := countTrumps(gs, myHand)
	encoding.TrumpSituation[0] = float32(myTrumpCount) / 11.0 // Max 11 trump in suit games

	// Estimate opponent trumps (total trump - my trump - trump in trick - trump played)
	totalTrump := getTotalTrumpCount(gs)
	trumpInTrick := 0
	for _, card := range gs.Trick {
		if isTrump(gs, card) {
			trumpInTrick++
		}
	}
	trumpPlayed := 0
	for _, trick := range gs.CardsPlayed {
		for _, card := range trick {
			if isTrump(gs, card) {
				trumpPlayed++
			}
		}
	}
	opponentTrump := totalTrump - myTrumpCount - trumpInTrick - trumpPlayed
	if opponentTrump < 0 {
		opponentTrump = 0
	}
	encoding.TrumpSituation[1] = float32(opponentTrump) / 11.0

	// 3. Trick Analysis (5 features)

	// Lead suit distribution in my hand
	if len(gs.Trick) > 0 {
		leadCard := gs.Trick[0]
		leadSuit := getEffectiveSuit(gs, leadCard)

		for _, card := range myHand {
			if getEffectiveSuit(gs, card) == leadSuit {
				suitIdx := int(card.Suit) - 1
				if suitIdx >= 0 && suitIdx < 4 {
					encoding.LeadSuitInHand[suitIdx] += 1.0 / 8.0 // Normalize by max suit length
				}
			}
		}
	}

	// Trump in trick
	encoding.TrumpInTrick = float32(trumpInTrick)

	// 4. Valid Moves Mask (32 features)
	for _, card := range validMoves {
		encoding.ValidMovesMask[CardToIndex(card)] = 1.0
	}

	return encoding
}

// Helper functions

func countTrumps(gs *game.GameState, hand []game.Card) int {
	count := 0
	for _, card := range hand {
		if isTrump(gs, card) {
			count++
		}
	}
	return count
}

func isTrump(gs *game.GameState, card game.Card) bool {
	if gs.Mode == game.ModeNull {
		return false
	}
	if card.Rank == game.Jack {
		return true
	}
	if gs.Mode == game.ModeSuit && card.Suit == gs.TrumpSuit {
		return true
	}
	return false
}

func getTotalTrumpCount(gs *game.GameState) int {
	if gs.Mode == game.ModeGrand {
		return 4 // Only jacks
	} else if gs.Mode == game.ModeSuit {
		return 11 // 4 jacks + 7 suit cards
	}
	return 0
}

func getEffectiveSuit(gs *game.GameState, card game.Card) game.Suit {
	if gs.Mode != game.ModeNull && card.Rank == game.Jack {
		if gs.Mode == game.ModeGrand {
			return game.NoSuit
		}
		return gs.TrumpSuit
	}
	if gs.Mode == game.ModeSuit && card.Suit == gs.TrumpSuit {
		return gs.TrumpSuit
	}
	return card.Suit
}

func CardToIndex(card game.Card) int {
	suitOffset := int(card.Suit-1) * 8 // Clubs=0, Spades=8, Hearts=16, Diamonds=24
	rankOffset := int(card.Rank)       // Seven=0, Eight=1, ..., Ace=7
	return suitOffset + rankOffset
}
