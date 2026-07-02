package agent

import "testing"

func TestMergeMetricsIncludesHandStrengthOutcomes(t *testing.T) {
	agent := NewHeuristicAgent("test")
	agent.EnableMetrics()
	agent.MergeMetrics(AgentMetricsSnapshot{
		PredictedProbability:    []float64{0.25, 0.75},
		ActualOutcomes:          []bool{true, false},
		OutcomeIsDeclarer:       []bool{true, false},
		HandGames:               3,
		HandWins:                2,
		SchneiderGames:          2,
		SchneiderWins:           1,
		SchwarzGames:            1,
		SchwarzWins:             1,
		SchneiderAnnouncedGames: 2,
		SchneiderAnnouncedWins:  1,
		SchwarzAnnouncedGames:   1,
		SchwarzAnnouncedWins:    0,
	})

	metrics := agent.GetMetrics()
	if len(metrics.PredictedProbability) != 2 || len(metrics.ActualOutcomes) != 2 || len(metrics.OutcomeIsDeclarer) != 2 {
		t.Fatalf("hand-strength outcomes were not merged: probabilities=%v outcomes=%v roles=%v",
			metrics.PredictedProbability, metrics.ActualOutcomes, metrics.OutcomeIsDeclarer)
	}
	if metrics.PredictedProbability[0] != 0.25 || !metrics.ActualOutcomes[0] {
		t.Fatalf("merged hand-strength outcome changed: probabilities=%v outcomes=%v",
			metrics.PredictedProbability, metrics.ActualOutcomes)
	}
	if metrics.HandGames != 3 || metrics.HandWins != 2 || metrics.SchneiderGames != 2 || metrics.SchneiderWins != 1 || metrics.SchwarzGames != 1 || metrics.SchwarzWins != 1 || metrics.SchneiderAnnouncedGames != 2 || metrics.SchneiderAnnouncedWins != 1 || metrics.SchwarzAnnouncedGames != 1 || metrics.SchwarzAnnouncedWins != 0 {
		t.Fatalf("declaration metrics were not merged: %+v", metrics)
	}
}
