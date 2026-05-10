package game

import "math/rand"

// Suit represents a card suit
type Suit int

const (
	NoSuit Suit = iota // Special marker for Jack effective suit in Grand games
	Clubs
	Spades
	Hearts
	Diamonds
)

// Rank represents a card rank in Skat
type Rank int

const (
	Seven Rank = iota
	Eight
	Nine
	Ten
	Jack
	Queen
	King
	Ace
)

// Card represents a playing card
type Card struct {
	Suit Suit `json:"suit"`
	Rank Rank `json:"rank"`
}

// Value returns the point value of the card
func (c Card) Value() int {
	switch c.Rank {
	case Ace:
		return 11
	case Ten:
		return 10
	case King:
		return 4
	case Queen:
		return 3
	case Jack:
		return 2
	default:
		return 0
	}
}

// NullRank returns the rank value in Null games (higher = stronger)
// In Null: A > K > Q > J > 10 > 9 > 8 > 7
func (c Card) NullRank() int {
	return c.Rank.NullRank()
}

// NullRank returns the rank value in Null games (higher = stronger)
// In Null: A > K > Q > J > 10 > 9 > 8 > 7
func (r Rank) NullRank() int {
	switch r {
	case Seven:
		return 1
	case Eight:
		return 2
	case Nine:
		return 3
	case Ten:
		return 4
	case Jack:
		return 5
	case Queen:
		return 6
	case King:
		return 7
	case Ace:
		return 8
	default:
		return 0
	}
}

// SkatRank returns the rank value for non-trump cards in regular Skat games
// Skat rank order: A > 10 > K > Q > 9 > 8 > 7
func (r Rank) SkatRank() int {
	switch r {
	case Seven:
		return 1
	case Eight:
		return 2
	case Nine:
		return 3
	case Queen:
		return 4
	case King:
		return 5
	case Ten:
		return 6
	case Ace:
		return 7
	default:
		return 0
	}
}

// BeatsInNull checks if this card beats another card in Null game
// In Null: no trumps, must follow suit, A > K > Q > J > 10 > 9 > 8 > 7
func (c Card) BeatsInNull(other Card, leadSuit Suit) bool {
	// Must be same suit to beat (or other is off-suit)
	if c.Suit == leadSuit && other.Suit == leadSuit {
		return c.NullRank() > other.NullRank()
	}
	// This card is lead suit, other is not
	if c.Suit == leadSuit {
		return true
	}
	// This card is not lead suit
	return false
}

// SortByNullRank sorts cards by Null game rank (A > K > Q > J > 10 > 9 > 8 > 7)
// Lower index = lower rank (7 is lowest)
func SortByNullRank(cards []Card) {
	for i := 0; i < len(cards); i++ {
		for j := i + 1; j < len(cards); j++ {
			if cards[i].NullRank() > cards[j].NullRank() {
				cards[i], cards[j] = cards[j], cards[i]
			}
		}
	}
}

type Cards []Card
type SkatCards [2]Card

// NewDeck creates a standard Skat deck (32 cards)
func NewDeck() Cards {
	deck := make([]Card, 0, 32)
	suits := []Suit{Clubs, Spades, Hearts, Diamonds}
	for _, suit := range suits {
		for rank := Seven; rank <= Ace; rank++ {
			deck = append(deck, Card{Suit: suit, Rank: rank})
		}
	}
	return deck
}

func (c Cards) Shuffle() {
	rand.Shuffle(len(c), func(i, j int) {
		c[i], c[j] = c[j], c[i]
	})
}

// GameValue calculates the game value for a hand given a mode and trump suit
// This is the value BEFORE playing - it's based on matadors only
func (c Cards) GameValue(mode GameMode, trumpSuit Suit) int {
	// Count matadors (consecutive jacks from club jack)
	jackSuits := make(map[Suit]bool)
	for _, card := range c {
		if card.Rank == Jack {
			jackSuits[card.Suit] = true
		}
	}

	// Calculate matadors (with or without)
	matadors := 0
	withJacks := jackSuits[Clubs] // "with" if we have club jack, "without" if not

	if withJacks {
		// "With" - count consecutive jacks from clubs
		if jackSuits[Clubs] {
			matadors++
			if jackSuits[Spades] {
				matadors++
				if jackSuits[Hearts] {
					matadors++
					if jackSuits[Diamonds] {
						matadors++
					}
				}
			}
		}
	} else {
		// "Without" - count how many top jacks are missing
		if !jackSuits[Clubs] {
			matadors++
			if !jackSuits[Spades] {
				matadors++
				if !jackSuits[Hearts] {
					matadors++
					if !jackSuits[Diamonds] {
						matadors++
					}
				}
			}
		}
	}

	// Get base value for the game type
	baseValue := 0
	switch mode {
	case ModeGrand:
		baseValue = 24
	case ModeSuit:
		switch trumpSuit {
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
		return 23 // Null games have fixed value
	}

	// Game value = base value × (matadors + 1 + game/schneider/schwarz bonuses)
	// For estimation purposes, we just use matadors + 1 (minimum multiplier)
	return baseValue * (matadors + 1)
}

// CountGamesPlayable counts how many games can be played given a certain game value
func (c Cards) CountGamesPlayable(gameValue int) int {
	count := 0

	// Check Grand
	if c.GameValue(ModeGrand, NoSuit) >= gameValue {
		count++
	}

	// Check each suit
	for _, suit := range []Suit{Diamonds, Hearts, Spades, Clubs} {
		if c.GameValue(ModeSuit, suit) >= gameValue {
			count++
		}
	}

	// Check Null (fixed value of 23)
	if 23 >= gameValue {
		count++
	}

	return count
}

func (c Cards) GetRemainingCards() Cards {
	allCards := Cards{}
	suits := []Suit{Clubs, Spades, Hearts, Diamonds}
	ranks := []Rank{Seven, Eight, Nine, Ten, Jack, Queen, King, Ace}

	for _, suit := range suits {
		for _, rank := range ranks {
			allCards = append(allCards, Card{Suit: suit, Rank: rank})
		}
	}

	handMap := make(map[Card]bool)
	for _, card := range c {
		handMap[card] = true
	}

	remaining := []Card{}
	for _, card := range allCards {
		if !handMap[card] {
			remaining = append(remaining, card)
		}
	}

	return remaining
}
