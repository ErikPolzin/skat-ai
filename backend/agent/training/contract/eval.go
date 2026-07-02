package contract

import (
	"fmt"
	"io"
	"math"
	"sort"
	"strings"

	"skat/agent"
	"skat/agent/strategies"
	"skat/game"
)

type EvalConfig struct {
	Games            int
	Selection        string
	BiddingThreshold float64
	BinWidth         float64
}

type Observation struct {
	Mode               game.GameMode
	Predicted          float64
	Actual             bool
	PlayedHand         bool
	AnnouncedSchneider bool
	AnnouncedSchwarz   bool
	IsSchneider        bool
	IsSchwarz          bool
}

type BucketStats struct {
	Low          float64
	High         float64
	Count        int
	Wins         int
	AvgPredicted float64
	ActualRate   float64
}

type ModeSummary struct {
	Label        string
	Games        int
	ActualRate   float64
	ExpectedRate float64
	Delta        float64
}

type EvalResult struct {
	Observations  []Observation
	Overall       ModeSummary
	ByMode        []ModeSummary
	ByDeclaration []ModeSummary
	ByOutcome     []ModeSummary
	Buckets       []BucketStats
	BrierScore    float64
	LogLoss       float64
}

func EvaluateEstimator(config EvalConfig, estimator strategies.ContractWinProbabilityEstimator) (EvalResult, error) {
	if estimator == nil {
		return EvalResult{}, fmt.Errorf("contract estimator is nil")
	}
	if config.Games <= 0 {
		return EvalResult{}, fmt.Errorf("eval games must be positive")
	}
	if config.Selection == "" {
		config.Selection = "heuristic"
	}
	if config.BiddingThreshold == 0 {
		config.BiddingThreshold = strategies.DefaultContractEvaluatorConfig().MinWinProbability
	}
	if config.BinWidth <= 0 {
		config.BinWidth = 0.1
	}

	observations, err := collectObservations(config.Games, config.Selection, config.BiddingThreshold, estimator)
	if err != nil {
		return EvalResult{}, err
	}
	return summarizeObservations(observations, config.BinWidth), nil
}

func collectObservations(targetGames int, selection string, biddingThreshold float64, estimator strategies.ContractWinProbabilityEstimator) ([]Observation, error) {
	var observations []Observation
	attempts := 0
	for len(observations) < targetGames {
		attempts++
		g, err := newSampleGame(selection, biddingThreshold, estimator)
		if err != nil {
			return nil, err
		}
		if g == nil {
			if attempts > targetGames*50 {
				break
			}
			continue
		}

		declarer := *g.Declarer
		declarerHand := append(game.Cards(nil), g.Players[declarer].Hand...)
		predicted := estimator.EstimateWinProbability(declarerHand, g.Mode, g.TrumpSuit, g.PlayedHand, g.AnnouncedSchneider, g.AnnouncedSchwarz)
		mode := g.Mode

		agent.WithAgentCardPlay(g)
		result := g.Result()
		observations = append(observations, Observation{
			Mode:               mode,
			Predicted:          predicted,
			Actual:             result.DeclarerWon,
			PlayedHand:         g.PlayedHand,
			AnnouncedSchneider: g.AnnouncedSchneider,
			AnnouncedSchwarz:   g.AnnouncedSchwarz,
			IsSchneider:        result.IsSchneider,
			IsSchwarz:          result.IsSchwarz,
		})
	}
	return observations, nil
}

func newSampleGame(selection string, biddingThreshold float64, estimator strategies.ContractWinProbabilityEstimator) (*game.GameState, error) {
	sampleAgent, err := newSampleAgent(selection, biddingThreshold, estimator)
	if err != nil {
		return nil, err
	}
	config := agent.NewThreeWayConfig(sampleAgent, sampleAgent.Clone(), sampleAgent.Clone())
	g := agent.WithAgentPlayers(game.NewGame(), config).WithCardsDealt()
	g = agent.WithAgentBidding(g, config)
	if g.Declarer == nil || g.Phase != game.PhaseSkatExchange {
		return nil, nil
	}
	var overbid bool
	g, overbid = agent.WithAgentSkatExchange(g)
	if overbid || g.Phase != game.PhasePlaying || g.Mode == game.ModeRamsch {
		return nil, nil
	}
	return g, nil
}

func newSampleAgent(selection string, biddingThreshold float64, estimator strategies.ContractWinProbabilityEstimator) (*agent.SkatAgent, error) {
	config := agent.HybridAgentConfig{
		BiddingType:      "heuristic",
		BiddingThreshold: biddingThreshold,
		GameChoiceType:   "heuristic",
		CardPlayType:     "heuristic",
	}
	switch selection {
	case "heuristic":
		return agent.NewHybridAgent("ContractEval", config)
	case "neural":
		config.ContractEstimator = estimator
		return agent.NewHybridAgent("ContractEval", config)
	default:
		return nil, fmt.Errorf("unknown selection model: %s", selection)
	}
}

func summarizeObservations(observations []Observation, binWidth float64) EvalResult {
	result := EvalResult{Observations: observations}
	if len(observations) == 0 {
		return result
	}

	byMode := map[game.GameMode][]Observation{}
	byDeclaration := map[string][]Observation{}
	byOutcome := map[string][]Observation{}
	buckets := map[int]*BucketStats{}

	for _, obs := range observations {
		actual := 0.0
		if obs.Actual {
			actual = 1
		}
		diff := obs.Predicted - actual
		result.BrierScore += diff * diff
		result.LogLoss += BinaryLogLoss(obs.Predicted, actual)
		byMode[obs.Mode] = append(byMode[obs.Mode], obs)
		if obs.PlayedHand {
			byDeclaration["Hand"] = append(byDeclaration["Hand"], obs)
		}
		if obs.AnnouncedSchneider {
			byDeclaration["Schneider"] = append(byDeclaration["Schneider"], obs)
		}
		if obs.AnnouncedSchwarz {
			byDeclaration["Schwarz"] = append(byDeclaration["Schwarz"], obs)
		}
		if obs.IsSchneider {
			byOutcome["Schneider"] = append(byOutcome["Schneider"], obs)
		}
		if obs.IsSchwarz {
			byOutcome["Schwarz"] = append(byOutcome["Schwarz"], obs)
		}

		key := int(math.Floor(obs.Predicted / binWidth))
		if obs.Predicted >= 1 {
			key = int(math.Floor((1 - 1e-9) / binWidth))
		}
		bucket := buckets[key]
		if bucket == nil {
			low := float64(key) * binWidth
			bucket = &BucketStats{Low: low, High: low + binWidth}
			buckets[key] = bucket
		}
		bucket.Count++
		bucket.AvgPredicted += obs.Predicted
		if obs.Actual {
			bucket.Wins++
		}
	}

	total := float64(len(observations))
	result.BrierScore /= total
	result.LogLoss /= total
	result.Overall = summarizeMode("All", observations)
	for _, mode := range []game.GameMode{game.ModeGrand, game.ModeSuit, game.ModeNull} {
		if summary := summarizeMode(string(mode), byMode[mode]); summary.Games > 0 {
			result.ByMode = append(result.ByMode, summary)
		}
	}
	for _, declaration := range []string{"Hand", "Schneider", "Schwarz"} {
		if summary := summarizeMode(declaration, byDeclaration[declaration]); summary.Games > 0 {
			result.ByDeclaration = append(result.ByDeclaration, summary)
		}
	}
	for _, outcome := range []string{"Schneider", "Schwarz"} {
		if summary := summarizeMode(outcome, byOutcome[outcome]); summary.Games > 0 {
			result.ByOutcome = append(result.ByOutcome, summary)
		}
	}

	keys := make([]int, 0, len(buckets))
	for key := range buckets {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	for _, key := range keys {
		bucket := *buckets[key]
		bucket.AvgPredicted /= float64(bucket.Count)
		bucket.ActualRate = float64(bucket.Wins) / float64(bucket.Count)
		result.Buckets = append(result.Buckets, bucket)
	}
	return result
}

func summarizeMode(label string, observations []Observation) ModeSummary {
	if len(observations) == 0 {
		return ModeSummary{Label: label}
	}
	wins := 0
	expectedWins := 0.0
	for _, obs := range observations {
		expectedWins += obs.Predicted
		if obs.Actual {
			wins++
		}
	}
	actualRate := float64(wins) / float64(len(observations))
	expectedRate := expectedWins / float64(len(observations))
	return ModeSummary{
		Label:        label,
		Games:        len(observations),
		ActualRate:   actualRate,
		ExpectedRate: expectedRate,
		Delta:        actualRate - expectedRate,
	}
}

func PrintEvalSummary(w io.Writer, result EvalResult) {
	if len(result.Observations) == 0 {
		fmt.Fprintln(w, "No completed games collected")
		return
	}

	fmt.Fprintf(w, "Games with estimates: %d\n", len(result.Observations))
	fmt.Fprintf(w, "%-12s %7s %10s %10s %12s\n", "Contract", "Games", "Actual", "Expected", "Above exp.")
	printModeSummary(w, result.Overall)
	for _, summary := range result.ByMode {
		printModeSummary(w, summary)
	}
	if len(result.ByDeclaration) > 0 {
		fmt.Fprintln(w, "Declaration frequencies and win rates:")
		for _, summary := range result.ByDeclaration {
			label := summary.Label
			if label != "Hand" {
				label += " announced"
			}
			fmt.Fprintf(w, "  %-9s %7d (%5.1f%%), actual %5.1f%%, expected %5.1f%%\n",
				label+":", summary.Games,
				float64(summary.Games)/float64(len(result.Observations))*100,
				summary.ActualRate*100, summary.ExpectedRate*100)
		}
	}
	if len(result.ByOutcome) > 0 {
		fmt.Fprintln(w, "Achieved result frequencies and win rates:")
		for _, summary := range result.ByOutcome {
			fmt.Fprintf(w, "  %-19s %7d (%5.1f%%), declarer won %5.1f%%\n",
				summary.Label+":", summary.Games,
				float64(summary.Games)/float64(len(result.Observations))*100,
				summary.ActualRate*100)
		}
	}
	fmt.Fprintf(w, "Brier score: %.4f\n", result.BrierScore)
	fmt.Fprintf(w, "Log loss: %.4f\n", result.LogLoss)
	fmt.Fprintf(w, "Overall win rate: %.1f%% | expected: %.1f%% | delta: %+.1f pp\n\n",
		result.Overall.ActualRate*100,
		result.Overall.ExpectedRate*100,
		result.Overall.Delta*100)

	fmt.Fprintf(w, "%-15s %-10s %-10s %-10s %s\n", "Expected", "Games", "Actual", "Avg Pred", "Vs Expectation")
	fmt.Fprintln(w, strings.Repeat("-", 70))
	for _, bucket := range result.Buckets {
		diff := bucket.ActualRate - bucket.AvgPredicted
		calibration := "ok"
		if diff > 0.05 {
			calibration = fmt.Sprintf("under %.1f%%", diff*100)
		} else if diff < -0.05 {
			calibration = fmt.Sprintf("over %.1f%%", -diff*100)
		}
		fmt.Fprintf(w, "%-15s %-10d %-10.1f %-10.1f %s\n",
			fmt.Sprintf("%.0f%%-%.0f%%", bucket.Low*100, bucket.High*100),
			bucket.Count,
			bucket.ActualRate*100,
			bucket.AvgPredicted*100,
			calibration)
	}
}

func printModeSummary(w io.Writer, summary ModeSummary) {
	if summary.Games == 0 {
		return
	}
	fmt.Fprintf(w, "%-12s %7d %9.1f%% %9.1f%% %+10.1f pp\n",
		summary.Label,
		summary.Games,
		summary.ActualRate*100,
		summary.ExpectedRate*100,
		summary.Delta*100)
}

func BinaryLogLoss(predicted, actual float64) float64 {
	p := clampProbability(predicted)
	if actual == 1 {
		return -math.Log(p)
	}
	return -math.Log(1 - p)
}

func clampProbability(probability float64) float64 {
	const epsilon = 1e-9
	if probability < epsilon {
		return epsilon
	}
	if probability > 1-epsilon {
		return 1 - epsilon
	}
	return probability
}
