package main

import (
	"flag"
	"fmt"
	"skat/agent"
	"skat/agent/training"
	"skat/logger"
)

func main() {
	episodes := flag.Int("episodes", 5000, "Number of training episodes")
	flag.Parse()

	// Initialize and disable logger for training
	logger.Initialize("train_game_choice")
	logger.Disable()

	fmt.Println("Skat AI - Game Mode Selection Training")
	fmt.Println("========================================")
	fmt.Printf("Episodes: %d\n\n", *episodes)

	trainer := training.NewGameChoiceTrainer()
	trainer.TrainGameChoice(*episodes)

	trainedAgent := trainer.GetAgent(0)

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

	fmt.Println("\nTraining complete!")
}
