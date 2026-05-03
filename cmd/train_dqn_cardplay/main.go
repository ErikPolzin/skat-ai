package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"time"

	"skat/agent"
	"skat/agent/strategies"
	"skat/agent/strategies/encoding"
	"skat/agent/training"
	"skat/agent/training/dqn"
	"skat/game"
)

// DQN Self-Play Training for Skat Card Play
//
// This program trains a Dueling DQN agent to play Skat using self-play.
//
// Key aspects of the training approach:
//
// 1. Separate Networks for Declarer and Defender:
//    - Declarer network: learns to maximize declarer's score (needs 61+ points to win)
//    - Defender network: learns to prevent declarer from reaching 61 points
//    - Each player uses the appropriate network based on their role
//
// 2. Self-Play Dynamics:
//    - All 3 players use DQN agents with the same networks
//    - Declarer uses the declarer network, both defenders use the defender network
//    - Networks are synced every N episodes to incorporate latest learning
//    - Epsilon-greedy exploration ensures diverse experiences
//
// 3. Experience Collection:
//    - All 3 players' experiences are collected from every game
//    - Rewards are assigned based on trick outcomes (positive if your team won)
//    - Terminal rewards based on game outcome (declarer wins if score >= 61)
//    - Separate replay buffers for declarer and defender experiences
//
// 4. Why This Works for Skat:
//    - Asymmetric game: declarer plays alone vs 2 defenders
//    - Self-play provides balanced training data for both roles
//    - Networks improve together: better defenders -> harder games for declarer -> better declarer
//    - Converges to Nash equilibrium where neither side can improve unilaterally
//
func main() {
	// Parse flags
	episodes := flag.Int("episodes", 100000, "Number of training episodes")
	bufferSize := flag.Int("buffer", 100000, "Replay buffer size")
	batchSize := flag.Int("batch", 256, "Batch size")
	gamma := flag.Float64("gamma", 0.95, "Discount factor")
	lr := flag.Float64("lr", 0.0005, "Learning rate")
	l2Reg := flag.Float64("l2", 0.0001, "L2 regularization")
	epsilonStart := flag.Float64("epsilon-start", 1.0, "Starting epsilon for exploration")
	epsilonEnd := flag.Float64("epsilon-end", 0.1, "Ending epsilon for exploration")
	epsilonDecay := flag.Float64("epsilon-decay", 0.995, "Epsilon decay rate per episode")
	trainSteps := flag.Int("train-steps", 4, "Training steps per episode")
	evalEvery := flag.Int("eval-every", 1000, "Evaluate every N episodes")
	evalGames := flag.Int("eval-games", 100, "Number of games per evaluation")
	saveEvery := flag.Int("save-every", 5000, "Save model every N episodes")
	outputWeights := flag.String("output", ".data/models/dqn_cardplay.weights", "Output weights file")
	seed := flag.Int64("seed", 0, "Random seed (0 = use current time)")

	flag.Parse()

	// Set random seed
	if *seed == 0 {
		rand.Seed(time.Now().UnixNano())
	} else {
		rand.Seed(*seed)
	}

	fmt.Println("============================================================")
	fmt.Println("Skat DQN Card Play Training")
	fmt.Println("============================================================")
	fmt.Printf("\nConfiguration:\n")
	fmt.Printf("  Episodes: %d\n", *episodes)
	fmt.Printf("  Buffer Size: %d\n", *bufferSize)
	fmt.Printf("  Batch Size: %d\n", *batchSize)
	fmt.Printf("  Gamma: %.3f\n", *gamma)
	fmt.Printf("  Learning Rate: %.5f\n", *lr)
	fmt.Printf("  L2 Regularization: %.5f\n", *l2Reg)
	fmt.Printf("  Epsilon: %.2f -> %.2f (decay: %.4f)\n", *epsilonStart, *epsilonEnd, *epsilonDecay)
	fmt.Printf("  Training Steps: %d per episode\n", *trainSteps)
	fmt.Printf("  Evaluation: every %d episodes\n", *evalEvery)
	fmt.Printf("  Save: every %d episodes\n", *saveEvery)
	fmt.Printf("  Output: %s\n\n", *outputWeights)

	// Create DQN trainer (nil strategy = fresh random initialization)
	trainer, err := dqn.NewDQNCardPlayTrainer(
		*bufferSize,
		*batchSize,
		float32(*gamma),
		*lr,
		*l2Reg,
		nil,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create trainer: %v\n", err)
		os.Exit(1)
	}

	trainer.SetEpsilon(float32(*epsilonStart))

	// Training loop
	fmt.Println("Starting self-play training...")
	startTime := time.Now()
	totalGames := 0

	// Network sync frequency: update agents with latest weights every N episodes
	// - Too frequent: unstable training (chasing a moving target)
	// - Too rare: agents play against outdated opponents
	// - Sweet spot: 10-100 episodes depending on learning rate
	syncEvery := 10

	// Create initial DQN agents with random weights
	var dqnAgents [3]*agent.SkatAgent
	updateDQNAgents := func() {
		// Get current weights from the online networks
		declWeights, defWeights := trainer.GetOnlineWeights()

		// Create a shared strategy with current weights
		// All 3 agents share the same strategy instance (same networks)
		dqnStrategy := strategies.NewDeepQLearningCardPlayStrategyFromWeightMaps(declWeights, defWeights)
		dqnStrategy.SetExploration(trainer.GetEpsilon())

		// Use heuristics for bidding and game choice (not training these yet)
		weightedBidding := strategies.NewWeightedHeuristicBiddingStrategy()
		weightedBidding.SetBiddingThreshold(0.65)

		for i := 0; i < 3; i++ {
			dqnAgents[i] = agent.NewAgentWithStrategies(
				fmt.Sprintf("DQN-%d", i),
				weightedBidding,                      // Heuristic bidding
				&agent.HeuristicGameChoiceStrategy{}, // Heuristic game choice
				dqnStrategy,                          // DQN card play (shared)
			)
		}
	}
	updateDQNAgents() // Initialize with random weights

	for episode := 1; episode <= *episodes; episode++ {
		// Periodically sync agents with latest network weights
		// This ensures agents play against the latest version of themselves
		if episode%syncEvery == 0 {
			updateDQNAgents()
		}

		// Play a self-play game and collect experiences from ALL players
		playGameAndCollectExperiences(dqnAgents, trainer)

		// Perform training steps if we have enough data
		for step := 0; step < *trainSteps; step++ {
			_, _, err := trainer.Train()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Training error at episode %d: %v\n", episode, err)
			}
		}

		// Update exploration rate
		trainer.DecayEpsilon()

		// Track stats
		totalGames++

		// Show progress every 10 episodes
		if episode%10 == 0 {
			declBufSize, defBufSize := trainer.GetBufferSizes()
			fmt.Printf("  [Ep %4d/%d] Eps=%.3f Buf=(%d,%d)\n",
				episode, *episodes, trainer.GetEpsilon(), declBufSize, defBufSize)
		}

		// Evaluate periodically
		if episode%*evalEvery == 0 {
			declBufSize, defBufSize := trainer.GetBufferSizes()
			elapsed := time.Since(startTime)
			gamesPerSec := float64(totalGames) / elapsed.Seconds()

			fmt.Printf("[Ep %6d] Eps=%.3f Buf=(%d,%d) Games/s=%.1f\n",
				episode, trainer.GetEpsilon(), declBufSize, defBufSize, gamesPerSec)

			// Evaluate against heuristic baseline
			fmt.Printf("  Evaluating against heuristic baseline (%d games)...\n", *evalGames)

			// Create DQN test agent with current weights (no exploration)
			declWeights, defWeights := trainer.GetOnlineWeights()
			testStrategy := strategies.NewDeepQLearningCardPlayStrategyFromWeightMaps(declWeights, defWeights)
			testStrategy.SetExploration(0.0) // Pure exploitation for evaluation

			weightedBidding := strategies.NewWeightedHeuristicBiddingStrategy()
			weightedBidding.SetBiddingThreshold(0.65)

			testAgent := agent.NewAgentWithStrategies(
				"DQN-Test",
				weightedBidding,
				&agent.HeuristicGameChoiceStrategy{},
				testStrategy,
			)

			baselineAgent := agent.NewHeuristicAgent("Baseline")

			// Run evaluation
			stats := training.EvaluateAgents(testAgent, baselineAgent, *evalGames)

			// Calculate declarer win rates
			testDeclWinRate := 0.0
			if stats.TestGames > 0 {
				testDeclWinRate = float64(stats.TestWins) / float64(stats.TestGames) * 100
			}
			baselineDeclWinRate := 0.0
			if stats.BaselineGames > 0 {
				baselineDeclWinRate = float64(stats.BaselineWins) / float64(stats.BaselineGames) * 100
			}

			// Calculate defender win rates (defender wins = opponent was declarer and lost)
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

			// Calculate game type win rates
			testGrandWinRate := 0.0
			if stats.TestGrandGames > 0 {
				testGrandWinRate = float64(stats.TestGrandWins) / float64(stats.TestGrandGames) * 100
			}
			testSuitWinRate := 0.0
			if stats.TestSuitGames > 0 {
				testSuitWinRate = float64(stats.TestSuitWins) / float64(stats.TestSuitGames) * 100
			}

			fmt.Printf("  → DQN:      Decl %.1f%% (%d/%d) | Def %.1f%% (%d/%d) | [Grand: %.1f%%, Suit: %.1f%%]\n",
				testDeclWinRate, stats.TestWins, stats.TestGames,
				testDefWinRate, testDefWins, testDefGames,
				testGrandWinRate, testSuitWinRate)
			fmt.Printf("  → Baseline: Decl %.1f%% (%d/%d) | Def %.1f%% (%d/%d)\n",
				baselineDeclWinRate, stats.BaselineWins, stats.BaselineGames,
				baselineDefWinRate, baselineDefWins, baselineDefGames)

			// Reset stats
			totalGames = 0
		}

		// Save checkpoint
		if episode%*saveEvery == 0 {
			declarerPath := fmt.Sprintf("%s.declarer.ep%d", *outputWeights, episode)
			defenderPath := fmt.Sprintf("%s.defender.ep%d", *outputWeights, episode)
			if err := trainer.SaveWeights(declarerPath, defenderPath); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to save checkpoint: %v\n", err)
			} else {
				fmt.Printf("  → Saved checkpoint: %s + %s\n", declarerPath, defenderPath)
			}
		}
	}

	// Save final model
	declarerPath := *outputWeights + ".declarer"
	defenderPath := *outputWeights + ".defender"
	if err := trainer.SaveWeights(declarerPath, defenderPath); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save final weights: %v\n", err)
		os.Exit(1)
	}

	elapsed := time.Since(startTime)
	fmt.Printf("\n✓ Training complete in %s\n", elapsed)
	fmt.Printf("✓ Declarer network saved to: %s\n", declarerPath)
	fmt.Printf("✓ Defender network saved to: %s\n", defenderPath)
}

// GameExperience wraps DQNExperience with role information
type GameExperience struct {
	Experience dqn.DQNExperience
	IsDeclarer bool
	PlayerPos  game.GamePosition
}

// playGameAndCollectExperiences plays a self-play game and collects DQN experiences from all players
func playGameAndCollectExperiences(agents [3]*agent.SkatAgent, trainer *dqn.DQNCardPlayTrainer) {
	// Create and play game
	g := game.NewGame()
	g = g.WithTestPlayers()
	g = g.WithCardsDealt()

	// Bidding phase
	for g.Phase == game.PhaseBidding {
		currentAgent := agents[g.CurrentPlayer]
		accept := currentAgent.Bid(g)
		if _, err := g.Bid(accept); err != nil {
			panic(fmt.Sprintf("Bidding error (should never happen in self-play): %v", err))
		}
	}

	// Check if game was passed
	if g.Declarer == nil {
		return // No card play experiences if everyone passed
	}

	// Skat exchange and game choice
	if g.Phase == game.PhaseSkatExchange {
		declarerAgent := agents[*g.Declarer]
		if _, err := g.SkatDecision(true); err != nil {
			panic(fmt.Sprintf("Skat decision error: %v", err))
		}
		mode, trumpSuit := declarerAgent.ChooseGame(g)
		card1, card2 := declarerAgent.ChooseSkatDiscard(g.Players[*g.Declarer].Hand, mode, trumpSuit)
		if _, err := g.Discard(card1, card2); err != nil {
			panic(fmt.Sprintf("Discard error: %v", err))
		}
		if _, err := g.DeclareGame(mode, trumpSuit, false, false); err != nil {
			panic(fmt.Sprintf("Declare game error: %v", err))
		}
	}

	// Card play phase - collect experiences for ALL players
	var trickExperiences [3]*GameExperience      // Pending experiences for current trick
	var previousTrickExps [3]*GameExperience     // Previous trick's experiences (need NextState filled)

	for g.Phase == game.PhasePlaying {
		validMoves := g.GetValidMoves()
		currentPlayer := g.CurrentPlayer
		playerIdx := int(currentPlayer)

		// Fill in NextState for this player's previous experience (if exists)
		if previousTrickExps[playerIdx] != nil && !previousTrickExps[playerIdx].Experience.Done {
			state, validMask := dqn.EncodeStateToDQN(g, currentPlayer, validMoves)
			previousTrickExps[playerIdx].Experience.NextState = state
			previousTrickExps[playerIdx].Experience.NextMask = validMask

			// Store the now-complete experience
			trainer.StoreExperience(previousTrickExps[playerIdx].Experience, previousTrickExps[playerIdx].IsDeclarer)
			previousTrickExps[playerIdx] = nil
		}

		// Get state encoding before move
		state, validMask := dqn.EncodeStateToDQN(g, currentPlayer, validMoves)

		// Agent selects card
		card := agents[currentPlayer].SelectMove(g, validMoves)
		action := cardToIndex(card)

		// Determine player's role
		isDeclarer := currentPlayer == *g.Declarer

		// Create pending experience for this player
		trickExperiences[playerIdx] = &GameExperience{
			Experience: dqn.DQNExperience{
				State:     state,
				Action:    action,
				ValidMask: validMask,
				Reward:    0, // Will be set when trick resolves
			},
			IsDeclarer: isDeclarer,
			PlayerPos:  currentPlayer,
		}

		// Play card
		if _, err := g.PlayCard(card); err != nil {
			panic(fmt.Sprintf("PlayCard error at pos %v: %v (valid moves: %v)", currentPlayer, err, validMoves))
		}

		// Resolve trick if complete
		if len(g.Trick) == 3 {
			// Store scores before resolution to determine winner
			scoresBefore := [2]int{g.DeclarerScore, g.OpponentScore}

			if _, err := g.ResolveTrick(); err != nil {
				panic(fmt.Sprintf("ResolveTrick error: %v", err))
			}

			// Compute trick reward based on actual winner
			scoresAfter := [2]int{g.DeclarerScore, g.OpponentScore}
			trickReward := computeTrickReward(scoresBefore, scoresAfter)

			// Assign rewards and handle terminal states
			for i := 0; i < 3; i++ {
				if trickExperiences[i] != nil {
					playerIsDeclarer := trickExperiences[i].IsDeclarer
					trickExperiences[i].Experience.Reward = trickReward[playerIsDeclarer]

					if g.Phase != game.PhasePlaying {
						// Game ended - mark as terminal
						trickExperiences[i].Experience.Done = true

						// Terminal reward: simple win/loss bonus added to final trick reward
						// This preserves the importance of point collection (via trick rewards)
						// while giving a clear signal about the game outcome
						declarerWon := g.DeclarerScore >= 61
						terminalBonus := float32(-3.0)
						if (playerIsDeclarer && declarerWon) || (!playerIsDeclarer && !declarerWon) {
							terminalBonus = 3.0
						}

						// Add terminal bonus to trick reward for final trick
						// Final trick gets: trick_reward (±0.5) + terminal_bonus (±3.0)
						trickExperiences[i].Experience.Reward += terminalBonus

						// Store complete terminal experience immediately
						trainer.StoreExperience(trickExperiences[i].Experience, trickExperiences[i].IsDeclarer)
					} else {
						// Game continues - move to previousTrickExps to await NextState
						previousTrickExps[i] = trickExperiences[i]
					}
				}
			}

			// Clear trick experiences for next trick
			trickExperiences = [3]*GameExperience{}
		}
	}
}

// computeTrickReward computes rewards for declarer and defenders based on who won the trick
// Returns map[isDeclarer]reward
func computeTrickReward(scoresBefore, scoresAfter [2]int) map[bool]float32 {
	rewards := make(map[bool]float32)

	// Calculate points won in this trick
	declarerPoints := scoresAfter[0] - scoresBefore[0]
	defenderPoints := scoresAfter[1] - scoresBefore[1]

	// Determine who won the trick
	declarerWonTrick := declarerPoints > 0
	trickValue := declarerPoints + defenderPoints

	// Normalize trick value to roughly [-1, 1]
	normalized := float32(trickValue) / 30.0 // Max trick value ~30

	// Scale down immediate rewards (terminal reward is more important)
	scaled := normalized * 0.5

	// Assign rewards: positive if your team won, negative if opponent won
	if declarerWonTrick {
		rewards[true] = scaled   // Declarer gets positive reward
		rewards[false] = -scaled // Defenders get negative reward
	} else {
		rewards[true] = -scaled // Declarer gets negative reward
		rewards[false] = scaled  // Defenders get positive reward
	}

	return rewards
}

// cardToIndex converts a card to its index (0-31)
func cardToIndex(card game.Card) int {
	return encoding.CardToIndex(card)
}
