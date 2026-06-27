package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"

	"skat/agent"
	"skat/game"
	"skat/server/db"
)

func TestAddAgentFailsWhenAgentConfigCannotBeLoaded(t *testing.T) {
	agent.ClearAgentCache()
	t.Cleanup(agent.ClearAgentCache)

	database := newTestDatabase()
	database.profiles["broken-agent"] = &db.ProfileEntry{
		ID:      "broken-agent",
		Name:    "Broken Agent",
		IsAgent: true,
	}
	gs := game.NewGame()
	database.games[gs.ID] = gs.Clone()

	server := NewServer(database)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/games/"+gs.ID+"/agents",
		strings.NewReader(`{"agent_id":"broken-agent"}`),
	)
	req = mux.SetURLVars(req, map[string]string{"id": gs.ID})
	rec := httptest.NewRecorder()

	server.handleAddAgent(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "failed to load agent config") {
		t.Fatalf("expected load error in response, got %q", rec.Body.String())
	}
	saved, err := server.cache.GetGameByID(gs.ID)
	if err != nil {
		t.Fatalf("failed to reload game: %v", err)
	}
	if saved.PlayerCount() != 0 {
		t.Fatalf("expected failed agent not to be added, got %d players", saved.PlayerCount())
	}
}
