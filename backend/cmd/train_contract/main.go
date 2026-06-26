package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"

	"skat/agent/strategies"
	strategiesio "skat/agent/strategies/io"
	"skat/agent/training/contract"
)

func main() {
	datasetFile := flag.String("dataset", ".data/contract_dataset.csv", "Path to contract dataset")
	epochs := flag.Int("epochs", 20, "Number of training epochs")
	batchSize := flag.Int("batch", 128, "Batch size")
	lr := flag.Float64("lr", 0.001, "Learning rate")
	l2Reg := flag.Float64("l2", 0.0001, "L2 regularization")
	evalEvery := flag.Int("eval-every", 1, "Evaluate every N epochs")
	evalGames := flag.Int("eval-games", 500, "Number of games per evaluation")
	evalSelection := flag.String("eval-selection", "heuristic", "Evaluation contract selection model: heuristic or neural")
	evalBiddingThreshold := flag.Float64("eval-bidding-threshold", 0.55, "Contract threshold used during training-time evaluation")
	outputWeights := flag.String("output", ".data/models/contract.weights", "Output weights file")
	flag.Parse()

	fmt.Println("============================================================")
	fmt.Println("Skat Neural Contract Win Probability Training")
	fmt.Println("============================================================")
	fmt.Printf("\nConfiguration:\n")
	fmt.Printf("  Dataset: %s\n", *datasetFile)
	fmt.Printf("  Epochs: %d\n", *epochs)
	fmt.Printf("  Batch Size: %d\n", *batchSize)
	fmt.Printf("  Learning Rate: %.5f\n", *lr)
	fmt.Printf("  L2 Regularization: %.5f\n", *l2Reg)
	fmt.Printf("  Evaluation: every %d epochs, %d games, %s selection\n", *evalEvery, *evalGames, *evalSelection)
	fmt.Printf("  Eval bidding threshold: %.2f\n", *evalBiddingThreshold)
	fmt.Printf("  Output: %s\n\n", *outputWeights)

	trainer, err := contract.NewTrainer(*batchSize, *lr, *l2Reg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create trainer: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Loading dataset from %s...\n", *datasetFile)
	if err := trainer.LoadDataset(*datasetFile); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load dataset: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  Examples: %d\n\n", trainer.GetDatasetSize())

	outputDir := filepath.Dir(*outputWeights)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create output directory: %v\n", err)
		os.Exit(1)
	}

	start := time.Now()
	bestEvalLogLoss := math.Inf(1)
	savedBest := false

	for epoch := 1; epoch <= *epochs; epoch++ {
		loss, rmse, err := trainer.Train()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Training error at epoch %d: %v\n", epoch, err)
			os.Exit(1)
		}
		fmt.Printf("  [Epoch %3d/%d] Loss: %.5f RMSE: %.4f\n", epoch, *epochs, loss, rmse)

		if *evalEvery > 0 && epoch%*evalEvery == 0 {
			fmt.Printf("\n[Epoch %d] Evaluating contract calibration (%d games, %s selection)...\n", epoch, *evalGames, *evalSelection)
			estimator := strategies.NewNeuralContractWinProbabilityEstimatorFromWeightMap(trainer.GetWeights())
			result, err := contract.EvaluateEstimator(contract.EvalConfig{
				Games:            *evalGames,
				Selection:        *evalSelection,
				BiddingThreshold: *evalBiddingThreshold,
			}, estimator)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Evaluation error at epoch %d: %v\n", epoch, err)
				os.Exit(1)
			}
			contract.PrintEvalSummary(os.Stdout, result)

			if result.LogLoss < bestEvalLogLoss {
				bestEvalLogLoss = result.LogLoss
				if err := strategiesio.SaveWeights(*outputWeights, trainer.GetWeights()); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to save best weights: %v\n", err)
					os.Exit(1)
				}
				savedBest = true
				fmt.Printf("  → New best checkpoint saved to %s (log loss %.4f)\n\n", *outputWeights, bestEvalLogLoss)
			}
		}
	}

	finalWeights := *outputWeights
	if savedBest {
		finalWeights = *outputWeights + ".final"
	}

	if err := strategiesio.SaveWeights(finalWeights, trainer.GetWeights()); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save weights: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nTraining complete in %s\n", time.Since(start))
	if savedBest {
		fmt.Printf("Best model saved to: %s (best log loss %.4f)\n", *outputWeights, bestEvalLogLoss)
		fmt.Printf("Final epoch model saved to: %s\n", finalWeights)
	} else {
		fmt.Printf("Model saved to: %s\n", finalWeights)
	}
}
