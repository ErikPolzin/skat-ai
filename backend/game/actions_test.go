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
	gs.PlayerScores[*gs.Declarer] = 0

	result := gs.Result()

	if result.BaseValue != 35 {
		t.Fatalf("expected base value 35, got %d", result.BaseValue)
	}
	if result.Value != 35 {
		t.Fatalf("expected value 35, got %d", result.Value)
	}
}

func TestForfeitingDeclarerResultUsesLostGameValue(t *testing.T) {
	gs := newDeclarerChoiceStateForTest(0)
	if _, err := gs.DeclareGame(ModeSuit, Clubs, false, false); err != nil {
		t.Fatalf("DeclareGame returned error: %v", err)
	}
	gs.Phase = PhaseComplete
	gs.ForfeitedPlayer = gs.Declarer

	result := gs.Result()

	if !result.IsForfeit {
		t.Fatalf("expected forfeit result")
	}
	if result.DeclarerWon {
		t.Fatalf("expected declarer to lose after forfeiting")
	}
	if result.Value != -96 {
		t.Fatalf("expected forfeited clubs game value -96, got %d", result.Value)
	}
	if points := gs.CalculatePlayerPoints(*gs.Declarer); points != -96 {
		t.Fatalf("expected declarer player points -96, got %d", points)
	}
}

func TestForfeitingDefenderResultUsesDeclarerWinValue(t *testing.T) {
	gs := newDeclarerChoiceStateForTest(0)
	if _, err := gs.DeclareGame(ModeSuit, Clubs, false, false); err != nil {
		t.Fatalf("DeclareGame returned error: %v", err)
	}
	forfeited := Speaker
	gs.Phase = PhaseComplete
	gs.ForfeitedPlayer = &forfeited

	result := gs.Result()

	if !result.IsForfeit {
		t.Fatalf("expected forfeit result")
	}
	if !result.DeclarerWon {
		t.Fatalf("expected declarer to win when defender forfeits")
	}
	if result.Value != 48 {
		t.Fatalf("expected forfeited clubs game value 48, got %d", result.Value)
	}
	if points := gs.CalculatePlayerPoints(*gs.Declarer); points != 48 {
		t.Fatalf("expected declarer player points 48, got %d", points)
	}
	if points := gs.CalculatePlayerPoints(forfeited); points != 0 {
		t.Fatalf("expected forfeiting defender player points 0, got %d", points)
	}
}

func TestTimerCanBeDisabled(t *testing.T) {
	gs := NewGame()
	gs.Phase = PhaseBidding
	gs.TimerEnabled = false
	gs.Players[gs.CurrentPlayer] = &PlayerState{ID: "dealer", Name: "Dealer"}

	gs.UpdateCurrentPlayerDeadline()

	if gs.CurrentPlayerDeadline != "" {
		t.Fatalf("expected no deadline when timer is disabled, got %q", gs.CurrentPlayerDeadline)
	}
}

func TestTimeoutEndsGameWithoutAddingPlayerResults(t *testing.T) {
	gs := NewGame()
	gs.Phase = PhasePlaying
	gs.GameNumber = 29
	gs.CurrentPlayer = Listener
	gs.Players = [3]*PlayerState{
		{ID: "dealer", Name: "Dealer"},
		{ID: "listener", Name: "Listener"},
		{ID: "speaker", Name: "Speaker"},
	}

	results := gs.ForfeitDueToInactivity()

	if results != nil {
		t.Fatalf("expected no synthetic timeout results, got %v", results)
	}
	if gs.Phase != PhaseComplete {
		t.Fatalf("expected timeout to complete game, got %s", gs.Phase)
	}
	if gs.ForfeitedPlayer == nil || *gs.ForfeitedPlayer != Listener {
		t.Fatalf("expected listener to be marked as forfeited, got %v", gs.ForfeitedPlayer)
	}
	if gs.PlayerResults() != nil {
		t.Fatalf("expected forfeited game not to produce player results")
	}
}
