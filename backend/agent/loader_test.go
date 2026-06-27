package agent

import (
	"errors"
	"strings"
	"testing"
)

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
