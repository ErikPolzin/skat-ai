package main

import (
	"fmt"
	"skat/logger"
	"skat/training"
)

func main() {
	// Initialize and disable logger for training
	logger.Initialize("train_game_choice")
	logger.Disable()

	fmt.Println("Skat AI - Game Mode Selection Training")
	fmt.Println("========================================")
	fmt.Println()

	trainer := training.NewGameChoiceTrainer()

	episodes := 5000
	fmt.Printf("Training game mode selection for %d episodes...\n", episodes)
	fmt.Println()

	trainer.TrainGameChoice(episodes)

	agent := trainer.GetAgent(0)

	// Save Q-table
	fmt.Println("\nSaving trained game choice agent...")
	if err := agent.SaveGameChoiceQTableBinary("game_choice_qtable.gob"); err != nil {
		fmt.Printf("✗ Error saving: %v\n", err)
	} else {
		fmt.Println("✓ Saved to game_choice_qtable.gob")
	}

	fmt.Println("\nTraining complete!")
}
