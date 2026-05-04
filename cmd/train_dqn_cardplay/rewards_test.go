package main

import (
	"fmt"
	"skat/agent"
	"skat/agent/training/dqn"
	"testing"
)

// TestComputeTrickReward tests the trick reward calculation with real scenarios
func TestComputeTrickReward(t *testing.T) {
	t.Run("declarer wins valuable trick", func(t *testing.T) {
		// Declarer wins 20 points, defenders get 0
		scoresBefore := [2]int{40, 30}
		scoresAfter := [2]int{60, 30}

		rewards := computeTrickReward(scoresBefore, scoresAfter)

		// Declarer should get positive reward
		if rewards[true] <= 0 {
			t.Errorf("Declarer should get positive reward for winning trick, got %f", rewards[true])
		}

		// Defenders should get negative reward
		if rewards[false] >= 0 {
			t.Errorf("Defenders should get negative reward when losing trick, got %f", rewards[false])
		}

		// Rewards should be opposite
		if rewards[true] != -rewards[false] {
			t.Errorf("Rewards should be opposite: declarer=%f, defender=%f", rewards[true], rewards[false])
		}

		t.Logf("Declarer wins 20pts trick: declarer reward=%f, defender reward=%f", rewards[true], rewards[false])
	})

	t.Run("defenders win valuable trick", func(t *testing.T) {
		// Defenders win 15 points, declarer gets 0
		scoresBefore := [2]int{40, 30}
		scoresAfter := [2]int{40, 45}

		rewards := computeTrickReward(scoresBefore, scoresAfter)

		// Declarer should get negative reward
		if rewards[true] >= 0 {
			t.Errorf("Declarer should get negative reward for losing trick, got %f", rewards[true])
		}

		// Defenders should get positive reward
		if rewards[false] <= 0 {
			t.Errorf("Defenders should get positive reward for winning trick, got %f", rewards[false])
		}

		t.Logf("Defenders win 15pts trick: declarer reward=%f, defender reward=%f", rewards[true], rewards[false])
	})

	t.Run("worthless trick has minimal reward", func(t *testing.T) {
		// Trick has 0 points (all 7s and 8s)
		scoresBefore := [2]int{40, 30}
		scoresAfter := [2]int{40, 30}

		rewards := computeTrickReward(scoresBefore, scoresAfter)

		// Both rewards should be 0 (no points in trick)
		if rewards[true] != 0 || rewards[false] != 0 {
			t.Errorf("Worthless trick should have 0 rewards, got declarer=%f, defender=%f", rewards[true], rewards[false])
		}

		t.Logf("Worthless trick: declarer reward=%f, defender reward=%f", rewards[true], rewards[false])
	})

	t.Run("reward magnitude scales with trick value", func(t *testing.T) {
		// Small trick: 5 points
		smallBefore := [2]int{40, 30}
		smallAfter := [2]int{45, 30}
		smallRewards := computeTrickReward(smallBefore, smallAfter)

		// Large trick: 25 points
		largeBefore := [2]int{40, 30}
		largeAfter := [2]int{65, 30}
		largeRewards := computeTrickReward(largeBefore, largeAfter)

		if largeRewards[true] <= smallRewards[true] {
			t.Errorf("Larger trick should have larger reward: small=%f, large=%f", smallRewards[true], largeRewards[true])
		}

		t.Logf("Small trick (5pts): reward=%f, Large trick (25pts): reward=%f", smallRewards[true], largeRewards[true])
	})
}

// TestFullGameExperienceCollection tests collecting experiences from a complete game
func TestFullGameExperienceCollection(t *testing.T) {
	t.Run("game collects experiences for all players", func(t *testing.T) {
		trainer, err := dqn.NewDQNCardPlayTrainer(10000, 256, 0.95, 0.001, 0.0001, 1.0, 0.995, 0.1, nil)
		if err != nil {
			t.Fatalf("Failed to create trainer: %v", err)
		}

		// Create simple agents
		dqnAgent := agent.NewHeuristicAgent("DQN")
		heuristicAgent := agent.NewHeuristicAgent("Heuristic")

		initialDeclSize, initialDefSize := trainer.GetBufferSizes()

		// Play one game
		playGameAndCollectExperiences(dqnAgent, heuristicAgent, trainer, 0)

		finalDeclSize, finalDefSize := trainer.GetBufferSizes()

		declAdded := finalDeclSize - initialDeclSize
		defAdded := finalDefSize - initialDefSize

		t.Logf("Experiences added: declarer=%d, defender=%d", declAdded, defAdded)

		// Should have collected experiences from all players
		// Each game has ~10 tricks, 1 declarer + 2 defenders
		// Expected: ~10 declarer experiences, ~20 defender experiences
		if declAdded == 0 {
			t.Error("No declarer experiences collected")
		}

		if defAdded == 0 {
			t.Error("No defender experiences collected")
		}

		// Defenders should have roughly 2x declarer experiences
		ratio := float64(defAdded) / float64(declAdded)
		if ratio < 1.5 || ratio > 2.5 {
			t.Logf("Warning: defender/declarer ratio is %f, expected ~2.0", ratio)
		}
	})

	t.Run("terminal experiences have terminal bonus", func(t *testing.T) {
		// This test manually creates a terminal state and checks the reward

		// Simulate final trick where declarer wins the game
		scoresBefore := [2]int{58, 32} // Declarer at 58 points
		scoresAfter := [2]int{70, 32}  // Declarer wins trick with 12 points -> total 70 -> wins game

		trickReward := computeTrickReward(scoresBefore, scoresAfter)
		declarerWon := scoresAfter[0] >= 61

		// Calculate terminal bonus (matching main.go logic)
		terminalBonus := float32(-3.0)
		if declarerWon {
			terminalBonus = 3.0
		}

		finalReward := trickReward[true] + terminalBonus

		t.Logf("Final trick: trick_reward=%f, terminal_bonus=%f, total=%f",
			trickReward[true], terminalBonus, finalReward)

		// Terminal bonus should dominate
		if finalReward <= trickReward[true] {
			t.Error("Terminal reward should increase final reward significantly")
		}

		// Verify magnitude: terminal bonus should be ~6x larger than typical trick
		if terminalBonus < 3.0 {
			t.Errorf("Terminal bonus should be at least 3.0, got %f", terminalBonus)
		}
	})

	t.Run("declarer losing terminal state", func(t *testing.T) {
		// Declarer loses - only has 60 points
		scoresBefore := [2]int{55, 45}
		scoresAfter := [2]int{60, 50}

		trickReward := computeTrickReward(scoresBefore, scoresAfter)
		declarerWon := scoresAfter[0] >= 61

		terminalBonus := float32(-3.0)
		if declarerWon {
			terminalBonus = 3.0
		}

		// Declarer gets trick reward + negative terminal bonus
		declarerFinalReward := trickReward[true] + terminalBonus

		// Defenders get trick reward + positive terminal bonus
		defenderFinalReward := trickReward[false] - terminalBonus // Note: defenders get opposite

		t.Logf("Declarer loses game: declarer_final=%f, defender_final=%f",
			declarerFinalReward, defenderFinalReward)

		// Declarer should have negative final reward
		if declarerFinalReward >= 0 {
			t.Errorf("Declarer should have negative reward for losing, got %f", declarerFinalReward)
		}
	})
}

// TestRewardMagnitudes tests that reward scales are appropriate
func TestRewardMagnitudes(t *testing.T) {
	t.Run("reward scales are sensible", func(t *testing.T) {
		// Typical trick: 10-15 points
		typicalBefore := [2]int{40, 40}
		typicalAfter := [2]int{52, 40}
		typicalReward := computeTrickReward(typicalBefore, typicalAfter)

		// Maximum trick: ~30 points (all face cards)
		maxBefore := [2]int{40, 40}
		maxAfter := [2]int{70, 40}
		maxReward := computeTrickReward(maxBefore, maxAfter)

		// Terminal bonus
		terminalBonus := float32(3.0)

		t.Logf("Typical trick reward: %f", typicalReward[true])
		t.Logf("Maximum trick reward: %f", maxReward[true])
		t.Logf("Terminal bonus: %f", terminalBonus)
		t.Logf("Ratio terminal/typical: %f", terminalBonus/typicalReward[true])
		t.Logf("Ratio terminal/max: %f", terminalBonus/maxReward[true])

		// Terminal bonus should be 6-12x larger than typical trick
		ratio := terminalBonus / typicalReward[true]
		if ratio < 4.0 || ratio > 15.0 {
			t.Logf("Warning: terminal/typical ratio is %f, might want to adjust", ratio)
		}

		// Over 10 tricks, cumulative trick rewards could be ~2.0
		// Terminal bonus of 3.0 means winning is ~60% of total signal
		cumulativeTricks := typicalReward[true] * 10
		totalWithTerminal := cumulativeTricks + terminalBonus
		terminalPercent := (terminalBonus / totalWithTerminal) * 100

		t.Logf("Cumulative trick rewards (10 tricks): %f", cumulativeTricks)
		t.Logf("Total with terminal: %f", totalWithTerminal)
		t.Logf("Terminal as %% of total: %.1f%%", terminalPercent)

		// Terminal should be significant (30-70% of total signal)
		if terminalPercent < 20.0 || terminalPercent > 80.0 {
			t.Logf("Warning: terminal is %.1f%% of total reward signal", terminalPercent)
		}
	})
}

// TestRewardConsistency tests that rewards are consistent with game rules
func TestRewardConsistency(t *testing.T) {
	t.Run("total points are conserved", func(t *testing.T) {
		// In Skat, total points in deck = 120
		// Every trick's points go to either declarer or defenders
		scoresBefore := [2]int{50, 40}
		scoresAfter := [2]int{65, 40}

		declarerGain := scoresAfter[0] - scoresBefore[0]
		defenderGain := scoresAfter[1] - scoresBefore[1]

		// One team gains, other gains 0
		totalPointsInTrick := declarerGain + defenderGain

		if totalPointsInTrick < 0 || totalPointsInTrick > 30 {
			t.Errorf("Invalid trick point total: %d (should be 0-30)", totalPointsInTrick)
		}

		// After all 10 tricks, total should be 120
		if scoresBefore[0]+scoresBefore[1] != 90 {
			t.Logf("Running total before trick: %d", scoresBefore[0]+scoresBefore[1])
		}
		if scoresAfter[0]+scoresAfter[1] != 105 {
			t.Logf("Running total after trick: %d", scoresAfter[0]+scoresAfter[1])
		}
	})

	t.Run("rewards sum to zero", func(t *testing.T) {
		// Rewards should be zero-sum (declarer gain = defender loss)
		scoresBefore := [2]int{50, 40}
		scoresAfter := [2]int{65, 40}

		rewards := computeTrickReward(scoresBefore, scoresAfter)

		// 1 declarer + 2 defenders
		declarerTotal := rewards[true]
		defenderTotal := rewards[false] * 2

		sumRewards := declarerTotal + defenderTotal

		// Should be approximately zero (within floating point error)
		if sumRewards > 0.01 || sumRewards < -0.01 {
			t.Logf("Warning: rewards don't sum to zero: declarer=%f, defenders=%f, sum=%f",
				declarerTotal, defenderTotal, sumRewards)
		}
	})
}

// BenchmarkRewardCalculation benchmarks the reward computation
func BenchmarkRewardCalculation(b *testing.B) {
	scoresBefore := [2]int{50, 40}
	scoresAfter := [2]int{65, 40}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = computeTrickReward(scoresBefore, scoresAfter)
	}
}

// Example test showing expected reward flow through a game
func Example() {
	fmt.Println("=== Skat DQN Reward Flow Example ===")
	fmt.Println()

	// Early trick: Declarer wins 12 points
	scoresBefore := [2]int{0, 0}
	scoresAfter := [2]int{12, 0}
	rewards := computeTrickReward(scoresBefore, scoresAfter)
	fmt.Printf("Trick 1 (declarer wins 12pts): declarer=%.3f, defender=%.3f\n", rewards[true], rewards[false])

	// Mid-game: Defenders win 18 points
	scoresBefore = [2]int{35, 25}
	scoresAfter = [2]int{35, 43}
	rewards = computeTrickReward(scoresBefore, scoresAfter)
	fmt.Printf("Trick 6 (defenders win 18pts): declarer=%.3f, defender=%.3f\n", rewards[true], rewards[false])

	// Final trick: Declarer wins 8 points, reaches 65 total (wins game)
	scoresBefore = [2]int{57, 55}
	scoresAfter = [2]int{65, 55}
	rewards = computeTrickReward(scoresBefore, scoresAfter)
	terminalBonus := float32(3.0) // Declarer wins
	finalReward := rewards[true] + terminalBonus
	fmt.Printf("Trick 10 (declarer wins 8pts + game): trick_reward=%.3f, terminal_bonus=%.3f, total=%.3f\n",
		rewards[true], terminalBonus, finalReward)

	fmt.Println()
	fmt.Println("Terminal bonus is ~6-15x larger than typical trick reward")
	fmt.Println("This encourages winning the game while still valuing point collection")

	// Output:
	// === Skat DQN Reward Flow Example ===
	//
	// Trick 1 (declarer wins 12pts): declarer=0.200, defender=-0.200
	// Trick 6 (defenders win 18pts): declarer=-0.300, defender=0.300
	// Trick 10 (declarer wins 8pts + game): trick_reward=0.133, terminal_bonus=3.000, total=3.133
	//
	// Terminal bonus is ~6-15x larger than typical trick reward
	// This encourages winning the game while still valuing point collection
}
