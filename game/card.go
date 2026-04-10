package game

import "fmt"

// Suit represents a card suit
type Suit int

const (
	Clubs Suit = iota
	Spades
	Hearts
	Diamonds
)

func (s Suit) String() string {
	return [...]string{"♣", "♠", "♥", "♦"}[s]
}

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

func (r Rank) String() string {
	return [...]string{"7", "8", "9", "10", "J", "Q", "K", "A"}[r]
}

// Card represents a playing card
type Card struct {
	Suit Suit
	Rank Rank
}

func (c Card) String() string {
	return fmt.Sprintf("%s%s", c.Rank, c.Suit)
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

// NewDeck creates a standard Skat deck (32 cards)
func NewDeck() []Card {
	deck := make([]Card, 0, 32)
	for suit := Clubs; suit <= Diamonds; suit++ {
		for rank := Seven; rank <= Ace; rank++ {
			deck = append(deck, Card{Suit: suit, Rank: rank})
		}
	}
	return deck
}
