package game

import "testing"

func newDeclarerChoiceStateForTest(bidValue int) *GameState {
	declarer := Listener
	return &GameState{
		Players: [3]*PlayerState{
			{ID: "dealer", Name: "Dealer"},
			{
				ID:   "declarer",
				Name: "Declarer",
				Hand: Cards{
					{Suit: Clubs, Rank: Jack},
					{Suit: Spades, Rank: Jack},
					{Suit: Clubs, Rank: Ace},
					{Suit: Clubs, Rank: Ten},
					{Suit: Clubs, Rank: King},
					{Suit: Hearts, Rank: Ace},
					{Suit: Hearts, Rank: Ten},
					{Suit: Diamonds, Rank: Ace},
					{Suit: Diamonds, Rank: Ten},
					{Suit: Spades, Rank: Ace},
				},
			},
			{ID: "speaker", Name: "Speaker"},
		},
		Skat:          SkatCards{{Suit: Hearts, Rank: Seven}, {Suit: Diamonds, Rank: Seven}},
		CurrentPlayer: Listener,
		Declarer:      &declarer,
		Phase:         PhaseDeclarerChoice,
		PlayedHand:    true,
		BidValue:      bidValue,
	}
}

func TestDeclareHandGameUsesHandMultiplierForBidValidation(t *testing.T) {
	gs := newDeclarerChoiceStateForTest(40)

	if _, err := gs.DeclareGame(ModeSuit, Clubs, false, false); err != nil {
		t.Fatalf("DeclareGame returned error: %v", err)
	}

	if gs.Overbid {
		t.Fatalf("expected hand game value to satisfy bid")
	}
	if gs.Phase != PhasePlaying {
		t.Fatalf("expected phase %s, got %s", PhasePlaying, gs.Phase)
	}
}

func TestDeclareHandGameUsesAnnouncementMultipliersForBidValidation(t *testing.T) {
	gs := newDeclarerChoiceStateForTest(60)

	if _, err := gs.DeclareGame(ModeSuit, Clubs, true, false); err != nil {
		t.Fatalf("DeclareGame returned error: %v", err)
	}

	if gs.Overbid {
		t.Fatalf("expected announced hand game value to satisfy bid")
	}
	if gs.Phase != PhasePlaying {
		t.Fatalf("expected phase %s, got %s", PhasePlaying, gs.Phase)
	}
}

func TestSuitGameMatadorsContinueThroughTrumpSuit(t *testing.T) {
	gs := newDeclarerChoiceStateForTest(84)
	gs.Players[*gs.Declarer].Hand = Cards{
		{Suit: Clubs, Rank: Jack},
		{Suit: Spades, Rank: Jack},
		{Suit: Hearts, Rank: Jack},
		{Suit: Diamonds, Rank: Jack},
		{Suit: Clubs, Rank: Ace},
		{Suit: Clubs, Rank: Ten},
		{Suit: Hearts, Rank: Ace},
		{Suit: Diamonds, Rank: Ace},
		{Suit: Spades, Rank: Ace},
		{Suit: Spades, Rank: Ten},
	}

	if _, err := gs.DeclareGame(ModeSuit, Clubs, false, false); err != nil {
		t.Fatalf("DeclareGame returned error: %v", err)
	}

	if gs.Matadors != 6 {
		t.Fatalf("expected 6 matadors, got %d", gs.Matadors)
	}
	if gs.Overbid {
		t.Fatalf("expected extended suit matadors to satisfy bid")
	}
}

func TestDeclareNullHandUsesNullHandValueForBidValidation(t *testing.T) {
	gs := newDeclarerChoiceStateForTest(35)

	if _, err := gs.DeclareGame(ModeNull, NoSuit, false, false); err != nil {
		t.Fatalf("DeclareGame returned error: %v", err)
	}

	if gs.Overbid {
		t.Fatalf("expected null hand value to satisfy bid")
	}
	if gs.Phase != PhasePlaying {
		t.Fatalf("expected phase %s, got %s", PhasePlaying, gs.Phase)
	}
}

func TestNullHandResultUsesNullHandValue(t *testing.T) {
	gs := newDeclarerChoiceStateForTest(0)
	gs.Mode = ModeNull
	gs.PlayedHand = true
	gs.DeclarerScore = 0

	result := gs.Result()

	if result.BaseValue != 35 {
		t.Fatalf("expected base value 35, got %d", result.BaseValue)
	}
	if result.Value != 35 {
		t.Fatalf("expected value 35, got %d", result.Value)
	}
}
