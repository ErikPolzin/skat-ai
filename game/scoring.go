package game

// CalculateGameValue calculates the final game value according to Skat rules
func (gs *GameState) CalculateGameValue() int {
	// Base value depends on game mode
	baseValue := 0

	switch gs.Mode {
	case ModeGrand:
		baseValue = 24
	case ModeSuit:
		// Suit game base values
		switch gs.TrumpSuit {
		case Diamonds:
			baseValue = 9
		case Hearts:
			baseValue = 10
		case Spades:
			baseValue = 11
		case Clubs:
			baseValue = 12
		}
	case ModeNull:
		// Null games have fixed values
		return 23 // Basic null
	}

	// Calculate matadors (consecutive jacks from top that declarer has/doesn't have)
	matadorCount := gs.countMatadors()

	// Calculate multiplier: 1 (game) + matadors + schneider + schwarz
	multiplier := 1 + matadorCount // Always at least 1 for "game"

	// Add multipliers for schneider and schwarz
	declarerWon, schneider, schwarz := gs.GetGameResult()

	if schneider {
		multiplier++ // +1 for schneider
	}
	if schwarz {
		multiplier++ // +1 for schwarz
	}

	gameValue := baseValue * multiplier

	// If declarer lost, game value is doubled and negative
	if !declarerWon {
		return -2 * gameValue
	}

	return gameValue
}

// countMatadors counts consecutive jacks from Club Jack down that declarer has (with) or doesn't have (without)
func (gs *GameState) countMatadors() int {
	if gs.Declarer < 0 || gs.Declarer >= GamePosition(len(gs.Players)) {
		return 0
	}

	declarer := gs.Players[gs.Declarer]
	if declarer == nil {
		return 0
	}

	// Collect all cards declarer had access to (hand + skat)
	// In Skat, matadors are calculated from all 12 cards the declarer had
	// (10-card hand + 2 skat cards), NOT just the 10 playing cards
	allCards := make(Cards, len(declarer.Hand))
	copy(allCards, declarer.Hand)

	// Add skat cards to the count
	allCards = append(allCards, gs.Skat[0], gs.Skat[1])

	// Jack order for matadors: Club, Spade, Heart, Diamond (high to low)
	jackOrder := []Suit{Clubs, Spades, Hearts, Diamonds}

	// Check if declarer has Club Jack
	hasClubJack := false
	for _, card := range allCards {
		if card.Rank == Jack && card.Suit == Clubs {
			hasClubJack = true
			break
		}
	}

	matadors := 0
	if hasClubJack {
		// "With" matadors - count consecutive jacks from top
		for _, suit := range jackOrder {
			hasJack := false
			for _, card := range allCards {
				if card.Rank == Jack && card.Suit == suit {
					hasJack = true
					break
				}
			}
			if hasJack {
				matadors++
			} else {
				break // Stop at first missing jack
			}
		}
	} else {
		// "Without" matadors - count consecutive jacks from top that are missing
		for _, suit := range jackOrder {
			hasJack := false
			for _, card := range allCards {
				if card.Rank == Jack && card.Suit == suit {
					hasJack = true
					break
				}
			}
			if !hasJack {
				matadors++
			} else {
				break // Stop at first jack found
			}
		}
	}

	return matadors
}

// CalculatePlayerPoints calculates points for each player
// Returns a map of position -> points scored
// In Skat, only the declarer's score changes - opponents don't gain/lose individual points
func (gs *GameState) CalculatePlayerPoints() map[GamePosition]int {
	points := make(map[GamePosition]int)

	gameValue := gs.CalculateGameValue()

	// Only the declarer's score changes
	points[gs.Declarer] = gameValue

	// Opponents don't gain or lose points
	for pos := Dealer; pos <= Speaker; pos++ {
		if pos != gs.Declarer {
			points[pos] = 0
		}
	}

	return points
}
