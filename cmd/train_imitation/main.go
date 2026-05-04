package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"skat/agent"
	"skat/agent/strategies"
	strategiesio "skat/agent/strategies/io"
	"skat/agent/training"
	"skat/agent/training/imitation"
)

func main() {
	// Parse flags
	datasetFile := flag.String("dataset", ".data/imitation_dataset.csv", "Path to imitation dataset")
	epochs := flag.Int("epochs", 50, "Number of training epochs")
	batchSize := flag.Int("batch", 128, "Batch size")
	lr := flag.Float64("lr", 0.001, "Learning rate")
	l2Reg := flag.Float64("l2", 0.0001, "L2 regularization")
	evalEvery := flag.Int("eval-every", 5, "Evaluate every N epochs")
	evalGames := flag.Int("eval-games", 200, "Number of games per evaluation")
	outputWeights := flag.String("output", ".data/models/imitation_cardplay.weights", "Output weights file")

	flag.Parse()

	fmt.Println("============================================================")
	fmt.Println("Skat Behavioral Cloning (Imitation Learning)")
	fmt.Println("============================================================")
	fmt.Printf("\nConfiguration:\n")
	fmt.Printf("  Dataset: %s\n", *datasetFile)
	fmt.Printf("  Epochs: %d\n", *epochs)
	fmt.Printf("  Batch Size: %d\n", *batchSize)
	fmt.Printf("  Learning Rate: %.5f\n", *lr)
	fmt.Printf("  L2 Regularization: %.5f\n", *l2Reg)
	fmt.Printf("  Evaluation: every %d epochs\n", *evalEvery)
	fmt.Printf("  Output: %s\n\n", *outputWeights)

	// Create trainer
	trainer, err := imitation.NewBehavioralCloningTrainer(*batchSize, *lr, *l2Reg)
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

	declExamples, defExamples := trainer.GetDatasetSizes()
	fmt.Printf("  Declarer examples: %d\n", declExamples)
	fmt.Printf("  Defender examples: %d\n", defExamples)
	fmt.Printf("  Total examples: %d\n\n", declExamples+defExamples)

	// Training loop
	fmt.Println("Starting training...")
	startTime := time.Now()

	for epoch := 1; epoch <= *epochs; epoch++ {
		// Train one epoch
		declLoss, defLoss, err := trainer.Train()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Training error at epoch %d: %v\n", epoch, err)
			os.Exit(1)
		}

		fmt.Printf("  [Epoch %3d/%d] Declarer Loss: %.4f | Defender Loss: %.4f\n",
			epoch, *epochs, declLoss, defLoss)

		// Evaluate periodically
		if epoch%*evalEvery == 0 {
			fmt.Printf("\n[Epoch %d] Evaluating against heuristic baseline (%d games)...\n", epoch, *evalGames)

			// Get current weights
			declWeights, defWeights := trainer.GetWeights()

			// Create test agent with current weights (no exploration)
			testStrategy := strategies.NewNeuralCardPlayStrategyFromWeightMaps(declWeights, defWeights)
			testStrategy.SetExploration(0.0)

			weightedBidding := strategies.NewWeightedHeuristicBiddingStrategy()
			weightedBidding.SetBiddingThreshold(0.65)

			testAgent := agent.NewAgentWithStrategies(
				"Imitation",
				weightedBidding,
				&agent.HeuristicGameChoiceStrategy{},
				testStrategy,
			)

			baselineAgent := agent.NewHeuristicAgent("Baseline")

			// Run evaluation with 50/50 split
			config := training.NewFiftyFiftySplitConfig(testAgent, baselineAgent, 0)
			stats := training.EvaluateAgents(config, *evalGames)

			// Calculate win rates
			testDeclWinRate := 0.0
			if stats.TestGames > 0 {
				testDeclWinRate = float64(stats.TestWins) / float64(stats.TestGames) * 100
			}
			baselineDeclWinRate := 0.0
			if stats.BaselineGames > 0 {
				baselineDeclWinRate = float64(stats.BaselineWins) / float64(stats.BaselineGames) * 100
			}

			// Calculate defender win rates
			testDefWins := stats.BaselineGames - stats.BaselineWins
			testDefGames := stats.BaselineGames
			testDefWinRate := 0.0
			if testDefGames > 0 {
				testDefWinRate = float64(testDefWins) / float64(testDefGames) * 100
			}

			baselineDefWins := stats.TestGames - stats.TestWins
			baselineDefGames := stats.TestGames
			baselineDefWinRate := 0.0
			if baselineDefGames > 0 {
				baselineDefWinRate = float64(baselineDefWins) / float64(baselineDefGames) * 100
			}

			fmt.Printf("  → Imitation: Decl %.1f%% (%d/%d) | Def %.1f%% (%d/%d)\n",
				testDeclWinRate, stats.TestWins, stats.TestGames,
				testDefWinRate, testDefWins, testDefGames)
			fmt.Printf("  → Baseline:  Decl %.1f%% (%d/%d) | Def %.1f%% (%d/%d)\n\n",
				baselineDeclWinRate, stats.BaselineWins, stats.BaselineGames,
				baselineDefWinRate, baselineDefWins, baselineDefGames)
		}
	}

	// Save final model
	declWeights, defWeights := trainer.GetWeights()
	declarerPath := *outputWeights + ".declarer"
	defenderPath := *outputWeights + ".defender"

	if err := strategiesio.SaveWeights(declarerPath, declWeights); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save declarer weights: %v\n", err)
		os.Exit(1)
	}
	if err := strategiesio.SaveWeights(defenderPath, defWeights); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save defender weights: %v\n", err)
		os.Exit(1)
	}

	elapsed := time.Since(startTime)
	fmt.Printf("\n✓ Training complete in %s\n", elapsed)
	fmt.Printf("✓ Declarer network saved to: %s\n", declarerPath)
	fmt.Printf("✓ Defender network saved to: %s\n", defenderPath)
}
