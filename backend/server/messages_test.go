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

func TestDeclarerStatsCountsOnlyDeclarerResults(t *testing.T) {
	results := []game.PlayerResultState{
		{GameID: "game-1", PlayerID: "alice", IsDeclarer: true, IsWinner: true},
		{GameID: "game-1", PlayerID: "bob", IsWinner: false},
		{GameID: "game-2", PlayerID: "bob", IsDeclarer: true, IsWinner: false},
		{GameID: "game-2", PlayerID: "alice", IsWinner: true},
		{GameID: "game-3", PlayerID: "alice", IsDeclarer: true, IsWinner: false},
	}

	stats := declarerStats(results)

	if stats["alice"].GamesPlayed != 2 || stats["alice"].Wins != 1 || stats["alice"].Losses != 1 {
		t.Fatalf("expected alice to have 2 declarer games, 1 win, 1 loss; got %+v", stats["alice"])
	}
	if stats["bob"].GamesPlayed != 1 || stats["bob"].Wins != 0 || stats["bob"].Losses != 1 {
		t.Fatalf("expected bob to have 1 declarer loss; got %+v", stats["bob"])
	}
}
