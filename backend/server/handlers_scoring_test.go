package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"

	"skat/game"
	"skat/server/db"
)

func TestChooseGameSavesImmediateOverbidLoss(t *testing.T) {
	database := newTestDatabase()
	server := NewServer(database)
	gs := game.NewGame()
	declarer := game.Listener
	gs.Phase = game.PhaseDeclarerChoice
	gs.CurrentPlayer = declarer
	gs.Declarer = &declarer
	gs.BidValue = 46
	gs.Players = [3]*game.PlayerState{
		{ID: "dealer", Name: "Dealer"},
		{ID: "declarer", Name: "Declarer"},
		{ID: "speaker", Name: "Speaker"},
	}
	if err := server.cache.SaveGame(gs); err != nil {
		t.Fatalf("failed to seed game: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/games/"+gs.ID+"/choose-game",
		strings.NewReader(`{"mode":"null","trump":""}`),
	)
	req = mux.SetURLVars(req, map[string]string{"id": gs.ID})
	req = req.WithContext(context.WithValue(req.Context(), profileContextKey{}, &db.ProfileEntry{
		ID:   "declarer",
		Name: "Declarer",
	}))
	rec := httptest.NewRecorder()

	server.handleChooseGame(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(database.playerResults) != 3 {
		t.Fatalf("expected three saved player results, got %d", len(database.playerResults))
	}
	foundDeclarer := false
	for _, result := range database.playerResults {
		if result.PlayerID == "declarer" && result.PlayerPoints != -92 {
			t.Fatalf("expected overbid declarer points -92, got %d", result.PlayerPoints)
		}
		if result.PlayerID == "declarer" {
			foundDeclarer = true
		}
	}
	if !foundDeclarer {
		t.Fatal("expected a saved result for the declarer")
	}
}
