package game

// Result calculates and returns the complete game result including all scoring breakdown
func (gs *GameState) Result() GameResult {
	result := GameResult{}

	// Check if game was forfeited
	result.IsForfeit = gs.ForfeitedPlayer >= 0

	// Base value depends on game mode
	switch gs.Mode {
	case ModeGrand:
		result.BaseValue = 24
	case ModeSuit:
		// Suit game base values
		switch gs.TrumpSuit {
		case Diamonds:
			result.BaseValue = 9
		case Hearts:
			result.BaseValue = 10
		case Spades:
			result.BaseValue = 11
		case Clubs:
			result.BaseValue = 12
		}
	case ModeNull:
		// Null games have fixed values
		result.BaseValue = 23
		result.Matadors = 0
		result.Multiplier = 1
		result.DeclarerWon = gs.DeclarerScore == 0
		result.IsSchneider = false
		result.IsSchwarz = false
		result.Value = 23
		if !result.DeclarerWon {
			result.Value = -46 // Null lost is doubled
		}
		return result
	}

	// Use stored matadors value (set when game type declared)
	matadorCount := gs.Matadors
	if matadorCount < 0 {
		matadorCount = -matadorCount // Use absolute value for multiplier
	}

	// Calculate multiplier: 1 (game) + matadors + hand + schneider + schwarz + announcements
	result.Multiplier = 1 + matadorCount

	// Determine game outcome
	result.DeclarerWon, result.IsSchneider, result.IsSchwarz = gs.GetGameResult()

	// Store hand and announcement flags
	result.PlayedHand = gs.PlayedHand
	result.AnnouncedSchneider = gs.AnnouncedSchneider
	result.AnnouncedSchwarz = gs.AnnouncedSchwarz

	// Add hand bonus (playing without picking up skat)
	if gs.PlayedHand {
		result.Multiplier++
	}

	// Add schneider bonuses
	if result.IsSchneider {
		result.Multiplier++
	}
	if gs.AnnouncedSchneider {
		result.Multiplier++
	}

	// Add schwarz bonuses
	if result.IsSchwarz {
		result.Multiplier++
	}
	if gs.AnnouncedSchwarz {
		result.Multiplier++
	}

	result.Matadors = gs.Matadors
	gameValue := result.BaseValue * result.Multiplier

	// If declarer overbid (game value < bid value), they automatically lose
	// and lose double the BID value (not game value)
	if gs.Overbid {
		result.DeclarerWon = false
		result.Value = -2 * int(gs.BidValue)
		return result
	}

	// If declarer lost, game value is doubled and negative
	if !result.DeclarerWon {
		result.Value = -2 * gameValue
	} else {
		result.Value = gameValue
	}

	return result
}

// CalculateGameValue calculates the final game value (for backward compatibility)
func (gs *GameState) CalculateGameValue() int {
	return gs.Result().Value
}

// countMatadorsWithSign returns matadors with sign (positive=with, negative=without)
func (gs *GameState) countMatadorsWithSign() int {
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
		return matadors // Positive = with
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
		return -matadors // Negative = without
	}
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
