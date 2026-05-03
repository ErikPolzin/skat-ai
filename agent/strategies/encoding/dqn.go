package encoding

import (
	"skat/game"
)

// DQNCardPlayEncoding represents the DQN network input for card play
// Total: 130 active features (PlayerRole removed from neural encoding)
type DQNCardPlayEncoding struct {
	// Card presence (96 features)
	MyHand      [32]float32 // Binary: cards in my hand
	TrickCards  [32]float32 // Binary: cards in current trick
	PlayedCards [32]float32 // Binary: all cards played so far (cumulative)

	// Game context (18 features - removed PlayerRole)
	GameMode        [5]float32 // One-hot: [Grand, Clubs, Spades, Hearts, Diamonds]
	TrickPosition   [3]float32 // [leading, second, third]
	Scores          [2]float32 // [declarer_score/120, opponent_score/120]
	TricksRemaining float32    // tricks_left / 10
	TrumpSituation  [2]float32 // [my_trump_count/11, estimated_opponent_trumps/11]
	Matadors        float32    // matadors / 4
	HandQuality     [4]float32 // [strong_hand, winning_position, losing_position, critical_trick]

	// Trick analysis (16 features)
	TrickValue         float32    // Current trick point value / 30
	LeadSuitInHand     [4]float32 // Count of lead suit cards in hand per suit (normalized)
	CanWinTrick        float32    // Binary: can I win this trick
	HighestTrickRank   float32    // Highest rank in trick / 7
	TrumpInTrick       float32    // Number of trump cards in trick
	PositionalFeatures [8]float32 // Advanced positional analysis

	// Valid moves mask (32 features) - kept separate from state
	ValidMovesMask [32]float32 // Binary: 1 if card is playable, 0 otherwise
}

// ToStateArray converts encoding to state array (130 features, no valid mask, no PlayerRole)
func (e *DQNCardPlayEncoding) ToSlice() [130]float32 {
	result := [130]float32{}
	idx := 0

	// Card presence (96)
	copy(result[idx:idx+32], e.MyHand[:])
	idx += 32
	copy(result[idx:idx+32], e.TrickCards[:])
	idx += 32
	copy(result[idx:idx+32], e.PlayedCards[:])
	idx += 32

	// Game context (18 features - PlayerRole removed)
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
	result[idx] = e.Matadors
	idx++
	copy(result[idx:idx+4], e.HandQuality[:])
	idx += 4

	// Trick analysis (16)
	result[idx] = e.TrickValue
	idx++
	copy(result[idx:idx+4], e.LeadSuitInHand[:])
	idx += 4
	result[idx] = e.CanWinTrick
	idx++
	result[idx] = e.HighestTrickRank
	idx++
	result[idx] = e.TrumpInTrick
	idx++
	copy(result[idx:idx+8], e.PositionalFeatures[:])
	idx += 8

	// Total: 96 + 5 + 3 + 2 + 1 + 2 + 1 + 4 + 1 + 4 + 1 + 1 + 1 + 8 = 130

	return result
}

// GetValidMask returns the valid moves mask
func (e *DQNCardPlayEncoding) GetValidMask() [32]float32 {
	return e.ValidMovesMask
}

// ToNetworkInput returns the complete network input (130 state + 32 mask = 162)
func (e *DQNCardPlayEncoding) ToNetworkInput() [162]float32 {
	result := [162]float32{}
	state := e.ToSlice()
	copy(result[0:130], state[:])
	copy(result[130:162], e.ValidMovesMask[:])
	return result
}

func EncodeDQNCardPlay(gs *game.GameState, myPosition game.GamePosition, validMoves []game.Card) DQNCardPlayEncoding {
	var encoding DQNCardPlayEncoding

	myHand := gs.Players[myPosition].Hand
	isDeclarer := gs.Declarer != nil && *gs.Declarer == myPosition

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

	// 2. Game Context (20 features)

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

	// Matadors
	encoding.Matadors = float32(gs.Matadors) / 4.0

	// Hand quality indicators
	encoding.HandQuality = calculateHandQuality(gs, myHand, isDeclarer)

	// 3. Trick Analysis (16 features)

	// Current trick value
	trickValue := 0
	for _, card := range gs.Trick {
		trickValue += card.Value()
	}
	encoding.TrickValue = float32(trickValue) / 30.0 // Max ~30 points

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

	// Can win trick
	if canWinTrick(gs, validMoves, gs.Trick) {
		encoding.CanWinTrick = 1.0
	}

	// Highest rank in trick
	if len(gs.Trick) > 0 {
		highestRank := gs.Trick[0].Rank
		for _, card := range gs.Trick[1:] {
			if card.Rank > highestRank {
				highestRank = card.Rank
			}
		}
		encoding.HighestTrickRank = float32(highestRank) / 7.0 // Ace = 7
	}

	// Trump in trick
	encoding.TrumpInTrick = float32(trumpInTrick)

	// Positional features
	encoding.PositionalFeatures = calculatePositionalFeatures(gs, myPosition, isDeclarer)

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

func canWinTrick(gs *game.GameState, validMoves []game.Card, trick []game.Card) bool {
	if len(trick) == 0 {
		return true // Leading always has chance to win
	}

	winningCard := trick[0]
	for _, card := range trick[1:] {
		if gs.CardBeats(card, winningCard) {
			winningCard = card
		}
	}

	for _, myCard := range validMoves {
		if gs.CardBeats(myCard, winningCard) {
			return true
		}
	}
	return false
}

func calculateHandQuality(gs *game.GameState, hand []game.Card, isDeclarer bool) [4]float32 {
	var quality [4]float32

	// Strong hand indicator (has high-value cards and trumps)
	points := 0
	trumpCount := 0
	for _, card := range hand {
		points += card.Value()
		if isTrump(gs, card) {
			trumpCount++
		}
	}
	quality[0] = float32(points) / 60.0 // Normalize by half total points

	// Winning position (declarer with good score, or defender with opponent struggling)
	if isDeclarer {
		if gs.DeclarerScore > 60 {
			quality[1] = 1.0
		}
	} else {
		if gs.OpponentScore > 60 {
			quality[1] = 1.0
		}
	}

	// Losing position
	if isDeclarer {
		if gs.DeclarerScore < 30 && len(hand) < 5 {
			quality[2] = 1.0
		}
	} else {
		if gs.OpponentScore < 30 && len(hand) < 5 {
			quality[2] = 1.0
		}
	}

	// Critical trick (close game, few cards left)
	scoreDiff := gs.DeclarerScore - gs.OpponentScore
	if scoreDiff < 0 {
		scoreDiff = -scoreDiff
	}
	if len(hand) <= 3 && scoreDiff < 20 {
		quality[3] = 1.0
	}

	return quality
}

func calculatePositionalFeatures(gs *game.GameState, myPosition game.GamePosition, isDeclarer bool) [8]float32 {
	var features [8]float32

	trickPos := len(gs.Trick)

	// Feature 0: Am I last to play (positional advantage)
	if trickPos == 2 {
		features[0] = 1.0
	}

	// Feature 1: Am I leading
	if trickPos == 0 {
		features[1] = 1.0
	}

	// Feature 2: Partner is winning (for defenders)
	if !isDeclarer && len(gs.Trick) > 0 {
		// Check if another defender is currently winning
		winningPos := gs.TrickStarter
		winningCard := gs.Trick[0]
		for i := 1; i < len(gs.Trick); i++ {
			if gs.CardBeats(gs.Trick[i], winningCard) {
				winningCard = gs.Trick[i]
				winningPos = (gs.TrickStarter + game.GamePosition(i)) % 3
			}
		}
		if gs.Declarer != nil && winningPos != *gs.Declarer && winningPos != myPosition {
			features[2] = 1.0
		}
	}

	// Feature 3: Declarer is winning (important for defenders)
	if !isDeclarer && len(gs.Trick) > 0 && gs.Declarer != nil {
		winningPos := gs.TrickStarter
		winningCard := gs.Trick[0]
		for i := 1; i < len(gs.Trick); i++ {
			if gs.CardBeats(gs.Trick[i], winningCard) {
				winningCard = gs.Trick[i]
				winningPos = (gs.TrickStarter + game.GamePosition(i)) % 3
			}
		}
		if winningPos == *gs.Declarer {
			features[3] = 1.0
		}
	}

	// Feature 4: High-value trick (>15 points)
	trickValue := 0
	for _, card := range gs.Trick {
		trickValue += card.Value()
	}
	if trickValue > 15 {
		features[4] = 1.0
	}

	// Feature 5: Low-value trick (<5 points)
	if trickValue < 5 {
		features[5] = 1.0
	}

	// Feature 6: Must follow suit (restricted choice)
	if len(gs.Trick) > 0 {
		leadSuit := getEffectiveSuit(gs, gs.Trick[0])
		sameSuitCount := 0
		for _, card := range gs.Players[myPosition].Hand {
			if getEffectiveSuit(gs, card) == leadSuit {
				sameSuitCount++
			}
		}
		if sameSuitCount > 0 && sameSuitCount < len(gs.Players[myPosition].Hand) {
			features[6] = 1.0
		}
	}

	// Feature 7: Free choice (can discard anything)
	if len(gs.Trick) > 0 {
		leadSuit := getEffectiveSuit(gs, gs.Trick[0])
		canFollowSuit := false
		for _, card := range gs.Players[myPosition].Hand {
			if getEffectiveSuit(gs, card) == leadSuit {
				canFollowSuit = true
				break
			}
		}
		if !canFollowSuit {
			features[7] = 1.0
		}
	}

	return features
}

func CardToIndex(card game.Card) int {
	suitOffset := int(card.Suit-1) * 8 // Clubs=0, Spades=8, Hearts=16, Diamonds=24
	rankOffset := int(card.Rank)       // Seven=0, Eight=1, ..., Ace=7
	return suitOffset + rankOffset
}
