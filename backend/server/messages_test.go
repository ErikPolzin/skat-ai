package server

import (
	"testing"

	"skat/game"
)

func TestAggregateSessionResultsMarksForfeitAsLossWithoutSyntheticPoints(t *testing.T) {
	forfeited := game.Dealer
	gs := &game.GameState{
		SessionID:       "session",
		ForfeitedPlayer: &forfeited,
		Players: [3]*game.PlayerState{
			{ID: "leader", Name: "Leader"},
			{ID: "second", Name: "Second"},
			{ID: "third", Name: "Third"},
		},
	}
	gameResults := []game.PlayerResultState{
		{GameID: "game-1", SessionID: "session", PlayerID: "leader", PlayerPoints: 200},
		{GameID: "game-1", SessionID: "session", PlayerID: "second", PlayerPoints: 100},
		{GameID: "game-1", SessionID: "session", PlayerID: "third", PlayerPoints: 50},
	}

	results := aggregateSessionResults(gs, gameResults)

	if len(results) != 3 {
		t.Fatalf("expected 3 session results, got %d", len(results))
	}
	byPlayer := make(map[string]game.PlayerSessionResultState)
	for _, result := range results {
		byPlayer[result.PlayerID] = result
	}

	if byPlayer["leader"].PlayerPoints != 200 {
		t.Fatalf("expected forfeiting player to keep completed-game total 200, got %d", byPlayer["leader"].PlayerPoints)
	}
	if !byPlayer["leader"].IsForfeit || byPlayer["leader"].IsWinner {
		t.Fatalf("expected forfeiting player to be marked as a non-winning forfeit")
	}
	if !byPlayer["second"].IsWinner {
		t.Fatalf("expected highest non-forfeiting player to win")
	}
}
