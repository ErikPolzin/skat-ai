package encoding

import (
	"skat/game"
)

const (
	StateFeatureSize = 175
	ValidMoveCount   = 32
	NetworkInputSize = StateFeatureSize + ValidMoveCount
)

// NeuralCardPlayEncoding represents the DQN network input for card play
// Total: 175 active state features plus 32 valid-move mask features.
type NeuralCardPlayEncoding struct {
	// Card presence (96 features)
	MyHand      [32]float32 // Binary: cards in my hand
	TrickCards  [32]float32 // Binary: cards in current trick
	PlayedCards [32]float32 // Binary: all cards played so far (cumulative)

	// Game context (13 features)
	GameMode        [6]float32 // One-hot: [Grand, Clubs, Spades, Hearts, Diamonds, Null]
	TrickPosition   [3]float32 // [leading, second, third]
	Scores          [2]float32 // [derived declarer score / 120, derived opponent score / 120]
	TricksRemaining float32    // tricks_left / 10
	MyTrumpCount    float32    // Trumps in hand / total trumps in this game mode

	// Trick analysis (2 features)
	FollowCount  float32 // Cards in hand matching the led effective suit / 10
	TrumpInTrick float32 // Trump cards in the current trick / 3

	// Team and trick context (10 features). Relative positions retain seat order;
	// role and partner identity can be derived from DeclarerRelative.
	DeclarerRelative      [3]float32 // [self, next, previous]
	TrickLeaderRelative   [3]float32 // [self, next, previous]
	CurrentWinnerRelative [3]float32 // [self, next, previous]
	TrickPoints           float32    // Points currently in trick / 30

	// Inference and card counting (54 features). Card classes are
	// [clubs, spades, hearts, diamonds, trump], using effective rather than printed suit.
	VoidClasses      [3][5]float32 // Relative player x card-class void signals
	RemainingByClass [5]float32    // Unseen side suits / 8 and trumps / total trumps
	HigherUnseen     [32]float32   // Cards that can beat each card / 31; zero means a standing winner
	ContractContext  [2]float32    // [bid/264, hand_game]

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

	// Game context (13 features)
	copy(result[idx:idx+6], e.GameMode[:])
	idx += 6
	copy(result[idx:idx+3], e.TrickPosition[:])
	idx += 3
	copy(result[idx:idx+2], e.Scores[:])
	idx += 2
	result[idx] = e.TricksRemaining
	idx++
	result[idx] = e.MyTrumpCount
	idx++

	// Trick analysis (2 features)
	result[idx] = e.FollowCount
	idx++
	result[idx] = e.TrumpInTrick
	idx++

	// Team and trick context (10 features)
	copy(result[idx:idx+3], e.DeclarerRelative[:])
	idx += 3
	copy(result[idx:idx+3], e.TrickLeaderRelative[:])
	idx += 3
	copy(result[idx:idx+3], e.CurrentWinnerRelative[:])
	idx += 3
	result[idx] = e.TrickPoints
	idx++

	// Inference and card-counting context (54 features)
	for playerIdx := 0; playerIdx < 3; playerIdx++ {
		copy(result[idx:idx+5], e.VoidClasses[playerIdx][:])
		idx += 5
	}
	copy(result[idx:idx+5], e.RemainingByClass[:])
	idx += 5
	copy(result[idx:idx+32], e.HigherUnseen[:])
	idx += 32
	copy(result[idx:idx+2], e.ContractContext[:])

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

	if totalTrumps := getTotalTrumpCount(gs); totalTrumps > 0 {
		encoding.MyTrumpCount = float32(countTrumps(gs, myHand)) / float32(totalTrumps)
	}

	// All unseen-card features use the same inventory, so trump and winner counts
	// cannot drift apart as cards move from the hand to the table and history.
	remainingByClass, higherUnseen := cardCounts(gs, myPosition, myHand)

	trumpInTrick := 0
	for _, card := range gs.Trick {
		if isTrump(gs, card) {
			trumpInTrick++
		}
	}

	// 3. Trick Analysis (2 features)

	// Number of legal followers is the useful part of the old four-slot lead-suit
	// distribution; the led class itself is already present in TrickCards.
	if len(gs.Trick) > 0 {
		leadSuit := getEffectiveSuit(gs, gs.Trick[0])
		for _, card := range myHand {
			if getEffectiveSuit(gs, card) == leadSuit {
				encoding.FollowCount += 1.0 / 10.0
			}
		}
	}

	encoding.TrumpInTrick = float32(trumpInTrick) / 3.0

	// 4. Team and public inference context
	if gs.Declarer != nil {
		encoding.DeclarerRelative[relativePosition(myPosition, *gs.Declarer)] = 1.0
	}

	encoding.TrickLeaderRelative[relativePosition(myPosition, gs.TrickStarter)] = 1.0
	winner := currentTrickWinner(gs)
	encoding.CurrentWinnerRelative[relativePosition(myPosition, winner)] = 1.0

	trickPoints := 0
	for _, card := range gs.Trick {
		trickPoints += card.Value()
	}
	encoding.TrickPoints = clamp01(float32(trickPoints) / 30.0)

	encoding.VoidClasses = inferVoids(gs, myPosition)

	encoding.RemainingByClass = remainingByClass
	encoding.HigherUnseen = higherUnseen

	encoding.ContractContext[0] = clamp01(float32(gs.BidValue) / 264.0)
	if gs.PlayedHand {
		encoding.ContractContext[1] = 1.0
	}

	// 5. Valid Moves Mask (32 features)
	for _, card := range validMoves {
		encoding.ValidMovesMask[CardToIndex(card)] = 1.0
	}

	return encoding
}

// Helper functions

func relativePosition(from, to game.GamePosition) int {
	return int((to - from + 3) % 3)
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

func inferVoids(gs *game.GameState, myPosition game.GamePosition) [3][5]float32 {
	var voidClasses [3][5]float32

	starter := game.Listener
	for _, trick := range gs.CardsPlayed {
		markVoidsForTrick(gs, myPosition, starter, trick, &voidClasses)
		starter = completedTrickWinner(gs, starter, trick)
	}
	markVoidsForTrick(gs, myPosition, gs.TrickStarter, gs.Trick, &voidClasses)

	return voidClasses
}

func markVoidsForTrick(gs *game.GameState, myPosition, starter game.GamePosition, trick []game.Card, voidClasses *[3][5]float32) {
	if len(trick) < 2 {
		return
	}

	leadClass := cardClass(gs, trick[0])
	for i := 1; i < len(trick); i++ {
		card := trick[i]
		if cardClass(gs, card) == leadClass {
			continue
		}

		player := (starter + game.GamePosition(i)) % 3
		relative := relativePosition(myPosition, player)
		voidClasses[relative][leadClass] = 1.0
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

func cardCounts(gs *game.GameState, myPosition game.GamePosition, myHand []game.Card) ([5]float32, [32]float32) {
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
	// In a non-hand game the declarer picked up, and therefore knows, both skat
	// cards. They remain hidden information for defenders.
	if gs.Declarer != nil && myPosition == *gs.Declarer && !gs.PlayedHand {
		for _, card := range gs.Skat {
			known[CardToIndex(card)] = true
		}
	}

	var byClass [5]float32
	var higherUnseen [32]float32
	deck := game.NewDeck()
	for _, card := range deck {
		if known[CardToIndex(card)] {
			continue
		}
		class := cardClass(gs, card)
		if class == 4 {
			byClass[class] += 1.0 / float32(getTotalTrumpCount(gs))
		} else {
			byClass[class] += 1.0 / 8.0
		}
		for _, candidate := range deck {
			if gs.CardBeats(card, candidate) {
				higherUnseen[CardToIndex(candidate)] += 1.0 / 31.0
			}
		}
	}

	return byClass, higherUnseen
}

// cardClass maps the four followable side suits to 0..3 and all trumps to 4.
// Unlike printed suit, these classes match the game's follow-suit rules.
func cardClass(gs *game.GameState, card game.Card) int {
	if isTrump(gs, card) {
		return 4
	}
	return int(card.Suit) - 1
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
