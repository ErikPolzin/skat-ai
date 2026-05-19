package encoding

import (
	"skat/game"
)

const (
	StateFeatureSize = 155
	ValidMoveCount   = 32
	NetworkInputSize = StateFeatureSize + ValidMoveCount
)

// NeuralCardPlayEncoding represents the DQN network input for card play
// Total: 155 active state features plus 32 valid-move mask features.
type NeuralCardPlayEncoding struct {
	// Card presence (96 features)
	MyHand      [32]float32 // Binary: cards in my hand
	TrickCards  [32]float32 // Binary: cards in current trick
	PlayedCards [32]float32 // Binary: all cards played so far (cumulative)

	// Game context (14 features)
	GameMode        [6]float32 // One-hot: [Grand, Clubs, Spades, Hearts, Diamonds, Null]
	TrickPosition   [3]float32 // [leading, second, third]
	Scores          [2]float32 // [derived declarer score / 120, derived opponent score / 120]
	TricksRemaining float32    // tricks_left / 10
	TrumpSituation  [2]float32 // [my_trump_count/11, estimated_opponent_trumps/11]

	// Trick analysis (5 features - let network learn the rest)
	LeadSuitInHand [4]float32 // Count of lead suit cards in hand per suit (normalized)
	TrumpInTrick   float32    // Number of trump cards in trick

	// Team and trick context (16 features)
	Role              [3]float32 // [declarer, defender, defender_partner_known]
	DeclarerRelative  [3]float32 // Declarer relative to me: [self, next, previous]
	PartnerRelative   [3]float32 // Defender partner relative to me, zero for declarer
	TrickLeaderRole   [3]float32 // [me, partner, declarer]
	CurrentWinnerRole [3]float32 // [me, partner, declarer]
	TrickPoints       float32    // Points currently in trick / 30

	// Public inference context (24 features)
	VoidSuits       [3][4]float32 // Relative player x suit void signals from public play
	VoidTrump       [3]float32    // Relative player trump-void signals from public play
	RemainingBySuit [4]float32    // Unseen cards by actual suit / 8
	RemainingTrumps float32       // Unseen trumps / max trump count
	GamePressure    [4]float32    // [bid/264, declarer_needed/61, defenders_needed/60, hand_game]

	// Valid moves mask (32 features) - kept separate from state
	ValidMovesMask [32]float32 // Binary: 1 if card is playable, 0 otherwise
}

// ToStateArray converts encoding to state array (no valid mask).
func (e *NeuralCardPlayEncoding) ToSlice() [StateFeatureSize]float32 {
	result := [StateFeatureSize]float32{}
	idx := 0

	// Card presence (96)
	copy(result[idx:idx+32], e.MyHand[:])
	idx += 32
	copy(result[idx:idx+32], e.TrickCards[:])
	idx += 32
	copy(result[idx:idx+32], e.PlayedCards[:])
	idx += 32

	// Game context (14 features)
	copy(result[idx:idx+6], e.GameMode[:])
	idx += 6
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

	// Team and trick context (16 features)
	copy(result[idx:idx+3], e.Role[:])
	idx += 3
	copy(result[idx:idx+3], e.DeclarerRelative[:])
	idx += 3
	copy(result[idx:idx+3], e.PartnerRelative[:])
	idx += 3
	copy(result[idx:idx+3], e.TrickLeaderRole[:])
	idx += 3
	copy(result[idx:idx+3], e.CurrentWinnerRole[:])
	idx += 3
	result[idx] = e.TrickPoints
	idx++

	// Public inference context (24 features)
	for playerIdx := 0; playerIdx < 3; playerIdx++ {
		copy(result[idx:idx+4], e.VoidSuits[playerIdx][:])
		idx += 4
	}
	copy(result[idx:idx+3], e.VoidTrump[:])
	idx += 3
	copy(result[idx:idx+4], e.RemainingBySuit[:])
	idx += 4
	result[idx] = e.RemainingTrumps
	idx++
	copy(result[idx:idx+4], e.GamePressure[:])

	return result
}

// GetValidMask returns the valid moves mask
func (e *NeuralCardPlayEncoding) GetValidMask() [32]float32 {
	return e.ValidMovesMask
}

// ToNetworkInput returns the complete network input (state + valid move mask).
func (e *NeuralCardPlayEncoding) ToNetworkInput() [NetworkInputSize]float32 {
	result := [NetworkInputSize]float32{}
	state := e.ToSlice()
	copy(result[0:StateFeatureSize], state[:])
	copy(result[StateFeatureSize:NetworkInputSize], e.ValidMovesMask[:])
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

	// 2. Game Context (14 features)

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
	case game.ModeNull:
		encoding.GameMode[5] = 1.0
	}

	// Trick position
	trickPos := len(gs.Trick)
	if trickPos < 3 {
		encoding.TrickPosition[trickPos] = 1.0
	}

	// Scores (normalized to 0-1, max score is 120)
	encoding.Scores[0] = float32(gs.DeclarerCardScore()) / 120.0
	encoding.Scores[1] = float32(gs.OpponentCardScore()) / 120.0

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

	// 4. Team and public inference context
	if gs.Declarer != nil {
		declarer := *gs.Declarer
		if myPosition == declarer {
			encoding.Role[0] = 1.0
		} else {
			encoding.Role[1] = 1.0
			encoding.Role[2] = 1.0
			if partner, ok := defenderPartner(gs, myPosition); ok {
				encoding.PartnerRelative[relativePosition(myPosition, partner)] = 1.0
			}
		}
		encoding.DeclarerRelative[relativePosition(myPosition, declarer)] = 1.0
	}

	encoding.TrickLeaderRole[roleIndex(gs, myPosition, gs.TrickStarter)] = 1.0
	winner := currentTrickWinner(gs)
	encoding.CurrentWinnerRole[roleIndex(gs, myPosition, winner)] = 1.0

	trickPoints := 0
	for _, card := range gs.Trick {
		trickPoints += card.Value()
	}
	encoding.TrickPoints = clamp01(float32(trickPoints) / 30.0)

	voidSuits, voidTrump := inferVoids(gs, myPosition)
	encoding.VoidSuits = voidSuits
	encoding.VoidTrump = voidTrump

	remainingBySuit, remainingTrumps := remainingCards(gs, myHand)
	encoding.RemainingBySuit = remainingBySuit
	encoding.RemainingTrumps = remainingTrumps

	encoding.GamePressure[0] = clamp01(float32(gs.BidValue) / 264.0)
	if gs.Mode == game.ModeNull {
		if gs.DeclarerCardScore() > 0 {
			encoding.GamePressure[1] = 1.0
		}
		encoding.GamePressure[2] = 1.0 - encoding.GamePressure[1]
	} else {
		encoding.GamePressure[1] = clamp01(float32(61-gs.DeclarerCardScore()) / 61.0)
		encoding.GamePressure[2] = clamp01(float32(60-gs.OpponentCardScore()) / 60.0)
	}
	if gs.PlayedHand {
		encoding.GamePressure[3] = 1.0
	}

	// 5. Valid Moves Mask (32 features)
	for _, card := range validMoves {
		encoding.ValidMovesMask[CardToIndex(card)] = 1.0
	}

	return encoding
}

// Helper functions

func defenderPartner(gs *game.GameState, myPosition game.GamePosition) (game.GamePosition, bool) {
	if gs.Declarer == nil || myPosition == *gs.Declarer {
		return 0, false
	}
	for _, pos := range game.AllPositions {
		if pos != myPosition && pos != *gs.Declarer {
			return pos, true
		}
	}
	return 0, false
}

func relativePosition(from, to game.GamePosition) int {
	return int((to - from + 3) % 3)
}

func roleIndex(gs *game.GameState, myPosition, pos game.GamePosition) int {
	if pos == myPosition {
		return 0
	}
	if gs.Declarer != nil && pos == *gs.Declarer {
		return 2
	}
	return 1
}

func currentTrickWinner(gs *game.GameState) game.GamePosition {
	if len(gs.Trick) == 0 {
		return gs.TrickStarter
	}

	winnerOffset := 0
	winningCard := gs.Trick[0]
	for i := 1; i < len(gs.Trick); i++ {
		if gs.CardBeats(gs.Trick[i], winningCard) {
			winnerOffset = i
			winningCard = gs.Trick[i]
		}
	}
	return (gs.TrickStarter + game.GamePosition(winnerOffset)) % 3
}

func inferVoids(gs *game.GameState, myPosition game.GamePosition) ([3][4]float32, [3]float32) {
	var voidSuits [3][4]float32
	var voidTrump [3]float32

	starter := game.Listener
	for _, trick := range gs.CardsPlayed {
		markVoidsForTrick(gs, myPosition, starter, trick, &voidSuits, &voidTrump)
		starter = completedTrickWinner(gs, starter, trick)
	}
	markVoidsForTrick(gs, myPosition, gs.TrickStarter, gs.Trick, &voidSuits, &voidTrump)

	return voidSuits, voidTrump
}

func markVoidsForTrick(gs *game.GameState, myPosition, starter game.GamePosition, trick []game.Card, voidSuits *[3][4]float32, voidTrump *[3]float32) {
	if len(trick) < 2 {
		return
	}

	leadSuit := getEffectiveSuit(gs, trick[0])
	leadIsTrump := isTrump(gs, trick[0])
	for i := 1; i < len(trick); i++ {
		card := trick[i]
		if getEffectiveSuit(gs, card) == leadSuit {
			continue
		}

		player := (starter + game.GamePosition(i)) % 3
		relative := relativePosition(myPosition, player)
		if leadIsTrump || leadSuit == game.NoSuit {
			voidTrump[relative] = 1.0
			continue
		}

		suitIdx := int(leadSuit) - 1
		if suitIdx >= 0 && suitIdx < 4 {
			voidSuits[relative][suitIdx] = 1.0
		}
	}
}

func completedTrickWinner(gs *game.GameState, starter game.GamePosition, trick []game.Card) game.GamePosition {
	if len(trick) == 0 {
		return starter
	}
	winnerOffset := 0
	winningCard := trick[0]
	for i := 1; i < len(trick); i++ {
		if gs.CardBeats(trick[i], winningCard) {
			winnerOffset = i
			winningCard = trick[i]
		}
	}
	return (starter + game.GamePosition(winnerOffset)) % 3
}

func remainingCards(gs *game.GameState, myHand []game.Card) ([4]float32, float32) {
	var known [32]bool
	for _, card := range myHand {
		known[CardToIndex(card)] = true
	}
	for _, card := range gs.Trick {
		known[CardToIndex(card)] = true
	}
	for _, trick := range gs.CardsPlayed {
		for _, card := range trick {
			known[CardToIndex(card)] = true
		}
	}

	var bySuit [4]float32
	remainingTrumps := 0
	for _, card := range game.NewDeck() {
		if known[CardToIndex(card)] {
			continue
		}
		suitIdx := int(card.Suit) - 1
		if suitIdx >= 0 && suitIdx < 4 {
			bySuit[suitIdx] += 1.0 / 8.0
		}
		if isTrump(gs, card) {
			remainingTrumps++
		}
	}

	maxTrumps := getTotalTrumpCount(gs)
	if maxTrumps == 0 {
		return bySuit, 0
	}
	return bySuit, clamp01(float32(remainingTrumps) / float32(maxTrumps))
}

func clamp01(value float32) float32 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

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
