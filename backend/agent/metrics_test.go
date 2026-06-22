package agent

import "testing"

func TestMergeMetricsIncludesHandStrengthOutcomes(t *testing.T) {
	agent := NewHeuristicAgent("test")
	agent.EnableMetrics()
	agent.MergeMetrics(AgentMetricsSnapshot{
		PredictedProbability: []float64{0.25, 0.75},
		ActualOutcomes:       []bool{true, false},
		OutcomeIsDeclarer:    []bool{true, false},
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
}
