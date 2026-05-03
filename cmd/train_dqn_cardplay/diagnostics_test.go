package main

import (
	"fmt"
	"math"
	"skat/agent"
	"skat/agent/strategies"
	"skat/agent/training/dqn"
	"skat/game"
	"testing"
)

// TestEpsilonSchedule checks if epsilon decay is appropriate
func TestEpsilonSchedule(t *testing.T) {
	trainer, _ := dqn.NewDQNCardPlayTrainer(10000, 256, 0.95, 0.0005, 0.0001, nil)
	trainer.SetEpsilon(1.0)

	// Track when epsilon reaches milestones
	episodes := []int{100, 250, 500, 1000, 2000, 5000}

	t.Log("Epsilon decay schedule (decay=0.995 per episode):")
	t.Logf("Episode %4d: eps=%.3f", 0, trainer.GetEpsilon())

	for _, target := range episodes {
		for trainer.GetEpsilon() > 0.1 {
			trainer.DecayEpsilon()
		}
		// Simulate more episodes
		currentEp := 0
		trainer.SetEpsilon(1.0)
		for currentEp < target {
			trainer.DecayEpsilon()
			currentEp++
		}
		t.Logf("Episode %4d: eps=%.3f", target, trainer.GetEpsilon())
	}

	// Find when eps reaches 0.1
	trainer.SetEpsilon(1.0)
	ep := 0
	for trainer.GetEpsilon() > 0.1001 {
		trainer.DecayEpsilon()
		ep++
	}
	t.Logf("\nEpsilon reaches 0.1 at episode %d", ep)
	t.Logf("For 5000 episode run, that's %.1f%% of training without exploration",
		float64(5000-ep)/50.0)
}

// TestNetworkOutputVariance checks if network outputs are too uniform
func TestNetworkOutputVariance(t *testing.T) {
	t.Run("fresh network has varied outputs", func(t *testing.T) {
		strategy := strategies.NewDeepQLearningCardPlayStrategy()
		strategy.SetExploration(0.0) // No epsilon

		// Create a test game
		g := game.NewGame()
		g = g.WithTestPlayers()
		g = g.WithCardsDealt()
		g.Phase = game.PhasePlaying
		g.Mode = game.ModeSuit
		g.TrumpSuit = game.Hearts
		declarer := game.Speaker
		g.Declarer = &declarer

		validMoves := g.GetValidMoves()
		if len(validMoves) < 3 {
			t.Skip("Need at least 3 valid moves for this test")
		}

		// Get multiple decisions
		choices := make(map[int]int)
		for i := 0; i < 100; i++ {
			card := strategy.SelectMove(g, validMoves)
			// Find card index
			for idx, c := range validMoves {
				if c == card {
					choices[idx]++
					break
				}
			}
		}

		// Check if network is deterministic (which it should be)
		if len(choices) != 1 {
			t.Errorf("Network should be deterministic, got %d different choices: %v",
				len(choices), choices)
		}

		t.Logf("Fresh network choices: %v", choices)
	})
}

// TestSelfPlayDiversity checks if self-play produces diverse games
func TestSelfPlayDiversity(t *testing.T) {
	t.Run("games have different outcomes", func(t *testing.T) {
		agents := [3]*agent.SkatAgent{
			agent.NewHeuristicAgent("P0"),
			agent.NewHeuristicAgent("P1"),
			agent.NewHeuristicAgent("P2"),
		}

		// Play 20 games and track outcomes
		declarerWins := 0
		defenderWins := 0

		for i := 0; i < 20; i++ {
			g := game.NewGame()
			g = g.WithTestPlayers()
			g = g.WithCardsDealt()

			// Quick play through
			for g.Phase == game.PhaseBidding {
				accept := agents[g.CurrentPlayer].Bid(g)
				g.Bid(accept)
			}

			if g.Declarer == nil {
				continue
			}

			if g.Phase == game.PhaseSkatExchange {
				declarerAgent := agents[*g.Declarer]
				g.SkatDecision(true)
				mode, trumpSuit := declarerAgent.ChooseGame(g)
				card1, card2 := declarerAgent.ChooseSkatDiscard(g.Players[*g.Declarer].Hand, mode, trumpSuit)
				g.Discard(card1, card2)
				g.DeclareGame(mode, trumpSuit, false, false)
			}

			for g.Phase == game.PhasePlaying {
				validMoves := g.GetValidMoves()
				card := agents[g.CurrentPlayer].SelectMove(g, validMoves)
				g.PlayCard(card)
				if len(g.Trick) == 3 {
					g.ResolveTrick()
				}
			}

			if g.DeclarerScore >= 61 {
				declarerWins++
			} else {
				defenderWins++
			}
		}

		t.Logf("Game outcomes: declarer=%d, defenders=%d", declarerWins, defenderWins)

		// Both should win some games
		if declarerWins == 0 || defenderWins == 0 {
			t.Logf("Warning: one side always wins - might indicate deterministic play")
		}
	})
}

// TestBufferTurnover checks if buffer has appropriate turnover
func TestBufferTurnover(t *testing.T) {
	bufferSize := 20000
	episodesPerGame := 10 // ~10 experiences per game
	totalEpisodes := 5000

	// Buffer fills at episode ~2000 (20000 / 10)
	fillEpisode := bufferSize / episodesPerGame

	// After buffer fills, how much turnover?
	remainingEpisodes := totalEpisodes - fillEpisode
	turnoverTimes := float64(remainingEpisodes*episodesPerGame) / float64(bufferSize)

	t.Logf("Buffer analysis:")
	t.Logf("  Buffer size: %d", bufferSize)
	t.Logf("  Experiences per game: ~%d", episodesPerGame)
	t.Logf("  Buffer fills at episode: ~%d", fillEpisode)
	t.Logf("  Remaining episodes: %d", remainingEpisodes)
	t.Logf("  Buffer turnover: %.1fx", turnoverTimes)

	if turnoverTimes < 1.0 {
		t.Logf("Warning: buffer only turns over %.1fx - old experiences persist", turnoverTimes)
	}
	if turnoverTimes > 5.0 {
		t.Logf("Warning: buffer turns over %.1fx - might be too fast", turnoverTimes)
	}
}

// TestLearningRateScale checks if learning rate is appropriate
func TestLearningRateScale(t *testing.T) {
	lr := 0.0005
	batchSize := 256
	_ = batchSize

	// Typical Q-value range
	typicalTrickReward := 0.2
	terminalBonus := 3.0
	maxQValue := terminalBonus + typicalTrickReward*10 // ~5.0

	// Typical gradient magnitude (rough estimate)
	typicalGradient := maxQValue * 0.1 // 10% of Q-value

	// Weight update per batch
	weightUpdate := lr * typicalGradient

	t.Logf("Learning rate analysis:")
	t.Logf("  Learning rate: %.4f", lr)
	t.Logf("  Batch size: %d", batchSize)
	t.Logf("  Estimated max Q-value: %.2f", maxQValue)
	t.Logf("  Estimated typical gradient: %.3f", typicalGradient)
	t.Logf("  Weight update per batch: %.5f", weightUpdate)
	t.Logf("  Updates per episode: 4")
	t.Logf("  Weight change per episode: %.5f", weightUpdate*4)

	// Rule of thumb: weight changes should be small (< 0.01 per episode)
	if weightUpdate*4 > 0.01 {
		t.Logf("Warning: learning rate might be too high")
	}
	if weightUpdate*4 < 0.0001 {
		t.Logf("Warning: learning rate might be too low")
	}
}

// TestTargetNetworkUpdateFrequency analyzes target network sync
func TestTargetNetworkUpdateFrequency(t *testing.T) {
	targetUpdateSteps := 100
	trainStepsPerEpisode := 4

	// Target updates every 100 training steps = every 25 episodes
	targetUpdateEpisodes := targetUpdateSteps / trainStepsPerEpisode

	// Agent sync every 10 episodes
	agentSyncEpisodes := 10

	t.Logf("Network synchronization analysis:")
	t.Logf("  Target network updates every %d training steps", targetUpdateSteps)
	t.Logf("  = every %d episodes", targetUpdateEpisodes)
	t.Logf("  Agent sync (for self-play) every %d episodes", agentSyncEpisodes)

	if agentSyncEpisodes < targetUpdateEpisodes {
		t.Logf("Info: agents sync more frequently than target network")
		t.Logf("  This is fine - agents should track online network")
	}

	// Check if target update frequency is appropriate
	if targetUpdateEpisodes < 10 {
		t.Logf("Warning: target updates very frequently (every %d eps) - might be unstable",
			targetUpdateEpisodes)
	}
	if targetUpdateEpisodes > 100 {
		t.Logf("Warning: target updates infrequently (every %d eps) - might lag too much",
			targetUpdateEpisodes)
	}
}

// TestRewardDistribution checks if rewards are well-distributed
func TestRewardDistribution(t *testing.T) {
	// Collect rewards from a few games
	rewards := []float32{}

	// Simulate trick rewards
	trickValues := []int{0, 5, 10, 15, 20, 25, 30}
	for _, val := range trickValues {
		normalized := float32(val) / 30.0
		scaled := normalized * 0.5
		rewards = append(rewards, scaled, -scaled)
	}

	// Add terminal rewards
	rewards = append(rewards, 3.0, -3.0)

	// Calculate statistics
	var sum, sumSq float32
	for _, r := range rewards {
		sum += r
		sumSq += r * r
	}
	mean := sum / float32(len(rewards))
	variance := (sumSq / float32(len(rewards))) - (mean * mean)
	stddev := float32(math.Sqrt(float64(variance)))

	t.Logf("Reward distribution:")
	t.Logf("  Mean: %.3f", mean)
	t.Logf("  Std dev: %.3f", stddev)
	t.Logf("  Range: [%.3f, %.3f]", -3.0-0.5, 3.0+0.5)

	// Mean should be close to 0 (zero-sum game)
	if math.Abs(float64(mean)) > 0.1 {
		t.Logf("Warning: rewards not centered around zero")
	}

	// Standard deviation should be reasonable (not too small or large)
	if stddev < 0.5 {
		t.Logf("Warning: low reward variance - might not provide enough learning signal")
	}
	if stddev > 3.0 {
		t.Logf("Warning: high reward variance - might cause instability")
	}
}

// Example showing hyperparameter interdependencies
func Example_hyperparameters() {
	fmt.Println("=== DQN Hyperparameter Analysis ===")
	fmt.Println()

	fmt.Println("Current settings:")
	fmt.Println("  Buffer: 20,000 experiences")
	fmt.Println("  Episodes: 5,000")
	fmt.Println("  Experiences/game: ~30 (10 tricks × 3 players)")
	fmt.Println("  Buffer fills at: episode ~667")
	fmt.Println()

	fmt.Println("Epsilon schedule:")
	fmt.Println("  Start: 1.0 → End: 0.1")
	fmt.Println("  Decay: 0.995 per episode")
	fmt.Println("  Reaches 0.1 at: episode ~460")
	fmt.Println("  % of training with eps=0.1: 90.8%")
	fmt.Println()

	fmt.Println("Potential issues:")
	fmt.Println("  1. Epsilon decays too fast - 90% of training has minimal exploration")
	fmt.Println("  2. After eps=0.1, buffer fills with similar strategies")
	fmt.Println("  3. Self-play agents sync every 10 eps - might cause instability")
	fmt.Println()

	fmt.Println("Recommendations:")
	fmt.Println("  - Slower epsilon decay: 0.998 (reaches 0.1 at ~2300 eps)")
	fmt.Println("  - Higher epsilon_min: 0.2 (maintain more exploration)")
	fmt.Println("  - Less frequent agent sync: every 50-100 episodes")
	fmt.Println("  - Lower learning rate: 0.0001-0.0002")

	// Output:
	// === DQN Hyperparameter Analysis ===
	//
	// Current settings:
	//   Buffer: 20,000 experiences
	//   Episodes: 5,000
	//   Experiences/game: ~30 (10 tricks × 3 players)
	//   Buffer fills at: episode ~667
	//
	// Epsilon schedule:
	//   Start: 1.0 → End: 0.1
	//   Decay: 0.995 per episode
	//   Reaches 0.1 at: episode ~460
	//   % of training with eps=0.1: 90.8%
	//
	// Potential issues:
	//   1. Epsilon decays too fast - 90% of training has minimal exploration
	//   2. After eps=0.1, buffer fills with similar strategies
	//   3. Self-play agents sync every 10 eps - might cause instability
	//
	// Recommendations:
	//   - Slower epsilon decay: 0.998 (reaches 0.1 at ~2300 eps)
	//   - Higher epsilon_min: 0.2 (maintain more exploration)
	//   - Less frequent agent sync: every 50-100 episodes
	//   - Lower learning rate: 0.0001-0.0002
}
