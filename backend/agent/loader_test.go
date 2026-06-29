package agent

import (
	"errors"
	"skat/game"
	"strings"
	"testing"
)

func TestRemoveAgentForPlayer(t *testing.T) {
	ClearAgentCache()
	t.Cleanup(ClearAgentCache)

	player := &game.PlayerState{ID: "transient-agent", IsAgent: true}
	want := NewHeuristicAgent(player.ID)
	SetAgentForPlayer(player, want)

	got, err := GetAgentForPlayer(player)
	if err != nil || got != want {
		t.Fatalf("expected cached agent before removal, got %p, %v", got, err)
	}
	RemoveAgentForPlayer(player)
	if _, err := GetAgentForPlayer(player); err == nil {
		t.Fatal("expected removed agent to require an unavailable config loader")
	}
}

func TestGetAgentForPlayerIDReturnsLoadErrorWithoutFallback(t *testing.T) {
	ClearAgentCache()
	SetAgentConfigLoader(func(profileID string) (*AgentConfigData, error) {
		return nil, errors.New("config unavailable")
	})
	t.Cleanup(func() {
		ClearAgentCache()
		SetAgentConfigLoader(nil)
	})

	loaded, err := GetAgentForPlayerID("broken-agent")

	if err == nil || !strings.Contains(err.Error(), "config unavailable") {
		t.Fatalf("expected config load error, got %v", err)
	}
	if loaded != nil {
		t.Fatalf("expected no fallback agent, got %#v", loaded)
	}
}

func TestBuildAgentFromConfigLoadsContractWeights(t *testing.T) {
	_, err := BuildAgentFromConfig(&AgentConfigData{
		ProfileID:           "neural-bidder",
		BiddingType:         "heuristic",
		GameChoiceType:      "heuristic",
		CardPlayType:        "heuristic",
		ContractWeightsPath: "/missing/contract.weights",
	})

	if err == nil || !strings.Contains(err.Error(), "failed to load contract weights") {
		t.Fatalf("expected contract weights load error, got %v", err)
	}
}
