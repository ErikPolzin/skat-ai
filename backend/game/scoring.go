package game

// Result calculates and returns the complete game result including all scoring breakdown
func (gs *GameState) Result() GameResult {
	result := GameResult{}

	// Check if game was forfeited
	result.IsForfeit = gs.ForfeitedPlayer != nil

	// Handle forfeit games with fixed penalty value
	if result.IsForfeit {
		result.DeclarerWon = gs.Declarer != nil && *gs.ForfeitedPlayer != *gs.Declarer
		if result.DeclarerWon {
			result.Value = 120 // Declarer wins when opponent forfeits
		} else {
			result.Value = -120 // Declarer loses when they forfeit
		}
		return result
	}

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
		result.BaseValue = gs.nullGameValue()
		result.Matadors = 0
		result.Multiplier = 1
		result.DeclarerWon = gs.DeclarerScore == 0
		result.IsSchneider = false
		result.IsSchwarz = false
		result.PlayedHand = gs.PlayedHand
		result.Value = result.BaseValue
		if !result.DeclarerWon {
			result.Value = -2 * result.BaseValue // Null lost is doubled
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

func (gs *GameState) nullGameValue() int {
	if gs.PlayedHand {
		return 35
	}
	return 23
}

func (gs *GameState) PlayerResults() *[3]PlayerResultState {
	if gs.Phase != PhaseComplete || gs.Declarer == nil {
		return nil
	}
	results := [3]PlayerResultState{}

	for pos := Dealer; pos <= Speaker; pos++ {
		player := gs.Players[pos]
		if player == nil {
			continue
		}
		isDeclarer := gs.Declarer != nil && pos == *gs.Declarer
		isForfeit := gs.ForfeitedPlayer != nil && pos == *gs.ForfeitedPlayer
		results[int(pos)] = PlayerResultState{
			GameID:         gs.ID,
			SessionID:      gs.SessionID,
			PlayerID:       player.ID,
			IsWinner:       gs.isWinner(pos),
			IsDeclarer:     isDeclarer,
			IsOverbid:      isDeclarer && gs.Overbid,
			IsForfeit:      isForfeit,
			PlayerPosition: pos,
			PlayerPoints:   gs.CalculatePlayerPoints(pos),
		}
	}
	return &results
}

// countMatadorsWithSign returns matadors with sign (positive=with, negative=without)
func (gs *GameState) countMatadorsWithSign() int {
	if gs.Declarer == nil {
		return 0
	}

	if *gs.Declarer < 0 || *gs.Declarer >= GamePosition(len(gs.Players)) {
		return 0
	}

	declarer := gs.Players[*gs.Declarer]
	if declarer == nil {
		return 0
	}

	// Collect all cards declarer had access to (hand + skat)
	// In Skat, matadors are calculated from all 12 cards the declarer had
	// (10-card hand + 2 skat cards), NOT just the 10 playing cards
	allCards := make(Cards, len(declarer.Hand))
	copy(allCards, declarer.Hand)
	allCards = append(allCards, gs.Skat[0], gs.Skat[1])

	// Matador order starts with the four jacks. In suit games it continues
	// through the trump suit cards in descending trump order.
	matadorOrder := []Card{
		{Suit: Clubs, Rank: Jack},
		{Suit: Spades, Rank: Jack},
		{Suit: Hearts, Rank: Jack},
		{Suit: Diamonds, Rank: Jack},
	}
	if gs.Mode == ModeSuit {
		matadorOrder = append(matadorOrder,
			Card{Suit: gs.TrumpSuit, Rank: Ace},
			Card{Suit: gs.TrumpSuit, Rank: Ten},
			Card{Suit: gs.TrumpSuit, Rank: King},
			Card{Suit: gs.TrumpSuit, Rank: Queen},
			Card{Suit: gs.TrumpSuit, Rank: Nine},
			Card{Suit: gs.TrumpSuit, Rank: Eight},
			Card{Suit: gs.TrumpSuit, Rank: Seven},
		)
	}

	// Check if declarer has Club Jack
	hasClubJack := false
	for _, card := range allCards {
		if card == matadorOrder[0] {
			hasClubJack = true
			break
		}
	}

	matadors := 0
	if hasClubJack {
		// "With" matadors - count consecutive top trumps from the Club Jack.
		for _, matador := range matadorOrder {
			hasMatador := false
			for _, card := range allCards {
				if card == matador {
					hasMatador = true
					break
				}
			}
			if hasMatador {
				matadors++
			} else {
				break // Stop at first missing top trump
			}
		}
		return matadors // Positive = with
	} else {
		// "Without" matadors - count consecutive top trumps that are missing.
		for _, matador := range matadorOrder {
			hasMatador := false
			for _, card := range allCards {
				if card == matador {
					hasMatador = true
					break
				}
			}
			if !hasMatador {
				matadors++
			} else {
				break // Stop at first top trump found
			}
		}
		return -matadors // Negative = without
	}
}

// CalculatePlayerPoints calculates points for a player
// In Skat, only the declarer's score changes - opponents don't gain/lose individual points
func (gs *GameState) CalculatePlayerPoints(pos GamePosition) int {
	if gs.ForfeitedPlayer != nil {
		if pos == *gs.ForfeitedPlayer {
			return -120
		} else {
			return 60
		}
	} else if gs.Declarer != nil && pos == *gs.Declarer {
		return gs.Result().Value
	}
	return 0
}

func (gs *GameState) isWinner(pos GamePosition) bool {
	if gs.ForfeitedPlayer != nil {
		return pos != *gs.ForfeitedPlayer
	}

	if gs.Declarer == nil {
		return false
	}

	result := gs.Result()
	if pos == *gs.Declarer {
		// Declarer wins if they won the game
		return result.DeclarerWon
	} else {
		// Defenders win if declarer lost
		return !result.DeclarerWon
	}
}
