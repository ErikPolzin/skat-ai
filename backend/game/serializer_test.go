package game

import "testing"

func TestSerializeForPlayerAllowsNextGameForUnlimitedSession(t *testing.T) {
	gs := NewGame()
	gs.MaxGames = 0
	gs.Phase = PhaseComplete
	gs.GameNumber = 42
	gs.Players = [3]*PlayerState{
		{ID: "alice", Name: "Alice"},
		{ID: "bob", Name: "Bob"},
		{ID: "cara", Name: "Cara"},
	}

	info := gs.SerializeForPlayer("alice")

	if !info.CanPlayNext {
		t.Fatalf("expected unlimited session to allow another game")
	}
	if info.State.MaxGames != 0 {
		t.Fatalf("expected max_games to remain 0, got %d", info.State.MaxGames)
	}
}
