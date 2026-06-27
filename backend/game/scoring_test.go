package game

import "testing"

func TestOverbidNullGameAlwaysDeductsDoubleBid(t *testing.T) {
	gs := newDeclarerChoiceStateForTest(46)
	gs.Mode = ModeNull
	gs.Overbid = true
	gs.Phase = PhaseComplete
	gs.PlayerScores[*gs.Declarer] = 0

	result := gs.Result()

	if result.DeclarerWon {
		t.Fatal("expected overbid declarer to lose despite taking no tricks")
	}
	if result.Value != -92 {
		t.Fatalf("expected overbid value -92, got %d", result.Value)
	}
	if points := gs.CalculatePlayerPoints(*gs.Declarer); points != -92 {
		t.Fatalf("expected declarer player points -92, got %d", points)
	}
}

func TestOverbidSuitGameAlwaysDeductsDoubleBid(t *testing.T) {
	gs := newDeclarerChoiceStateForTest(50)
	gs.Mode = ModeSuit
	gs.TrumpSuit = Clubs
	gs.Matadors = 1
	gs.Overbid = true
	gs.Phase = PhaseComplete
	gs.PlayerScores[*gs.Declarer] = 61

	result := gs.Result()

	if result.DeclarerWon {
		t.Fatal("expected overbid declarer to lose despite reaching 61 card points")
	}
	if result.Value != -100 {
		t.Fatalf("expected overbid value -100, got %d", result.Value)
	}
}

func TestRamschPlayerResultsDeductDoubleCardPoints(t *testing.T) {
	gs := &GameState{
		ID:        "game",
		SessionID: "session",
		Mode:      ModeRamsch,
		Phase:     PhaseComplete,
		Players: [3]*PlayerState{
			{ID: "dealer", Name: "Dealer"},
			{ID: "listener", Name: "Listener"},
			{ID: "speaker", Name: "Speaker"},
		},
		PlayerScores: [3]int{10, 40, 70},
	}

	results := gs.PlayerResults()
	if results == nil {
		t.Fatal("expected Ramsch player results")
	}

	wantPoints := [3]int{-20, -80, -140}
	for pos, want := range wantPoints {
		if got := results[pos].PlayerPoints; got != want {
			t.Errorf("position %d player points = %d, want %d", pos, got, want)
		}
	}
	if !results[Dealer].IsWinner {
		t.Error("expected player with the lowest Ramsch card score to win")
	}
}
