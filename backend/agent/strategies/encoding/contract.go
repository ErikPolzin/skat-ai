package encoding

import "skat/game"

const ContractFeatureSize = 85

// NeuralContractEncoding represents a declarer hand plus candidate contract.
// It is intentionally hand-level, unlike the trick-state card-play encoding.
type NeuralContractEncoding struct {
	HandCards [32]float32
	GameMode  [6]float32 // [Grand, Clubs, Spades, Hearts, Diamonds, Null]

	HandSize           float32
	TotalPoints        float32
	GameValue          float32
	TrumpCount         float32
	TrumpPoints        float32
	TopJacks           float32
	AceTenPairs        float32
	MaxSuitLength      float32
	SideAces           float32
	VoidSuits          float32
	SingletonSuits     float32
	HasTopTrumpControl float32

	RankCounts         [8]float32
	SuitCounts         [4]float32
	SuitPoints         [4]float32
	NonTrumpSuitCounts [4]float32
	NullSuitRisk       [4]float32
	TrumpCards         [11]float32
}

func (e *NeuralContractEncoding) ToSlice() [ContractFeatureSize]float32 {
	var result [ContractFeatureSize]float32
	idx := 0

	copy(result[idx:idx+32], e.HandCards[:])
	idx += 32
	copy(result[idx:idx+6], e.GameMode[:])
	idx += 6

	result[idx] = e.HandSize
	idx++
	result[idx] = e.TotalPoints
	idx++
	result[idx] = e.GameValue
	idx++
	result[idx] = e.TrumpCount
	idx++
	result[idx] = e.TrumpPoints
	idx++
	result[idx] = e.TopJacks
	idx++
	result[idx] = e.AceTenPairs
	idx++
	result[idx] = e.MaxSuitLength
	idx++
	result[idx] = e.SideAces
	idx++
	result[idx] = e.VoidSuits
	idx++
	result[idx] = e.SingletonSuits
	idx++
	result[idx] = e.HasTopTrumpControl
	idx++

	copy(result[idx:idx+8], e.RankCounts[:])
	idx += 8
	copy(result[idx:idx+4], e.SuitCounts[:])
	idx += 4
	copy(result[idx:idx+4], e.SuitPoints[:])
	idx += 4
	copy(result[idx:idx+4], e.NonTrumpSuitCounts[:])
	idx += 4
	copy(result[idx:idx+4], e.NullSuitRisk[:])
	idx += 4
	copy(result[idx:idx+11], e.TrumpCards[:])

	return result
}

func EncodeNeuralContract(hand game.Cards, mode game.GameMode, trumpSuit game.Suit) NeuralContractEncoding {
	var encoding NeuralContractEncoding

	for _, card := range hand {
		encoding.HandCards[CardToIndex(card)] = 1
		encoding.RankCounts[card.Rank] += 1.0 / 4.0
		if card.Suit >= game.Clubs && card.Suit <= game.Diamonds {
			suitIdx := int(card.Suit - 1)
			encoding.SuitCounts[suitIdx] += 1.0 / 8.0
			encoding.SuitPoints[suitIdx] += float32(card.Value()) / 30.0
		}
	}

	switch mode {
	case game.ModeGrand:
		encoding.GameMode[0] = 1
	case game.ModeSuit:
		switch trumpSuit {
		case game.Clubs:
			encoding.GameMode[1] = 1
		case game.Spades:
			encoding.GameMode[2] = 1
		case game.Hearts:
			encoding.GameMode[3] = 1
		case game.Diamonds:
			encoding.GameMode[4] = 1
		}
	case game.ModeNull:
		encoding.GameMode[5] = 1
	}

	encoding.HandSize = clamp01(float32(len(hand)) / 12.0)
	encoding.TotalPoints = clamp01(float32(hand.TotalPoints()) / 120.0)
	encoding.GameValue = clamp01(float32(hand.GameValue(mode, trumpSuit)) / 264.0)

	totalTrumps := contractTotalTrumps(mode)
	if totalTrumps > 0 {
		encoding.TrumpCount = float32(hand.ContractTrumpCount(mode, trumpSuit)) / float32(totalTrumps)
		encoding.TrumpPoints = clamp01(float32(hand.ContractTrumpPoints(mode, trumpSuit)) / 60.0)
	}

	encoding.TopJacks = float32(hand.CountTopJacks()) / 4.0
	encoding.AceTenPairs = float32(hand.CountAceTenPairs()) / 4.0
	encoding.MaxSuitLength = float32(hand.MaxSuitLength()) / 8.0
	encoding.SideAces = float32(hand.SideAceCount(mode, trumpSuit)) / 4.0
	encoding.VoidSuits = float32(hand.VoidSuitCount(mode, trumpSuit)) / 4.0
	encoding.SingletonSuits = float32(hand.SingletonSuitCount(mode, trumpSuit)) / 4.0
	if hand.HasTopTrumpControl(mode, trumpSuit) {
		encoding.HasTopTrumpControl = 1
	}

	nonTrumpCounts := hand.NonTrumpSuitCounts(mode, trumpSuit)
	for suit := game.Clubs; suit <= game.Diamonds; suit++ {
		encoding.NonTrumpSuitCounts[suit-1] = float32(nonTrumpCounts[suit]) / 8.0
		encoding.NullSuitRisk[suit-1] = nullSuitRisk(hand, suit)
	}

	for _, card := range hand {
		if idx, ok := contractTrumpFeatureIndex(card, mode, trumpSuit); ok {
			encoding.TrumpCards[idx] = 1
		}
	}

	return encoding
}

func contractTotalTrumps(mode game.GameMode) int {
	switch mode {
	case game.ModeGrand:
		return 4
	case game.ModeSuit:
		return 11
	default:
		return 0
	}
}

func contractTrumpFeatureIndex(card game.Card, mode game.GameMode, trumpSuit game.Suit) (int, bool) {
	if mode == game.ModeNull {
		return 0, false
	}
	if card.Rank == game.Jack {
		switch card.Suit {
		case game.Clubs:
			return 0, true
		case game.Spades:
			return 1, true
		case game.Hearts:
			return 2, true
		case game.Diamonds:
			return 3, true
		}
	}
	if mode != game.ModeSuit || card.Suit != trumpSuit {
		return 0, false
	}
	switch card.Rank {
	case game.Seven:
		return 4, true
	case game.Eight:
		return 5, true
	case game.Nine:
		return 6, true
	case game.Ten:
		return 7, true
	case game.Queen:
		return 8, true
	case game.King:
		return 9, true
	case game.Ace:
		return 10, true
	}
	return 0, false
}

func nullSuitRisk(hand game.Cards, suit game.Suit) float32 {
	cardsInSuit := 0
	highCards := 0
	for _, card := range hand {
		if card.Suit != suit {
			continue
		}
		cardsInSuit++
		if card.Rank == game.Ace || card.Rank == game.King || card.Rank == game.Queen || card.Rank == game.Jack {
			highCards++
		}
	}
	if cardsInSuit == 0 {
		return 0
	}
	return clamp01(float32(highCards) / float32(cardsInSuit))
}
