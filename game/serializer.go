package game

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

var SUIT_STRINGS = [...]string{"♣", "♠", "♥", "♦"}

func (s Suit) String() string {
	return SUIT_STRINGS[s]
}

func ParseSuit(s string) (Suit, error) {
	idx := slices.Index(SUIT_STRINGS[:], s)
	if idx == -1 {
		return Clubs, fmt.Errorf("Unknown suit %s", s)
	}
	return Suit(idx), nil
}

var RANK_STRINGS = [...]string{"7", "8", "9", "10", "J", "Q", "K", "A"}

func (r Rank) String() string {
	return RANK_STRINGS[r]
}

func ParseRank(s string) (Rank, error) {
	idx := slices.Index(RANK_STRINGS[:], s)
	if idx == -1 {
		return Seven, fmt.Errorf("Unknown rank %s", s)
	}
	return Rank(idx), nil
}

func (c Card) String() string {
	return fmt.Sprintf("%s.%s", c.Rank, c.Suit)
}

func ParseCard(s string) (Card, error) {
	split := strings.Split(s, ".")
	rank, err := ParseRank(split[0])
	if err != nil {
		return Card{}, err
	}
	suit, err := ParseSuit(split[1])
	if err != nil {
		return Card{}, err
	}
	return Card{
		Rank: rank,
		Suit: suit,
	}, nil
}

func (cs *Cards) String() string {
	cardsStrings := []string{}
	for _, c := range *cs {
		cardsStrings = append(cardsStrings, c.String())
	}
	return strings.Join(cardsStrings, "-")
}

func ParseCards(s string) (Cards, error) {
	if len(s) == 0 {
		return nil, nil
	}
	split := strings.Split(s, "-")
	cards := Cards{}
	for _, s := range split {
		card, err := ParseCard(s)
		if err != nil {
			return nil, err
		}
		cards = append(cards, card)
	}
	return cards, nil
}

func (cs *SkatCards) String() string {
	cardsStrings := []string{}
	for _, c := range *cs {
		cardsStrings = append(cardsStrings, c.String())
	}
	return strings.Join(cardsStrings, "-")
}

func ParseSkatCards(s string) (SkatCards, error) {
	skatCards, err := ParseCards(s)
	if err != nil {
		return SkatCards{}, fmt.Errorf("cannot parse skat cards: %s", s)
	}
	if len(skatCards) >= 2 {
		return SkatCards{skatCards[0], skatCards[1]}, nil
	} else {
		return SkatCards{}, fmt.Errorf("incorrect number of skat cards: %d", len(skatCards))
	}
}

// GameInfo represents game state information for the API
type GameInfo struct {
	State       *GameState `json:"state"`
	PlayerID    string     `json:"player_id,omitempty"`
	Hand        []Card     `json:"hand,omitempty"`
	Skat        [2]Card    `json:"skat,omitempty"`
	CanPlayNext bool       `json:"can_play_next"`
}

func (c *Rank) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.String())
}

func (c *Rank) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	rank, err := ParseRank(s)
	if err != nil {
		return err
	}
	c = &rank
	return nil
}

func (c *Suit) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.String())
}

func (c *Suit) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	suit, err := ParseSuit(s)
	if err != nil {
		return err
	}
	c = &suit
	return nil
}

// ToGameInfo serializes the game state for a specific player
// playerID can be empty to get a public view without player-specific data
func (gs *GameState) SerializeForPlayer(playerID string) *GameInfo {
	// Update card counts for all players before serializing
	for _, player := range gs.Players {
		if player != nil {
			player.CardCount = len(player.Hand)
		}
	}

	// Determine if players can start the next game (max 10 games per session)
	canPlayNext := gs.Phase == PhaseComplete && gs.GameNumber < 10

	info := &GameInfo{
		State:       gs,
		PlayerID:    playerID,
		CanPlayNext: canPlayNext,
	}

	player := gs.GetPlayerById(playerID)
	position := gs.GetPositionForPlayer(playerID)

	if player != nil {
		info.Hand = player.Hand
		if position == gs.Declarer {
			info.Skat = gs.Skat
		}
	}

	return info
}
