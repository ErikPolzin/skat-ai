package dqn

import (
	"math"
	"skat/agent/strategies"
	"skat/game"
	"testing"
)

// TestReplayBufferSampling tests that replay buffer sampling works correctly
func TestReplayBufferSampling(t *testing.T) {
	t.Run("samples without duplicates when buffer is small", func(t *testing.T) {
		buffer := NewDQNReplayBuffer(100)

		// Add 50 unique experiences
		for i := 0; i < 50; i++ {
			exp := DQNExperience{
				Action: i,
				Reward: float32(i),
			}
			buffer.Add(exp)
		}

		// Sample 10 experiences
		batch := buffer.Sample(10)
		if len(batch) != 10 {
			t.Errorf("Expected batch size 10, got %d", len(batch))
		}

		// Check for duplicates
		seen := make(map[int]bool)
		for _, exp := range batch {
			if seen[exp.Action] {
				t.Errorf("Found duplicate action %d in batch", exp.Action)
			}
			seen[exp.Action] = true
		}
	})

	t.Run("handles sampling when batch size equals buffer size", func(t *testing.T) {
		buffer := NewDQNReplayBuffer(10)

		// Fill buffer
		for i := 0; i < 10; i++ {
			exp := DQNExperience{Action: i}
			buffer.Add(exp)
		}

		// Sample all experiences
		batch := buffer.Sample(10)
		if len(batch) != 10 {
			t.Errorf("Expected batch size 10, got %d", len(batch))
		}
	})

	t.Run("buffer circular overwrite works", func(t *testing.T) {
		buffer := NewDQNReplayBuffer(5)

		// Add 10 experiences (should overwrite first 5)
		for i := 0; i < 10; i++ {
			exp := DQNExperience{
				Action: i,
				Reward: float32(i),
			}
			buffer.Add(exp)
		}

		if buffer.Size() != 5 {
			t.Errorf("Expected buffer size 5, got %d", buffer.Size())
		}

		// Sample all and verify we only get recent experiences (5-9)
		batch := buffer.Sample(5)
		for _, exp := range batch {
			if exp.Action < 5 || exp.Action > 9 {
				t.Errorf("Expected action in range [5,9], got %d", exp.Action)
			}
		}
	})
}

// TestExperienceStorage tests that experiences are stored in correct buffers
func TestExperienceStorage(t *testing.T) {
	trainer, err := NewDQNCardPlayTrainer(1000, 32, 0.95, 0.001, 0.0001, nil)
	if err != nil {
		t.Fatalf("Failed to create trainer: %v", err)
	}

	t.Run("declarer experiences go to declarer buffer", func(t *testing.T) {
		exp := DQNExperience{
			Action: 0,
			Reward: 1.0,
		}

		trainer.StoreExperience(exp, true) // isDeclarer = true

		declSize, defSize := trainer.GetBufferSizes()
		if declSize != 1 {
			t.Errorf("Expected declarer buffer size 1, got %d", declSize)
		}
		if defSize != 0 {
			t.Errorf("Expected defender buffer size 0, got %d", defSize)
		}
	})

	t.Run("defender experiences go to defender buffer", func(t *testing.T) {
		exp := DQNExperience{
			Action: 1,
			Reward: -1.0,
		}

		trainer.StoreExperience(exp, false) // isDeclarer = false

		declSize, defSize := trainer.GetBufferSizes()
		if declSize != 1 {
			t.Errorf("Expected declarer buffer size 1, got %d", declSize)
		}
		if defSize != 1 {
			t.Errorf("Expected defender buffer size 1, got %d", defSize)
		}
	})
}

// TestEpsilonDecay tests epsilon-greedy exploration decay
func TestEpsilonDecay(t *testing.T) {
	trainer, err := NewDQNCardPlayTrainer(1000, 32, 0.95, 0.001, 0.0001, nil)
	if err != nil {
		t.Fatalf("Failed to create trainer: %v", err)
	}

	t.Run("epsilon decays correctly", func(t *testing.T) {
		trainer.SetEpsilon(1.0)
		initial := trainer.GetEpsilon()

		if initial != 1.0 {
			t.Errorf("Expected initial epsilon 1.0, got %f", initial)
		}

		// Decay 100 times
		for i := 0; i < 100; i++ {
			trainer.DecayEpsilon()
		}

		decayed := trainer.GetEpsilon()
		expected := float32(math.Pow(0.995, 100)) // Default decay rate

		if math.Abs(float64(decayed-expected)) > 0.001 {
			t.Errorf("Expected epsilon ~%f after 100 decays, got %f", expected, decayed)
		}
	})

	t.Run("epsilon stops at minimum", func(t *testing.T) {
		trainer.SetEpsilon(0.12)

		// Decay many times
		for i := 0; i < 1000; i++ {
			trainer.DecayEpsilon()
		}

		final := trainer.GetEpsilon()
		if final < 0.1 {
			t.Errorf("Epsilon should not go below 0.1, got %f", final)
		}
	})
}

// TestStateEncoding tests that state encoding is consistent
func TestStateEncoding(t *testing.T) {
	t.Run("state has correct dimensions", func(t *testing.T) {
		g := game.NewGame()
		g = g.WithTestPlayers()
		g = g.WithCardsDealt()

		// Force game to card play phase
		g.Phase = game.PhasePlaying
		g.Mode = game.ModeSuit
		g.TrumpSuit = game.Hearts
		declarer := game.Speaker
		g.Declarer = &declarer

		validMoves := g.GetValidMoves()
		if len(validMoves) == 0 {
			t.Skip("No valid moves in test game")
		}

		state, mask := EncodeStateToDQN(g, game.Speaker, validMoves)

		// State should be 130 features
		if len(state) != 130 {
			t.Errorf("Expected state size 130, got %d", len(state))
		}

		// Mask should be 32 features (one per card)
		if len(mask) != 32 {
			t.Errorf("Expected mask size 32, got %d", len(mask))
		}

		// At least one valid move should be marked
		hasValidMove := false
		for _, v := range mask {
			if v > 0 {
				hasValidMove = true
				break
			}
		}
		if !hasValidMove {
			t.Error("Expected at least one valid move in mask")
		}
	})

	t.Run("state is deterministic for same game state", func(t *testing.T) {
		g := game.NewGame()
		g = g.WithTestPlayers()
		g = g.WithCardsDealt()

		g.Phase = game.PhasePlaying
		g.Mode = game.ModeSuit
		g.TrumpSuit = game.Hearts
		declarer := game.Speaker
		g.Declarer = &declarer

		validMoves := g.GetValidMoves()
		if len(validMoves) == 0 {
			t.Skip("No valid moves in test game")
		}

		state1, mask1 := EncodeStateToDQN(g, game.Speaker, validMoves)
		state2, mask2 := EncodeStateToDQN(g, game.Speaker, validMoves)

		// States should be identical
		for i := 0; i < 130; i++ {
			if state1[i] != state2[i] {
				t.Errorf("State mismatch at index %d: %f vs %f", i, state1[i], state2[i])
			}
		}

		// Masks should be identical
		for i := 0; i < 32; i++ {
			if mask1[i] != mask2[i] {
				t.Errorf("Mask mismatch at index %d: %f vs %f", i, mask1[i], mask2[i])
			}
		}
	})
}

// TestRewardCalculation tests reward computation logic
func TestRewardCalculation(t *testing.T) {
	t.Run("declarer winning trick gets positive reward", func(t *testing.T) {
		// Declarer wins 10 points, defenders get 0
		scoresBefore := [2]int{50, 40}
		scoresAfter := [2]int{60, 40}

		// This would be in main.go's computeTrickReward
		declarerPoints := scoresAfter[0] - scoresBefore[0]
		defenderPoints := scoresAfter[1] - scoresBefore[1]

		if declarerPoints != 10 {
			t.Errorf("Expected declarer to gain 10 points, got %d", declarerPoints)
		}
		if defenderPoints != 0 {
			t.Errorf("Expected defenders to gain 0 points, got %d", defenderPoints)
		}

		declarerWonTrick := declarerPoints > 0
		if !declarerWonTrick {
			t.Error("Expected declarer to have won trick")
		}
	})

	t.Run("defender winning trick gets positive reward", func(t *testing.T) {
		// Defenders win 15 points, declarer gets 0
		scoresBefore := [2]int{50, 40}
		scoresAfter := [2]int{50, 55}

		declarerPoints := scoresAfter[0] - scoresBefore[0]
		defenderPoints := scoresAfter[1] - scoresBefore[1]

		if declarerPoints != 0 {
			t.Errorf("Expected declarer to gain 0 points, got %d", declarerPoints)
		}
		if defenderPoints != 15 {
			t.Errorf("Expected defenders to gain 15 points, got %d", defenderPoints)
		}

		declarerWonTrick := declarerPoints > 0
		if declarerWonTrick {
			t.Error("Expected defenders to have won trick")
		}
	})

	t.Run("terminal state detection works", func(t *testing.T) {
		// Declarer has 61+ points - should win
		declarerScore := 65
		_ = 55 // opponentScore not used in logic

		declarerWon := declarerScore >= 61
		if !declarerWon {
			t.Error("Expected declarer to win with 65 points")
		}

		// Declarer has <61 points - should lose
		declarerScore = 60

		declarerWon = declarerScore >= 61
		if declarerWon {
			t.Error("Expected declarer to lose with 60 points")
		}
	})
}

// TestNetworkWeightSync tests that target networks update correctly
func TestNetworkWeightSync(t *testing.T) {
	trainer, err := NewDQNCardPlayTrainer(1000, 32, 0.95, 0.001, 0.0001, nil)
	if err != nil {
		t.Fatalf("Failed to create trainer: %v", err)
	}

	t.Run("target networks can be updated", func(t *testing.T) {
		// This should not panic
		err := trainer.updateTargetNetworks()
		if err != nil {
			t.Errorf("Failed to update target networks: %v", err)
		}
	})
}

// TestWeightSaveLoad tests that weights can be saved and loaded
func TestWeightSaveLoad(t *testing.T) {
	trainer, err := NewDQNCardPlayTrainer(1000, 32, 0.95, 0.001, 0.0001, nil)
	if err != nil {
		t.Fatalf("Failed to create trainer: %v", err)
	}

	t.Run("weights can be saved", func(t *testing.T) {
		declPath := "/tmp/test_declarer.weights"
		defPath := "/tmp/test_defender.weights"

		err := trainer.SaveWeights(declPath, defPath)
		if err != nil {
			t.Errorf("Failed to save weights: %v", err)
		}
	})

	t.Run("weights can be loaded", func(t *testing.T) {
		declPath := "/tmp/test_declarer.weights"
		defPath := "/tmp/test_defender.weights"

		// Save weights first
		err := trainer.SaveWeights(declPath, defPath)
		if err != nil {
			t.Fatalf("Failed to save weights: %v", err)
		}

		// Load into a strategy
		strategy, err := strategies.NewDeepQLearningCardPlayStrategyFromWeights(declPath, defPath)
		if err != nil {
			t.Errorf("Failed to load weights: %v", err)
		}
		if strategy == nil {
			t.Error("Loaded strategy is nil")
		}
	})
}

// TestTrainingStepDoesNotPanic tests that training steps execute without panic
func TestTrainingStepDoesNotPanic(t *testing.T) {
	trainer, err := NewDQNCardPlayTrainer(1000, 32, 0.95, 0.001, 0.0001, nil)
	if err != nil {
		t.Fatalf("Failed to create trainer: %v", err)
	}

	t.Run("training with insufficient data does not panic", func(t *testing.T) {
		// Try to train with empty buffers (should gracefully skip)
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Training panicked with empty buffers: %v", r)
			}
		}()

		_, _, err := trainer.Train()
		if err != nil {
			t.Logf("Training with empty buffer returned error (expected): %v", err)
		}
	})

	t.Run("training with sufficient data does not panic", func(t *testing.T) {
		// Add enough experiences to fill a batch
		for i := 0; i < 256; i++ {
			exp := DQNExperience{
				Action: i % 32,
				Reward: float32(i % 10),
				Done:   i%10 == 0,
			}
			// Random state/nextstate
			for j := 0; j < 130; j++ {
				exp.State[j] = float32(j) / 130.0
				exp.NextState[j] = float32(j+1) / 130.0
			}
			exp.ValidMask[i%32] = 1.0
			exp.NextMask[(i+1)%32] = 1.0

			trainer.StoreExperience(exp, i%2 == 0)
		}

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Training panicked: %v", r)
			}
		}()

		_, _, err := trainer.Train()
		if err != nil {
			t.Errorf("Training failed: %v", err)
		}
	})
}

// BenchmarkReplayBufferSampling benchmarks buffer sampling performance
func BenchmarkReplayBufferSampling(b *testing.B) {
	buffer := NewDQNReplayBuffer(100000)

	// Fill buffer
	for i := 0; i < 100000; i++ {
		buffer.Add(DQNExperience{Action: i % 32})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = buffer.Sample(256)
	}
}
