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

	// Calculate matadors (consecutive jacks from top)
	// This is simplified - in real Skat you count consecutive jacks with/without
	matadors := 1 // Always at least 1 (game)

	// Add multipliers for schneider and schwarz
	declarerWon, schneider, schwarz := gs.GetGameResult()

	if schneider {
		matadors++ // +1 for schneider
	}
	if schwarz {
		matadors++ // +1 for schwarz
	}

	gameValue := baseValue * matadors

	// If declarer lost, game value is doubled and negative
	if !declarerWon {
		return -2 * gameValue
	}

	return gameValue
}

// CalculatePlayerPoints calculates points for each player
// Returns a map of position -> points scored
func (gs *GameState) CalculatePlayerPoints() map[GamePosition]int {
	points := make(map[GamePosition]int)

	gameValue := gs.CalculateGameValue()

	declarerWon, _, _ := gs.GetGameResult()

	if declarerWon {
		// Declarer wins the game value
		points[gs.Declarer] = gameValue

		// Opponents each lose half (rounded)
		opponentLoss := -gameValue / 2
		for pos := Dealer; pos <= Speaker; pos++ {
			if pos != gs.Declarer {
				points[pos] = opponentLoss
			}
		}
	} else {
		// Declarer loses double the game value
		points[gs.Declarer] = gameValue // Already negative from CalculateGameValue

		// Opponents each win half
		opponentWin := -gameValue / 2 // Negate because gameValue is negative
		for pos := Dealer; pos <= Speaker; pos++ {
			if pos != gs.Declarer {
				points[pos] = opponentWin
			}
		}
	}

	return points
}
