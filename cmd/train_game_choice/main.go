package main

import (
	"flag"
	"fmt"
	"skat/agent"
	"skat/agent/training"
	"skat/logger"
	"time"
)

func main() {
	episodes := flag.Int("episodes", 5000, "Number of training episodes")
	withQLearningBidding := flag.Bool("with-qlearning-bidding", false, "Train with Q-learning bidding strategy")
	flag.Parse()

	// Initialize and disable logger for training
	logger.Initialize("train_game_choice")
	logger.Disable()

	fmt.Println("Skat AI - Game Mode Selection Training")
	fmt.Println("========================================")
	fmt.Printf("Episodes: %d\n", *episodes)
	if *withQLearningBidding {
		fmt.Println("Using Q-learning bidding strategy")
	}
	fmt.Println()

	var trainer *training.GameChoiceTrainer

	if *withQLearningBidding {
		// Load the trained bidding Q-table
		fmt.Println("Loading bidding Q-table...")
		biddingData, err := agent.LoadQTableData("bidding_qtable.gob", true)
		if err != nil {
			fmt.Printf("Error loading bidding Q-table: %v\n", err)
			fmt.Println("Please train bidding first: go run cmd/train_bidding/main.go")
			return
		}
		fmt.Println("✓ Loaded bidding Q-table")
		trainer = training.NewGameChoiceTrainerWithQLearningBidding(biddingData.QTable)
	} else {
		trainer = training.NewGameChoiceTrainer()
	}

	startTime := time.Now()
	trainer.TrainGameChoice(*episodes)
	duration := time.Since(startTime)

	trainedAgent := trainer.GetGameChoiceAgent(0)

	// Save Q-table
	fmt.Println("\nSaving trained game choice agent...")
	if qStrat, ok := trainedAgent.GetGameChoiceStrategy().(*agent.QLearningGameChoiceStrategy); ok {
		data := &agent.QTableData{
			QTable:  qStrat.GetQTable(),
			Epsilon: qStrat.GetEpsilon(),
		}
		if err := agent.SaveQTableData(data, "game_choice_qtable.gob", true); err != nil {
			fmt.Printf("✗ Error saving: %v\n", err)
		} else {
			fmt.Println("✓ Saved to game_choice_qtable.gob")
		}
	}

	fmt.Printf("\nTraining time: %v\n", duration.Round(time.Second))
	fmt.Println("\nTraining complete!")
}
