package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"

	"skat/agent"
	"skat/agent/strategies"
	strategiesio "skat/agent/strategies/io"
	"skat/agent/training"
	"skat/agent/training/cardplay"

	"gorgonia.org/gorgonia"
)

func main() {
	// Parse flags
	datasetFile := flag.String("dataset", ".data/cardplay_dataset.csv", "Path to cardplay dataset")
	epochs := flag.Int("epochs", 20, "Number of training epochs")
	batchSize := flag.Int("batch", 128, "Batch size")
	lr := flag.Float64("lr", 0.001, "Learning rate")
	l2Reg := flag.Float64("l2", 0.0001, "L2 regularization")
	evalEvery := flag.Int("eval-every", 1, "Evaluate every N epochs")
	evalGames := flag.Int("eval-games", 500, "Number of games per evaluation")
	evalBiddingThreshold := flag.Float64("eval-bidding-threshold", 0.55, "Heuristic bidding threshold used during training-time evaluation")
	initialWeights := flag.String("initial", "", "Optional initial combined card-play weights to continue training from")
	outputWeights := flag.String("output", ".data/models/cardplay.weights", "Output weights file")

	flag.Parse()

	fmt.Println("============================================================")
	fmt.Println("Skat Neural Card Play Training")
	fmt.Println("============================================================")
	fmt.Printf("\nConfiguration:\n")
	fmt.Printf("  Dataset: %s\n", *datasetFile)
	fmt.Printf("  Epochs: %d\n", *epochs)
	fmt.Printf("  Batch Size: %d\n", *batchSize)
	fmt.Printf("  Learning Rate: %.5f\n", *lr)
	fmt.Printf("  L2 Regularization: %.5f\n", *l2Reg)
	fmt.Printf("  Evaluation: every %d epochs\n", *evalEvery)
	fmt.Printf("  Eval bidding threshold: %.2f\n", *evalBiddingThreshold)
	if *initialWeights != "" {
		fmt.Printf("  Initial weights: %s\n", *initialWeights)
	}
	fmt.Printf("  Output: %s\n\n", *outputWeights)

	// Create trainer
	trainer, err := cardplay.NewTrainer(*batchSize, *lr, *l2Reg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create trainer: %v\n", err)
		os.Exit(1)
	}

	// Load dataset
	fmt.Printf("Loading dataset from %s...\n", *datasetFile)
	if err := trainer.LoadDataset(*datasetFile); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load dataset: %v\n", err)
		os.Exit(1)
	}
	if *initialWeights != "" {
		declWeights, defWeights, ramWeights, err := loadInitialWeights(*initialWeights)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to load initial weights: %v\n", err)
			os.Exit(1)
		}
		if err := trainer.SetWeights(declWeights, defWeights, ramWeights); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to set initial weights: %v\n", err)
			os.Exit(1)
		}
	}

	declExamples, defExamples, ramExamples := trainer.GetDatasetSizes()
	fmt.Printf("  Declarer examples: %d\n", declExamples)
	fmt.Printf("  Defender examples: %d\n", defExamples)
	fmt.Printf("  Ramsch examples: %d\n", ramExamples)
	fmt.Printf("  Total examples: %d\n\n", declExamples+defExamples+ramExamples)

	// Training loop
	fmt.Println("Starting training...")
	startTime := time.Now()
	bestEvalScore := math.Inf(-1)
	savedBest := false

	outputDir := filepath.Dir(*outputWeights)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create output directory: %v\n", err)
		os.Exit(1)
	}

	for epoch := 1; epoch <= *epochs; epoch++ {
		// Train one epoch
		declLoss, defLoss, ramLoss, declAcc, defAcc, ramAcc, err := trainer.Train()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Training error at epoch %d: %v\n", epoch, err)
			os.Exit(1)
		}

		fmt.Printf("  [Epoch %3d/%d] Decl Loss: %.4f Acc: %.1f%% | Def Loss: %.4f Acc: %.1f%% | Ramsch Loss: %.4f Acc: %.1f%%\n",
			epoch, *epochs, declLoss, declAcc*100, defLoss, defAcc*100, ramLoss, ramAcc*100)

		// Evaluate periodically
		if epoch%*evalEvery == 0 {
			fmt.Printf("\n[Epoch %d] Evaluating against heuristic baseline (%d games)...\n", epoch, *evalGames)

			// Get current weights
			declWeights, defWeights, ramWeights := trainer.GetWeights()

			// Create test agent with current weights (no exploration)
			testStrategy := strategies.NewNeuralCardPlayStrategyFromWeightMaps(declWeights, defWeights, ramWeights)
			testStrategy.SetExploration(0.0)

			cfg := strategies.DefaultContractEvaluatorConfig()
			cfg.MinWinProbability = *evalBiddingThreshold

			testAgent := agent.NewAgentWithStrategies(
				"Card Play",
				strategies.NewHeuristicBiddingStrategyWithConfig(cfg),
				strategies.NewHeuristicGameChoiceStrategyWithConfig(cfg),
				testStrategy,
			)

			baselineAgent := agent.NewHeuristicAgent("Baseline")

			// Run evaluation with 50/50 split
			config := agent.NewFiftyFiftySplitConfig(testAgent, baselineAgent)
			training.EvaluateAgents(config, *evalGames)
			testStats := config.TestAgent.GetMetrics()
			baselineStats := config.BaselineAgent.GetMetrics()
			testDeclAdjusted := handAdjustedWinRate(testStats, roleDeclarer)
			testDefAdjusted := handAdjustedWinRate(testStats, roleDefender)
			baselineDeclAdjusted := handAdjustedWinRate(baselineStats, roleDeclarer)
			baselineDefAdjusted := handAdjustedWinRate(baselineStats, roleDefender)

			// Calculate win rates
			testDeclWinRate := 0.0
			if testStats.Games > 0 {
				testDeclWinRate = float64(testStats.Wins) / float64(testStats.Games) * 100
			}
			baselineDeclWinRate := 0.0
			if baselineStats.Games > 0 {
				baselineDeclWinRate = float64(baselineStats.Wins) / float64(baselineStats.Games) * 100
			}

			// Calculate defender win rates
			testDefWinRate := 0.0
			if testStats.DefenderGames > 0 {
				testDefWinRate = float64(testStats.DefenderWins) / float64(testStats.DefenderGames) * 100
			}

			baselineDefWinRate := 0.0
			if baselineStats.DefenderGames > 0 {
				baselineDefWinRate = float64(baselineStats.DefenderWins) / float64(baselineStats.DefenderGames) * 100
			}

			fmt.Printf("  → Card Play: Decl %.1f%% (%d/%d) | Def %.1f%% (%d/%d)\n",
				testDeclWinRate, testStats.Wins, testStats.Games,
				testDefWinRate, testStats.DefenderWins, testStats.DefenderGames)
			fmt.Printf("  → Baseline:  Decl %.1f%% (%d/%d) | Def %.1f%% (%d/%d)\n\n",
				baselineDeclWinRate, baselineStats.Wins, baselineStats.Games,
				baselineDefWinRate, baselineStats.DefenderWins, baselineStats.DefenderGames)
			fmt.Printf("  → Hand-adjusted: Card Play Decl %+.1fpp (expected %.1f%%) | Def %+.1fpp (expected %.1f%%)\n",
				testDeclAdjusted.excessPct, testDeclAdjusted.expectedPct,
				testDefAdjusted.excessPct, testDefAdjusted.expectedPct)
			fmt.Printf("  → Hand-adjusted: Baseline  Decl %+.1fpp (expected %.1f%%) | Def %+.1fpp (expected %.1f%%)\n",
				baselineDeclAdjusted.excessPct, baselineDeclAdjusted.expectedPct,
				baselineDefAdjusted.excessPct, baselineDefAdjusted.expectedPct)

			testAdjusted := handAdjustedWinRate(testStats, roleAny)
			baselineAdjusted := handAdjustedWinRate(baselineStats, roleAny)
			evalScore := testAdjusted.excessPct - baselineAdjusted.excessPct
			pointDiff := testStats.Points - baselineStats.Points
			fmt.Printf("  → Adjusted advantage: %+.1fpp | Raw point diff: %d\n\n", evalScore, pointDiff)
			if evalScore > bestEvalScore {
				bestEvalScore = evalScore
				if err := strategiesio.SaveCombinedCardPlayWeights(*outputWeights, declWeights, defWeights, ramWeights); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to save best weights: %v\n", err)
					os.Exit(1)
				}
				savedBest = true
				fmt.Printf("  → New best checkpoint saved to %s (adjusted advantage %+.1fpp)\n\n", *outputWeights, evalScore)
			}
		}
	}

	// Save final model
	declWeights, defWeights, ramWeights := trainer.GetWeights()

	finalWeights := *outputWeights
	if savedBest {
		finalWeights = *outputWeights + ".final"
	}

	if err := strategiesio.SaveCombinedCardPlayWeights(finalWeights, declWeights, defWeights, ramWeights); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save weights: %v\n", err)
		os.Exit(1)
	}

	elapsed := time.Since(startTime)
	fmt.Printf("\n✓ Training complete in %s\n", elapsed)
	if savedBest {
		fmt.Printf("✓ Best model saved to: %s (best adjusted advantage %+.1fpp)\n", *outputWeights, bestEvalScore)
		fmt.Printf("✓ Final epoch model saved to: %s\n", finalWeights)
	} else {
		fmt.Printf("✓ Model saved to: %s\n", finalWeights)
	}
}

type evaluationRole int

const (
	roleAny evaluationRole = iota
	roleDeclarer
	roleDefender
)

type adjustedWinRate struct {
	expectedPct float64
	actualPct   float64
	excessPct   float64
	games       int
}

// handAdjustedWinRate compares outcomes with the strategy-independent contract
// estimate recorded for the same hands. Excess percentage points above zero
// indicate that card play won more often than hand strength alone predicted.
func handAdjustedWinRate(metrics agent.AgentMetricsSnapshot, role evaluationRole) adjustedWinRate {
	count := len(metrics.PredictedProbability)
	if len(metrics.ActualOutcomes) < count {
		count = len(metrics.ActualOutcomes)
	}
	if len(metrics.OutcomeIsDeclarer) < count {
		count = len(metrics.OutcomeIsDeclarer)
	}

	var expected, actual float64
	games := 0
	for i := 0; i < count; i++ {
		isDeclarer := metrics.OutcomeIsDeclarer[i]
		if role == roleDeclarer && !isDeclarer || role == roleDefender && isDeclarer {
			continue
		}
		expected += metrics.PredictedProbability[i]
		if metrics.ActualOutcomes[i] {
			actual++
		}
		games++
	}
	if games == 0 {
		return adjustedWinRate{}
	}

	expectedPct := expected / float64(games) * 100
	actualPct := actual / float64(games) * 100
	return adjustedWinRate{
		expectedPct: expectedPct,
		actualPct:   actualPct,
		excessPct:   actualPct - expectedPct,
		games:       games,
	}
}

func loadInitialWeights(path string) (strategies.CardPlayNetworkWeights, strategies.CardPlayNetworkWeights, strategies.CardPlayNetworkWeights, error) {
	declGraph := gorgonia.NewGraph()
	defGraph := gorgonia.NewGraph()
	ramGraph := gorgonia.NewGraph()
	declWeights, defWeights, ramWeights, err := strategiesio.LoadCombinedCardPlayWeights(path, declGraph, defGraph, ramGraph)
	if err != nil {
		return nil, nil, nil, err
	}
	return strategies.CardPlayNetworkWeights(declWeights), strategies.CardPlayNetworkWeights(defWeights), strategies.CardPlayNetworkWeights(ramWeights), nil
}
