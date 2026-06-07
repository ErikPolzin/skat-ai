package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"

	"skat/game"
	"skat/server/db"
)

type testDatabase struct {
	profiles       map[string]*db.ProfileEntry
	games          map[string]*game.GameState
	sessions       map[string]*game.GameSessionState
	playerResults  []game.PlayerResultState
	sessionResults []game.PlayerSessionResultState
	ratings        map[string]*db.PlayerRating
}

func newTestDatabase() *testDatabase {
	return &testDatabase{
		profiles: make(map[string]*db.ProfileEntry),
		games:    make(map[string]*game.GameState),
		sessions: make(map[string]*game.GameSessionState),
		ratings:  make(map[string]*db.PlayerRating),
	}
}

func (d *testDatabase) Close() error      { return nil }
func (d *testDatabase) InitSchema() error { return nil }
func (d *testDatabase) GetProfile(profileID string) (*db.ProfileEntry, error) {
	profile, ok := d.profiles[profileID]
	if !ok {
		return nil, fmt.Errorf("profile not found")
	}
	return profile, nil
}
func (d *testDatabase) GetProfileByName(name string) (*db.ProfileEntry, error) {
	for _, profile := range d.profiles {
		if profile.Name == name {
			return profile, nil
		}
	}
	return nil, fmt.Errorf("profile not found")
}
func (d *testDatabase) SaveProfile(profile db.ProfileEntry) error {
	d.profiles[profile.ID] = &profile
	return nil
}
func (d *testDatabase) SaveGameSession(session game.GameSessionState) error {
	d.sessions[session.ID] = &session
	return nil
}
func (d *testDatabase) GetGameSession(sessionID string) (*game.GameSessionState, error) {
	session, ok := d.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("game session not found")
	}
	return session, nil
}
func (d *testDatabase) GetGameByID(gameID string) (*game.GameState, error) {
	gs, ok := d.games[gameID]
	if !ok {
		return nil, fmt.Errorf("game not found")
	}
	return gs, nil
}
func (d *testDatabase) GetGameBySessionCode(sessionCode string) (*game.GameState, error) {
	for _, gs := range d.games {
		if string(gs.Code) == sessionCode {
			return gs, nil
		}
	}
	return nil, fmt.Errorf("game not found")
}
func (d *testDatabase) SaveGame(gs game.GameState) error {
	copy := gs
	d.games[gs.ID] = &copy
	d.games[gs.SessionID] = &copy
	if session, ok := d.sessions[gs.SessionID]; ok {
		session.GameID = gs.ID
	}
	return nil
}
func (d *testDatabase) DeleteGame(gameID string) error {
	delete(d.games, gameID)
	return nil
}
func (d *testDatabase) RemovePlayer(gameID, playerID string) error         { return nil }
func (d *testDatabase) ListOpenSessions() ([]game.GameSessionState, error) { return nil, nil }
func (d *testDatabase) ListPlayers(gameID string) ([3]*game.PlayerState, error) {
	gs, err := d.GetGameByID(gameID)
	if err != nil {
		return [3]*game.PlayerState{}, err
	}
	return gs.Players, nil
}
func (d *testDatabase) SavePlayerResults(results []game.PlayerResultState) error {
	d.playerResults = append(d.playerResults, results...)
	return nil
}
func (d *testDatabase) SavePlayerSessionResults(results []game.PlayerSessionResultState) error {
	d.sessionResults = append([]game.PlayerSessionResultState(nil), results...)
	return nil
}
func (d *testDatabase) GetPlayerSessionResults(playerID string, limit int) ([]game.PlayerSessionResultState, error) {
	return nil, nil
}
func (d *testDatabase) GetSessionPlayerResults(sessionID string) ([]game.PlayerSessionResultState, error) {
	results := make([]game.PlayerSessionResultState, 0)
	for _, result := range d.sessionResults {
		if result.SessionID == sessionID {
			results = append(results, result)
		}
	}
	return results, nil
}
func (d *testDatabase) CountGamesInSession(sessionID string) (int, error) { return 0, nil }
func (d *testDatabase) GetPlayerResultsForSession(sessionID string) ([]game.PlayerResultState, error) {
	results := make([]game.PlayerResultState, 0)
	for _, result := range d.playerResults {
		if result.SessionID == sessionID {
			results = append(results, result)
		}
	}
	return results, nil
}
func (d *testDatabase) GetFormattedSessionResults(sessionID string) ([]game.SessionGameResult, error) {
	return nil, nil
}
func (d *testDatabase) ListAgentProfiles() ([]db.ProfileEntry, error) { return nil, nil }
func (d *testDatabase) CleanupStaleGames(inactiveMinutes int, onlinePlayerIDs []string) (int, error) {
	return 0, nil
}
func (d *testDatabase) GetActiveGamesByPlayer(playerID string) ([]game.GameState, error) {
	games := make([]game.GameState, 0)
	seen := make(map[string]bool)
	for _, gs := range d.games {
		if seen[gs.ID] || gs.Phase == game.PhaseComplete {
			continue
		}
		for _, player := range gs.Players {
			if player != nil && player.ID == playerID {
				games = append(games, *gs)
				seen[gs.ID] = true
				break
			}
		}
	}
	return games, nil
}
func (d *testDatabase) GetSpectatableGames(excludePlayerID string) ([]game.GameState, error) {
	return nil, nil
}
func (d *testDatabase) GetAllExpiredGames() ([]game.GameState, error) { return nil, nil }
func (d *testDatabase) GetPlayerRating(profileID string) (*db.PlayerRating, error) {
	if rating, ok := d.ratings[profileID]; ok {
		return rating, nil
	}
	rating := &db.PlayerRating{ProfileID: profileID, Rating: 1000}
	d.ratings[profileID] = rating
	return rating, nil
}
func (d *testDatabase) SavePlayerRating(rating db.PlayerRating) error {
	d.ratings[rating.ProfileID] = &rating
	return nil
}
func (d *testDatabase) GetLeaderboard(limit int) ([]db.PlayerRating, error) { return nil, nil }
func (d *testDatabase) GetAgentConfig(profileID string) (*db.AgentConfig, error) {
	return nil, fmt.Errorf("agent config not found")
}
func (d *testDatabase) SaveAgentConfig(config db.AgentConfig) error { return nil }
func (d *testDatabase) DeleteAgentConfig(profileID string) error    { return nil }

func waitForSavedGame(t *testing.T, database db.Database, gameID string, condition func(*game.GameState) bool) *game.GameState {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	var saved *game.GameState
	var err error
	for time.Now().Before(deadline) {
		saved, err = database.GetGameByID(gameID)
		if err == nil && condition(saved) {
			return saved
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("failed to reload game: %v", err)
	}
	t.Fatalf("saved game did not reach expected state: %+v", saved)
	return nil
}

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
	database := newTestDatabase()
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
	if err := server.cache.SaveGame(*gs); err != nil {
		t.Fatalf("failed to seed cache: %v", err)
	}
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
	saved := waitForSavedGame(t, database, gs.SessionID, func(saved *game.GameState) bool {
		return saved.Phase == game.PhaseComplete
	})
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
	database := newTestDatabase()
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
	if err := server.cache.SaveGame(*gs); err != nil {
		t.Fatalf("failed to seed cache: %v", err)
	}
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
	saved := waitForSavedGame(t, database, gs.SessionID, func(saved *game.GameState) bool {
		return saved.ForfeitedPlayer != nil
	})
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
