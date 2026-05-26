package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"

	"skat/game"
	"skat/server/db"
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

func TestLeaveInProgressGamePersistsCompleteForfeit(t *testing.T) {
	database := db.NewMemoryDatabase()
	server := NewServer(database)
	gs := game.NewGame()
	gs.Phase = game.PhasePlaying
	gs.Players = [3]*game.PlayerState{
		{ID: "alice", Name: "Alice"},
		{ID: "bob", Name: "Bob"},
		{ID: "cara", Name: "Cara"},
	}
	if err := database.SaveGameSession(game.GameSessionState{
		ID:           gs.SessionID,
		Code:         string(gs.Code),
		GameID:       gs.ID,
		PlayerCount:  gs.PlayerCount(),
		MaxGames:     gs.MaxGames,
		PassPolicy:   string(gs.PassPolicy),
		TimerEnabled: gs.TimerEnabled,
	}); err != nil {
		t.Fatalf("failed to save session: %v", err)
	}
	if err := database.SaveGame(*gs); err != nil {
		t.Fatalf("failed to save game: %v", err)
	}
	if err := database.SavePlayerResults([]game.PlayerResultState{
		{GameID: gs.ID, SessionID: gs.SessionID, PlayerID: "alice", PlayerPoints: 50},
		{GameID: gs.ID, SessionID: gs.SessionID, PlayerID: "bob", PlayerPoints: 40},
		{GameID: gs.ID, SessionID: gs.SessionID, PlayerID: "cara", PlayerPoints: 30},
	}); err != nil {
		t.Fatalf("failed to save player results: %v", err)
	}
	server.cache.games[gs.ID] = gs

	req := httptest.NewRequest(http.MethodPost, "/api/games/"+gs.ID+"/leave", nil)
	req = mux.SetURLVars(req, map[string]string{"id": gs.ID})
	req = req.WithContext(context.WithValue(req.Context(), profileContextKey{}, &db.ProfileEntry{
		ID:   "alice",
		Name: "Alice",
	}))
	rec := httptest.NewRecorder()

	server.handleLeaveGame(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	saved, err := database.GetGameByID(gs.SessionID)
	if err != nil {
		t.Fatalf("failed to reload game: %v", err)
	}
	if saved.Phase != game.PhaseComplete {
		t.Fatalf("expected saved game phase complete, got %s", saved.Phase)
	}
	if saved.ForfeitedPlayer == nil || *saved.ForfeitedPlayer != game.Dealer {
		t.Fatalf("expected dealer to be persisted as forfeited, got %v", saved.ForfeitedPlayer)
	}
	activeGames, err := database.GetActiveGamesByPlayer("alice")
	if err != nil {
		t.Fatalf("failed to get active games: %v", err)
	}
	if len(activeGames) != 0 {
		t.Fatalf("expected no active games after leaving, got %d", len(activeGames))
	}
}

func TestStrictTournamentLeaveAfterCompletedGameCountsAsForfeit(t *testing.T) {
	database := db.NewMemoryDatabase()
	server := NewServer(database)
	gs := game.NewGame()
	gs.Phase = game.PhaseComplete
	gs.GameNumber = 0
	gs.MaxGames = 3
	gs.CompletionPolicy = game.CompletionPolicyStrict
	gs.Players = [3]*game.PlayerState{
		{ID: "alice", Name: "Alice"},
		{ID: "bob", Name: "Bob"},
		{ID: "cara", Name: "Cara"},
	}
	if err := database.SaveGameSession(game.GameSessionState{
		ID:               gs.SessionID,
		Code:             string(gs.Code),
		GameID:           gs.ID,
		PlayerCount:      gs.PlayerCount(),
		MaxGames:         gs.MaxGames,
		PassPolicy:       string(gs.PassPolicy),
		TimerEnabled:     gs.TimerEnabled,
		CompletionPolicy: string(gs.CompletionPolicy),
	}); err != nil {
		t.Fatalf("failed to save session: %v", err)
	}
	if err := database.SaveGame(*gs); err != nil {
		t.Fatalf("failed to save game: %v", err)
	}
	server.cache.games[gs.ID] = gs

	req := httptest.NewRequest(http.MethodPost, "/api/games/"+gs.ID+"/leave", nil)
	req = mux.SetURLVars(req, map[string]string{"id": gs.ID})
	req = req.WithContext(context.WithValue(req.Context(), profileContextKey{}, &db.ProfileEntry{
		ID:   "alice",
		Name: "Alice",
	}))
	rec := httptest.NewRecorder()

	server.handleLeaveGame(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	saved, err := database.GetGameByID(gs.SessionID)
	if err != nil {
		t.Fatalf("failed to reload game: %v", err)
	}
	if saved.ForfeitedPlayer == nil || *saved.ForfeitedPlayer != game.Dealer {
		t.Fatalf("expected dealer to be persisted as forfeited, got %v", saved.ForfeitedPlayer)
	}
	sessionResults, err := database.GetSessionPlayerResults(gs.SessionID)
	if err != nil {
		t.Fatalf("failed to load session results: %v", err)
	}
	if len(sessionResults) != 3 {
		t.Fatalf("expected finalized session results, got %d", len(sessionResults))
	}
}
