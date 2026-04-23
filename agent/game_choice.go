package agent

import (
	"math/rand"
	"skat/game"
)

// GameAction encodes game mode + suit choice into a single integer
// Grand: 0, Suit Clubs: 1, Suit Spades: 2, Suit Hearts: 3, Suit Diamonds: 4
func encodeGameAction(mode game.GameMode, suit game.Suit) int {
	if mode == game.ModeGrand {
		return 0
	}
	return int(suit) + 1 // Clubs=1, Spades=2, Hearts=3, Diamonds=4
}

func decodeGameAction(action int) (game.GameMode, game.Suit) {
	if action == 0 {
		return game.ModeGrand, game.Clubs // Suit doesn't matter for Grand
	}
	return game.ModeSuit, game.Suit(action - 1)
}

// ChooseGame selects the best game mode using Q-learning
// State: hand after picking up skat (includes skat cards)
func (sa *SkatAgent) ChooseGame(state *game.GameState) (game.GameMode, game.Suit) {
	hand := state.Players[state.CurrentPlayer].Hand
	handState := sa.EvaluateHandWithSkat(hand)

	if sa.gameChoiceQTable[handState] == nil {
		sa.gameChoiceQTable[handState] = make(map[int]float64)
	}

	// Get valid game actions (those that meet the bid value)
	cards := game.Cards(hand)
	validActions := make([]int, 0, 5)

	// Check Grand (action 0)
	if cards.GameValue(game.ModeGrand, game.NoSuit) >= sa.CurrentBid {
		validActions = append(validActions, 0)
	}

	// Check each suit (actions 1-4)
	for suit := game.Clubs; suit <= game.Diamonds; suit++ {
		if cards.GameValue(game.ModeSuit, suit) >= sa.CurrentBid {
			validActions = append(validActions, int(suit)+1)
		}
	}

	// If no valid games, fall back to all games (shouldn't happen with proper bidding)
	if len(validActions) == 0 {
		for i := 0; i < 5; i++ {
			validActions = append(validActions, i)
		}
	}

	var mode game.GameMode
	var suit game.Suit

	if rand.Float64() < sa.GameChoiceEpsilon {
		// Explore: choose random VALID game
		action := validActions[rand.Intn(len(validActions))]
		mode, suit = decodeGameAction(action)
		sa.CurrentGameChoice = action
	} else {
		// Exploit: choose best Q-value among VALID actions, with heuristic fallback
		bestAction := validActions[0]
		bestQ := sa.getGameChoiceQ(handState, validActions[0])

		for _, action := range validActions[1:] {
			q := sa.getGameChoiceQ(handState, action)
			if q > bestQ {
				bestQ = q
				bestAction = action
			}
		}

		// If all Q-values are zero (untrained), use heuristic
		if bestQ == 0.0 {
			mode, suit = sa.heuristicGameChoiceValid(hand, sa.CurrentBid)
			sa.CurrentGameChoice = encodeGameAction(mode, suit)
		} else {
			mode, suit = decodeGameAction(bestAction)
			sa.CurrentGameChoice = bestAction
		}
	}

	return mode, suit
}

// EvaluateHandWithSkat counts high cards INCLUDING skat
func (sa *SkatAgent) EvaluateHandWithSkat(hand []game.Card) int {
	aces := 0
	tens := 0
	jacks := 0
	suitCounts := make(map[game.Suit]int)

	for _, card := range hand {
		if card.Rank == game.Jack {
			jacks++
		}
		if card.Rank == game.Ace {
			aces++
		}
		if card.Rank == game.Ten {
			tens++
		}
		suitCounts[card.Suit]++
	}

	maxSuitCount := 0
	for _, count := range suitCounts {
		if count > maxSuitCount {
			maxSuitCount = count
		}
	}

	// For game choice, we don't have a bid context, so use 0 for gamesPlayable
	// (This function is called after bidding is complete)
	gamesPlayable := 0

	return encodeHandState(aces, tens, jacks, maxSuitCount, gamesPlayable)
}

// heuristicGameChoice provides a fallback strategy when Q-values are untrained
func (sa *SkatAgent) heuristicGameChoice(hand []game.Card) (game.GameMode, game.Suit) {
	jacks := 0
	jackSuits := make(map[game.Suit]bool)
	suitCounts := make(map[game.Suit]int)
	suitPoints := make(map[game.Suit]int)

	for _, card := range hand {
		if card.Rank == game.Jack {
			jacks++
			jackSuits[card.Suit] = true
		}
		suitCounts[card.Suit]++
		suitPoints[card.Suit] += card.Value()
	}

	// Strong jacks → Grand
	if jacks >= 3 {
		return game.ModeGrand, game.Clubs
	}
	if jackSuits[game.Clubs] && jackSuits[game.Spades] {
		return game.ModeGrand, game.Clubs
	}

	// Find best suit game
	bestSuit := game.Clubs
	bestScore := 0

	for suit := game.Clubs; suit <= game.Diamonds; suit++ {
		score := suitCounts[suit]*10 + suitPoints[suit]
		if score > bestScore {
			bestScore = score
			bestSuit = suit
		}
	}

	if suitCounts[bestSuit] >= 5 {
		return game.ModeSuit, bestSuit
	}

	return game.ModeGrand, game.Clubs
}

// heuristicGameChoiceValid chooses the best game that meets the bid value
func (sa *SkatAgent) heuristicGameChoiceValid(hand []game.Card, bidValue int) (game.GameMode, game.Suit) {
	cards := game.Cards(hand)
	jacks := 0
	jackSuits := make(map[game.Suit]bool)
	suitCounts := make(map[game.Suit]int)
	suitPoints := make(map[game.Suit]int)

	for _, card := range hand {
		if card.Rank == game.Jack {
			jacks++
			jackSuits[card.Suit] = true
		}
		suitCounts[card.Suit]++
		suitPoints[card.Suit] += card.Value()
	}

	// Strong jacks → try Grand first if valid
	if (jacks >= 3 || (jackSuits[game.Clubs] && jackSuits[game.Spades])) {
		if cards.GameValue(game.ModeGrand, game.NoSuit) >= bidValue {
			return game.ModeGrand, game.Clubs
		}
	}

	// Find best VALID suit game
	type suitScore struct {
		suit  game.Suit
		score int
		valid bool
	}

	suits := make([]suitScore, 0, 4)
	for suit := game.Clubs; suit <= game.Diamonds; suit++ {
		score := suitCounts[suit]*10 + suitPoints[suit]
		valid := cards.GameValue(game.ModeSuit, suit) >= bidValue
		suits = append(suits, suitScore{suit, score, valid})
	}

	// Sort by score descending
	for i := 0; i < len(suits)-1; i++ {
		for j := i + 1; j < len(suits); j++ {
			if suits[i].score < suits[j].score {
				suits[i], suits[j] = suits[j], suits[i]
			}
		}
	}

	// Pick best valid suit
	for _, s := range suits {
		if s.valid && suitCounts[s.suit] >= 5 {
			return game.ModeSuit, s.suit
		}
	}

	// Fallback: try Grand if valid
	if cards.GameValue(game.ModeGrand, game.NoSuit) >= bidValue {
		return game.ModeGrand, game.Clubs
	}

	// Fallback: pick any valid suit
	for _, s := range suits {
		if s.valid {
			return game.ModeSuit, s.suit
		}
	}

	// Last resort: return best heuristic choice (even if invalid)
	return sa.heuristicGameChoice(hand)
}

// OnGameChoiceEnd updates Q-values for game choice based on outcome
func (sa *SkatAgent) OnGameChoiceEnd(handState int, wonGame bool, pointsScored int) {
	reward := 0.0

	if wonGame {
		// Reward proportional to how well we won
		safetyMargin := float64(pointsScored-61) / 60.0
		reward = 1.0 + safetyMargin*0.3
	} else {
		// Penalty proportional to how badly we lost
		if pointsScored >= 55 {
			reward = -0.4 // Close loss
		} else if pointsScored >= 45 {
			reward = -0.7
		} else {
			reward = -1.0 // Bad loss
		}
	}

	oldQ := sa.getGameChoiceQ(handState, sa.CurrentGameChoice)
	newQ := oldQ + sa.gameChoiceAlpha*(reward-oldQ)
	sa.setGameChoiceQ(handState, sa.CurrentGameChoice, newQ)
}

func (sa *SkatAgent) getGameChoiceQ(handState, action int) float64 {
	if sa.gameChoiceQTable[handState] == nil {
		return 0.0
	}
	return sa.gameChoiceQTable[handState][action]
}

func (sa *SkatAgent) setGameChoiceQ(handState, action int, value float64) {
	if sa.gameChoiceQTable[handState] == nil {
		sa.gameChoiceQTable[handState] = make(map[int]float64)
	}
	sa.gameChoiceQTable[handState][action] = value
}

// ChooseSkatDiscard selects which 2 cards to discard based on chosen game mode
// This is a game-aware heuristic that discards strategically
func (sa *SkatAgent) ChooseSkatDiscard(hand []game.Card, mode game.GameMode, trumpSuit game.Suit) (game.Card, game.Card) {
	if len(hand) != 12 {
		// Fallback: just return first 2 cards if hand size is wrong
		return hand[0], hand[1]
	}

	// Score each card based on game mode
	type cardScore struct {
		card  game.Card
		score int
	}

	scored := make([]cardScore, len(hand))

	for i, card := range hand {
		score := 0

		if mode == game.ModeGrand {
			// Grand: Only jacks are trump
			if card.Rank == game.Jack {
				score = 1000 // Never discard jacks
				// Rank jacks by order
				switch card.Suit {
				case game.Clubs:
					score += 40
				case game.Spades:
					score += 30
				case game.Hearts:
					score += 20
				case game.Diamonds:
					score += 10
				}
			} else {
				// For non-jacks, value high cards
				score = card.Value() * 2
				// Prefer keeping aces
				if card.Rank == game.Ace {
					score += 50
				}
			}
		} else if mode == game.ModeSuit {
			// Suit game: Jacks + trump suit are trump
			if card.Rank == game.Jack {
				score = 1000 // Never discard jacks
				switch card.Suit {
				case game.Clubs:
					score += 40
				case game.Spades:
					score += 30
				case game.Hearts:
					score += 20
				case game.Diamonds:
					score += 10
				}
			} else if card.Suit == trumpSuit {
				// Trump cards are valuable
				score = 500 + card.Value()*2
				if card.Rank == game.Ace {
					score += 50
				}
			} else {
				// Off-suit cards: value based on points, but deprioritize
				score = card.Value()
				// Don't discard off-suit aces if possible
				if card.Rank == game.Ace {
					score += 30
				}
			}
		}

		scored[i] = cardScore{card, score}
	}

	// Sort by score (ascending - we want to discard lowest scored cards)
	for i := 0; i < len(scored)-1; i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[i].score > scored[j].score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	// Return 2 lowest scored cards
	return scored[0].card, scored[1].card
}
