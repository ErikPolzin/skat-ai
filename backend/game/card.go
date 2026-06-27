package game

import (
	"cmp"
	"math/rand"
	"slices"
)

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

type Cards []Card
type SkatCards [2]Card

// SortByValue sorts cards by point value, lowest first.
func (c Cards) SortByValue() {
	slices.SortStableFunc(c, func(a, b Card) int {
		return cmp.Compare(a.Value(), b.Value())
	})
}

// SortByNullRank sorts cards by Null game rank, lowest first.
func (c Cards) SortByNullRank() {
	slices.SortStableFunc(c, func(a, b Card) int {
		return cmp.Compare(a.NullRank(), b.NullRank())
	})
}

// TotalPoints returns the card-point total for the hand.
func (c Cards) TotalPoints() int {
	total := 0
	for _, card := range c {
		total += card.Value()
	}
	return total
}

// CountRank returns how many cards of the given rank are in the hand.
func (c Cards) CountRank(rank Rank) int {
	count := 0
	for _, card := range c {
		if card.Rank == rank {
			count++
		}
	}
	return count
}

// CountRanks returns how many cards match any of the given ranks.
func (c Cards) CountRanks(ranks ...Rank) int {
	selected := map[Rank]bool{}
	for _, rank := range ranks {
		selected[rank] = true
	}
	count := 0
	for _, card := range c {
		if selected[card.Rank] {
			count++
		}
	}
	return count
}

// CountTopJacks returns the number of consecutive top jacks held, starting at clubs.
func (c Cards) CountTopJacks() int {
	jacks := map[Suit]bool{}
	for _, card := range c {
		if card.Rank == Jack {
			jacks[card.Suit] = true
		}
	}
	count := 0
	for _, suit := range []Suit{Clubs, Spades, Hearts, Diamonds} {
		if !jacks[suit] {
			break
		}
		count++
	}
	return count
}

// CountAceTenPairs returns how many suits contain both ace and ten.
func (c Cards) CountAceTenPairs() int {
	count := 0
	for suit := Clubs; suit <= Diamonds; suit++ {
		hasAce, hasTen := false, false
		for _, card := range c {
			if card.Suit != suit {
				continue
			}
			hasAce = hasAce || card.Rank == Ace
			hasTen = hasTen || card.Rank == Ten
		}
		if hasAce && hasTen {
			count++
		}
	}
	return count
}

// MaxSuitLength returns the largest number of cards held in any natural suit.
func (c Cards) MaxSuitLength() int {
	counts := map[Suit]int{}
	maxCount := 0
	for _, card := range c {
		counts[card.Suit]++
		if counts[card.Suit] > maxCount {
			maxCount = counts[card.Suit]
		}
	}
	return maxCount
}

// IsTrump reports whether the card is trump in the given contract.
func (c Card) IsTrump(mode GameMode, trumpSuit Suit) bool {
	if mode == ModeNull {
		return false
	}
	if c.Rank == Jack {
		return true
	}
	return mode == ModeSuit && c.Suit == trumpSuit
}

// EffectiveSuit returns the suit used by the contract's follow-suit rules.
func (c Card) EffectiveSuit(mode GameMode, trumpSuit Suit) Suit {
	if mode != ModeNull && c.Rank == Jack {
		if mode == ModeGrand {
			return NoSuit
		}
		return trumpSuit
	}
	if mode == ModeSuit && c.Suit == trumpSuit {
		return trumpSuit
	}
	return c.Suit
}

// ContractClass maps side suits to 0..3 and trumps to 4.
func (c Card) ContractClass(mode GameMode, trumpSuit Suit) int {
	if c.IsTrump(mode, trumpSuit) {
		return 4
	}
	return int(c.Suit) - 1
}

// TrumpCount returns the number of trumps in the full deck for this mode.
func (m GameMode) TrumpCount() int {
	switch m {
	case ModeGrand, ModeRamsch:
		return 4
	case ModeSuit:
		return 11
	default:
		return 0
	}
}

// ContractTrumpCount returns how many cards are trump in the given contract.
func (c Cards) ContractTrumpCount(mode GameMode, trumpSuit Suit) int {
	count := 0
	for _, card := range c {
		if card.IsTrump(mode, trumpSuit) {
			count++
		}
	}
	return count
}

// ContractTrumpPoints returns the card-point total held in trumps.
func (c Cards) ContractTrumpPoints(mode GameMode, trumpSuit Suit) int {
	total := 0
	for _, card := range c {
		if card.IsTrump(mode, trumpSuit) {
			total += card.Value()
		}
	}
	return total
}

// TrumpValue returns the card's trump hierarchy value, or zero if it is not trump.
func (c Card) TrumpValue(mode GameMode, trumpSuit Suit) int {
	if mode != ModeNull && c.Rank == Jack {
		switch c.Suit {
		case Clubs:
			return 11
		case Spades:
			return 10
		case Hearts:
			return 9
		case Diamonds:
			return 8
		}
	}
	if mode == ModeSuit && c.Suit == trumpSuit && c.Rank != Jack {
		switch c.Rank {
		case Ace:
			return 7
		case Ten:
			return 6
		case King:
			return 5
		case Queen:
			return 4
		case Nine:
			return 3
		case Eight:
			return 2
		case Seven:
			return 1
		}
	}
	return 0
}

// Beats reports whether the card beats other under the contract rules.
func (c Card) Beats(other Card, mode GameMode, trumpSuit Suit) bool {
	if mode == ModeNull {
		if c.Suit != other.Suit {
			return false
		}
		return c.NullRank() > other.NullRank()
	}

	aValue := c.TrumpValue(mode, trumpSuit)
	bValue := other.TrumpValue(mode, trumpSuit)
	if aValue > 0 && bValue > 0 {
		return aValue > bValue
	}
	if aValue > 0 {
		return true
	}
	if bValue > 0 {
		return false
	}
	if c.Suit == other.Suit {
		return c.Rank.SkatRank() > other.Rank.SkatRank()
	}
	return false
}

// TrickWinner returns the player position that won a trick started by starter.
func (c Cards) TrickWinner(starter GamePosition, mode GameMode, trumpSuit Suit) GamePosition {
	if len(c) == 0 {
		return starter
	}

	winnerOffset := 0
	winningCard := c[0]
	for i := 1; i < len(c); i++ {
		if c[i].Beats(winningCard, mode, trumpSuit) {
			winnerOffset = i
			winningCard = c[i]
		}
	}
	return (starter + GamePosition(winnerOffset)) % 3
}

// HasTopTrumpControl reports whether the hand has a top jack or trump ace.
func (c Cards) HasTopTrumpControl(mode GameMode, trumpSuit Suit) bool {
	if mode == ModeNull {
		return false
	}
	for _, card := range c {
		if card.Rank == Jack && (card.Suit == Clubs || card.Suit == Spades) {
			return true
		}
		if mode == ModeSuit && card.Suit == trumpSuit && card.Rank == Ace {
			return true
		}
	}
	return false
}

// SideAceCount returns how many non-trump aces the hand contains.
func (c Cards) SideAceCount(mode GameMode, trumpSuit Suit) int {
	count := 0
	for _, card := range c {
		if card.Rank != Ace || card.IsTrump(mode, trumpSuit) {
			continue
		}
		count++
	}
	return count
}

// VoidSuitCount returns how many natural suits contain no non-trump cards.
func (c Cards) VoidSuitCount(mode GameMode, trumpSuit Suit) int {
	counts := c.NonTrumpSuitCounts(mode, trumpSuit)
	count := 0
	for suit := Clubs; suit <= Diamonds; suit++ {
		if counts[suit] == 0 {
			count++
		}
	}
	return count
}

// SingletonSuitCount returns how many natural suits contain one non-trump card.
func (c Cards) SingletonSuitCount(mode GameMode, trumpSuit Suit) int {
	counts := c.NonTrumpSuitCounts(mode, trumpSuit)
	count := 0
	for suit := Clubs; suit <= Diamonds; suit++ {
		if counts[suit] == 1 {
			count++
		}
	}
	return count
}

// NonTrumpSuitCounts counts natural suits after removing contract trumps.
func (c Cards) NonTrumpSuitCounts(mode GameMode, trumpSuit Suit) map[Suit]int {
	counts := map[Suit]int{}
	for _, card := range c {
		if card.IsTrump(mode, trumpSuit) {
			continue
		}
		counts[card.Suit]++
	}
	return counts
}

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
