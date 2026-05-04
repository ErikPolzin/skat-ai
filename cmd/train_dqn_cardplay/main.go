package main

import (
	"flag"
	"fmt"
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
//   - Declarer network: learns to maximize declarer's score (needs 61+ points to win)
//   - Defender network: learns to prevent declarer from reaching 61 points
//   - Each player uses the appropriate network based on their role
//
// 2. Self-Play Dynamics:
//   - All 3 players use DQN agents with the same networks
//   - Declarer uses the declarer network, both defenders use the defender network
//   - Networks are synced every N episodes to incorporate latest learning
//   - Epsilon-greedy exploration ensures diverse experiences
//
// 3. Experience Collection:
//   - All 3 players' experiences are collected from every game
//   - Rewards are assigned based on trick outcomes (positive if your team won)
//   - Terminal rewards based on game outcome (declarer wins if score >= 61)
//   - Separate replay buffers for declarer and defender experiences
//
// 4. Why This Works for Skat:
//   - Asymmetric game: declarer plays alone vs 2 defenders
//   - Self-play provides balanced training data for both roles
//   - Networks improve together: better defenders -> harder games for declarer -> better declarer
//   - Converges to Nash equilibrium where neither side can improve unilaterally
func main() {
	// Parse flags
	episodes := flag.Int("episodes", 10000, "Number of training episodes")
	bufferSize := flag.Int("buffer", 20000, "Replay buffer size")
	batchSize := flag.Int("batch", 256, "Batch size")
	gamma := flag.Float64("gamma", 0.7, "Discount factor")
	lr := flag.Float64("lr", 0.0003, "Learning rate")
	l2Reg := flag.Float64("l2", 0.0001, "L2 regularization")
	epsilonStart := flag.Float64("epsilon-start", 1.0, "Starting epsilon for exploration")
	epsilonEnd := flag.Float64("epsilon-end", 0.1, "Ending epsilon for exploration")
	epsilonDecay := flag.Float64("epsilon-decay", 0.999, "Epsilon decay rate per episode")
	trainSteps := flag.Int("train-steps", 4, "Training steps per episode")
	evalEvery := flag.Int("eval-every", 100, "Evaluate every N episodes")
	evalGames := flag.Int("eval-games", 200, "Number of games per evaluation")
	saveEvery := flag.Int("save-every", 5000, "Save model every N episodes")
	outputWeights := flag.String("output", ".data/models/dqn_cardplay.weights", "Output weights file")
	trainingSplit := flag.String("split", "40/30/30", "Training split: heuristic/mixed/selfplay (e.g., 70/20/10)")

	flag.Parse()

	// Parse training split
	heuristicPct, mixedPct, selfPlayPct, err := parseTrainingSplit(*trainingSplit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid training split: %v\n", err)
		os.Exit(1)
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
	fmt.Printf("  Epsilon: %.2f -> %.2f (decay: %.6f)\n", *epsilonStart, *epsilonEnd, *epsilonDecay)
	fmt.Printf("  Training Steps: %d per episode\n", *trainSteps)
	fmt.Printf("  Training Split: %d%% heuristic / %d%% mixed / %d%% self-play\n", heuristicPct, mixedPct, selfPlayPct)
	fmt.Printf("  Evaluation: every %d episodes\n", *evalEvery)
	fmt.Printf("  Save: every %d episodes\n", *saveEvery)
	fmt.Printf("  Output: %s\n", *outputWeights)
	fmt.Println()

	// Create DQN trainer (nil strategy = fresh random initialization, or use loaded weights)
	trainer, err := dqn.NewDQNCardPlayTrainer(
		*bufferSize,
		*batchSize,
		float32(*gamma),
		*lr,
		*l2Reg,
		float32(*epsilonStart),
		float32(*epsilonDecay),
		float32(*epsilonEnd),
		nil,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create trainer: %v\n", err)
		os.Exit(1)
	}

	// Training loop
	fmt.Println("Starting self-play training...")
	startTime := time.Now()
	totalGames := 0

	// Network sync frequency: update agents with latest weights every N episodes
	// - Too frequent: unstable training (chasing a moving target)
	// - Too rare: agents play against outdated opponents
	// - Sweet spot: 10-100 episodes depending on learning rate
	syncEvery := 40

	// Create DQN strategy with initial random weights
	declWeights, defWeights := trainer.GetOnlineWeights()
	dqnStrategy := strategies.NewNeuralCardPlayStrategyFromWeightMaps(declWeights, defWeights)
	dqnStrategy.SetExploration(trainer.GetEpsilon())

	// Use heuristics for bidding and game choice (not training these yet)
	weightedBidding := strategies.NewWeightedHeuristicBiddingStrategy()
	weightedBidding.SetBiddingThreshold(0.65)

	// Create single DQN agent (reuse same instance to preserve metrics)
	dqnAgent := agent.NewAgentWithStrategies(
		"DQN",
		weightedBidding,                      // Heuristic bidding
		&agent.HeuristicGameChoiceStrategy{}, // Heuristic game choice
		dqnStrategy,                          // DQN card play
	)
	dqnAgent.EnableMetrics()

	// Function to update the DQN strategy with latest weights
	updateDQNWeights := func() {
		// Get current weights from the online networks
		declWeights, defWeights := trainer.GetOnlineWeights()

		// Update the strategy's weights
		dqnStrategy.UpdateWeights(declWeights, defWeights)
		dqnStrategy.SetExploration(trainer.GetEpsilon())
	}

	// Create heuristic agent for mixed training
	heuristicAgent := agent.NewHeuristicAgent("Heuristic")

	for episode := 1; episode <= *episodes; episode++ {
		// Periodically sync agents with latest network weights
		// This ensures agents play with the latest trained version
		if episode%syncEvery == 0 {
			updateDQNWeights()
		}

		// Determine episode type based on training split
		episodeType := determineEpisodeType(episode, heuristicPct, mixedPct, selfPlayPct)
		playGameWithMode(dqnAgent, heuristicAgent, trainer, episode, episodeType)

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

			// Get metrics from single DQN agent
			metrics := dqnAgent.GetMetrics()

			// Calculate declarer win rate
			declWinRate := 0.0
			if metrics.Games > 0 {
				declWinRate = float64(metrics.Wins) / float64(metrics.Games) * 100
			}

			// Calculate defender win rate
			defWinRate := 0.0
			if metrics.DefenderGames > 0 {
				defWinRate = float64(metrics.DefenderWins) / float64(metrics.DefenderGames) * 100
			}

			fmt.Printf("  [Ep %4d/%d] Eps=%.3f Buf=(%d,%d) Decl=%.1f%% (%d/%d) Def=%.1f%% (%d/%d)\n",
				episode, *episodes, trainer.GetEpsilon(), declBufSize, defBufSize,
				declWinRate, metrics.Wins, metrics.Games,
				defWinRate, metrics.DefenderWins, metrics.DefenderGames)

			// Reset metrics for next window
			dqnAgent.ResetMetrics()
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
			testStrategy := strategies.NewNeuralCardPlayStrategyFromWeightMaps(declWeights, defWeights)
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

			// Run evaluation with 50/50 split (test bidding, even declarer/defender cardplay)
			config := training.NewFiftyFiftySplitConfig(testAgent, baselineAgent, 0)
			stats := training.EvaluateAgents(config, *evalGames)

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

// playGameAndCollectExperiences plays a game with 50/50 declarer/defender split and collects DQN experiences
func playGameAndCollectExperiences(dqnAgent, heuristicAgent *agent.SkatAgent, trainer *dqn.DQNCardPlayTrainer, episode int) {
	// Create game
	g := game.NewGame()
	g = g.WithTestPlayers()
	g = g.WithCardsDealt()

	// Set up agents based on 50/50 split - all heuristic agents bid
	agents := [3]*agent.SkatAgent{
		heuristicAgent,
		heuristicAgent.CachedClone(),
		heuristicAgent.CachedClone().CachedClone(),
	}

	// Bidding phase
	for g.Phase == game.PhaseBidding {
		currentAgent := agents[g.CurrentPlayer]
		accept := currentAgent.Bid(g)
		if _, err := g.Bid(accept); err != nil {
			panic(fmt.Sprintf("Bidding error: %v", err))
		}
	}

	// Check if game was passed
	if g.Declarer == nil {
		panic("Declarer is nil after bidding")
	}

	// After bidding, swap agents for cardplay based on episode number
	// Even episodes: DQN as declarer, Odd episodes: heuristic as declarer
	declarerPos := int(*g.Declarer)
	if episode%2 == 0 {
		// DQN as declarer
		agents[declarerPos] = dqnAgent
		agents[(declarerPos+1)%3] = heuristicAgent
		agents[(declarerPos+2)%3] = heuristicAgent.CachedClone()
	} else {
		// Heuristic as declarer
		agents[declarerPos] = heuristicAgent
		agents[(declarerPos+1)%3] = dqnAgent
		agents[(declarerPos+2)%3] = dqnAgent.CachedClone()
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
	var trickExperiences [3]*GameExperience  // Pending experiences for current trick
	var previousTrickExps [3]*GameExperience // Previous trick's experiences (need NextState filled)

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

						// No terminal bonus - maximizing trick rewards naturally maximizes game wins
						// The agent should learn that collecting points in tricks leads to winning games

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

	// Record game results for DQN agent (for metrics tracking)
	if g.Phase == game.PhaseComplete && g.Declarer != nil {
		playerResults := g.PlayerResults()
		if playerResults != nil {
			// Record for all positions where DQN agent is present
			for pos := 0; pos < 3; pos++ {
				if agents[pos] == dqnAgent || agents[pos] == dqnAgent.CachedClone() || agents[pos] == dqnAgent.CachedClone().CachedClone() {
					dqnAgent.RecordGameResult(g, playerResults[pos])
				}
			}
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

	// Normalize trick value to roughly [-0.5, 0.5]
	// Moderate rewards for stable learning
	// Typical trick: 10-15 points -> 0.17-0.25 reward
	// Max trick: 30 points -> 0.5 reward
	normalized := float32(trickValue) / 30.0 // Max trick value ~30
	scaled := normalized * 0.5

	// Assign rewards: positive if your team won, negative if opponent won
	if declarerWonTrick {
		rewards[true] = scaled   // Declarer gets positive reward
		rewards[false] = -scaled // Defenders get negative reward
	} else {
		rewards[true] = -scaled // Declarer gets negative reward
		rewards[false] = scaled // Defenders get positive reward
	}

	return rewards
}

// cardToIndex converts a card to its index (0-31)
func cardToIndex(card game.Card) int {
	return encoding.CardToIndex(card)
}

// parseTrainingSplit parses a training split string like "70/20/10" into percentages
func parseTrainingSplit(split string) (heuristic, mixed, selfPlay int, err error) {
	var h, m, s int
	n, scanErr := fmt.Sscanf(split, "%d/%d/%d", &h, &m, &s)
	if scanErr != nil || n != 3 {
		return 0, 0, 0, fmt.Errorf("expected format: X/Y/Z (e.g., 70/20/10)")
	}
	if h < 0 || m < 0 || s < 0 {
		return 0, 0, 0, fmt.Errorf("percentages must be non-negative")
	}
	if h+m+s != 100 {
		return 0, 0, 0, fmt.Errorf("percentages must sum to 100, got %d", h+m+s)
	}
	return h, m, s, nil
}

// EpisodeType represents the type of training episode
type EpisodeType int

const (
	EpisodeHeuristic EpisodeType = iota // DQN vs all heuristic
	EpisodeMixed                        // DQN vs heuristic (50/50 declarer)
	EpisodeSelfPlay                     // DQN vs DQN
)

// determineEpisodeType determines what type of episode to run based on the split percentages
func determineEpisodeType(episode, heuristicPct, mixedPct, selfPlayPct int) EpisodeType {
	// Use modulo to cycle through percentages
	pos := (episode - 1) % 100

	if pos < heuristicPct {
		return EpisodeHeuristic
	} else if pos < heuristicPct+mixedPct {
		return EpisodeMixed
	} else {
		return EpisodeSelfPlay
	}
}

// playGameWithMode plays a game with the specified training mode
func playGameWithMode(dqnAgent, heuristicAgent *agent.SkatAgent, trainer *dqn.DQNCardPlayTrainer, episode int, mode EpisodeType) {
	switch mode {
	case EpisodeHeuristic:
		playGameHeuristicOnly(dqnAgent, heuristicAgent, trainer)
	case EpisodeMixed:
		playGameAndCollectExperiences(dqnAgent, heuristicAgent, trainer, episode)
	case EpisodeSelfPlay:
		playGameSelfPlay(dqnAgent, trainer)
	}
}

// playGameHeuristicOnly plays a game where all agents are heuristic (DQN observes)
// This is used to seed the replay buffer with diverse experiences
func playGameHeuristicOnly(dqnAgent, heuristicAgent *agent.SkatAgent, trainer *dqn.DQNCardPlayTrainer) {
	// Create game
	g := game.NewGame()
	g = g.WithTestPlayers()
	g = g.WithCardsDealt()

	// All heuristic agents
	agents := [3]*agent.SkatAgent{
		heuristicAgent,
		heuristicAgent.CachedClone(),
		heuristicAgent.CachedClone().CachedClone(),
	}

	// Bidding phase
	for g.Phase == game.PhaseBidding {
		currentAgent := agents[g.CurrentPlayer]
		accept := currentAgent.Bid(g)
		if _, err := g.Bid(accept); err != nil {
			panic(fmt.Sprintf("Bidding error: %v", err))
		}
	}

	if g.Declarer == nil {
		panic("Declarer is nil after bidding")
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

	// Card play phase - collect experiences from heuristic play
	var trickExperiences [3]*GameExperience
	var previousTrickExps [3]*GameExperience

	for g.Phase == game.PhasePlaying {
		validMoves := g.GetValidMoves()
		currentPlayer := g.CurrentPlayer
		playerIdx := int(currentPlayer)

		if previousTrickExps[playerIdx] != nil && !previousTrickExps[playerIdx].Experience.Done {
			state, validMask := dqn.EncodeStateToDQN(g, currentPlayer, validMoves)
			previousTrickExps[playerIdx].Experience.NextState = state
			previousTrickExps[playerIdx].Experience.NextMask = validMask
			trainer.StoreExperience(previousTrickExps[playerIdx].Experience, previousTrickExps[playerIdx].IsDeclarer)
			previousTrickExps[playerIdx] = nil
		}

		state, validMask := dqn.EncodeStateToDQN(g, currentPlayer, validMoves)
		card := agents[currentPlayer].SelectMove(g, validMoves)
		action := cardToIndex(card)
		isDeclarer := currentPlayer == *g.Declarer

		trickExperiences[playerIdx] = &GameExperience{
			Experience: dqn.DQNExperience{
				State:     state,
				Action:    action,
				ValidMask: validMask,
				Reward:    0,
			},
			IsDeclarer: isDeclarer,
			PlayerPos:  currentPlayer,
		}

		if _, err := g.PlayCard(card); err != nil {
			panic(fmt.Sprintf("PlayCard error: %v", err))
		}

		if len(g.Trick) == 3 {
			scoresBefore := [2]int{g.DeclarerScore, g.OpponentScore}
			if _, err := g.ResolveTrick(); err != nil {
				panic(fmt.Sprintf("ResolveTrick error: %v", err))
			}
			scoresAfter := [2]int{g.DeclarerScore, g.OpponentScore}
			trickReward := computeTrickReward(scoresBefore, scoresAfter)

			for i := 0; i < 3; i++ {
				if trickExperiences[i] != nil {
					playerIsDeclarer := trickExperiences[i].IsDeclarer
					trickExperiences[i].Experience.Reward = trickReward[playerIsDeclarer]

					if g.Phase != game.PhasePlaying {
						trickExperiences[i].Experience.Done = true
						trainer.StoreExperience(trickExperiences[i].Experience, trickExperiences[i].IsDeclarer)
					} else {
						previousTrickExps[i] = trickExperiences[i]
					}
				}
			}
			trickExperiences = [3]*GameExperience{}
		}
	}
}

// playGameSelfPlay plays a game where all agents are DQN
func playGameSelfPlay(dqnAgent *agent.SkatAgent, trainer *dqn.DQNCardPlayTrainer) {
	// Create game
	g := game.NewGame()
	g = g.WithTestPlayers()
	g = g.WithCardsDealt()

	// All DQN agents
	agents := [3]*agent.SkatAgent{
		dqnAgent,
		dqnAgent.CachedClone(),
		dqnAgent.CachedClone().CachedClone(),
	}

	// Bidding phase - use heuristic bidding
	for g.Phase == game.PhaseBidding {
		currentAgent := agents[g.CurrentPlayer]
		accept := currentAgent.Bid(g)
		if _, err := g.Bid(accept); err != nil {
			panic(fmt.Sprintf("Bidding error: %v", err))
		}
	}

	if g.Declarer == nil {
		panic("Declarer is nil after bidding")
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

	// Card play phase
	var trickExperiences [3]*GameExperience
	var previousTrickExps [3]*GameExperience

	for g.Phase == game.PhasePlaying {
		validMoves := g.GetValidMoves()
		currentPlayer := g.CurrentPlayer
		playerIdx := int(currentPlayer)

		if previousTrickExps[playerIdx] != nil && !previousTrickExps[playerIdx].Experience.Done {
			state, validMask := dqn.EncodeStateToDQN(g, currentPlayer, validMoves)
			previousTrickExps[playerIdx].Experience.NextState = state
			previousTrickExps[playerIdx].Experience.NextMask = validMask
			trainer.StoreExperience(previousTrickExps[playerIdx].Experience, previousTrickExps[playerIdx].IsDeclarer)
			previousTrickExps[playerIdx] = nil
		}

		state, validMask := dqn.EncodeStateToDQN(g, currentPlayer, validMoves)
		card := agents[currentPlayer].SelectMove(g, validMoves)
		action := cardToIndex(card)
		isDeclarer := currentPlayer == *g.Declarer

		trickExperiences[playerIdx] = &GameExperience{
			Experience: dqn.DQNExperience{
				State:     state,
				Action:    action,
				ValidMask: validMask,
				Reward:    0,
			},
			IsDeclarer: isDeclarer,
			PlayerPos:  currentPlayer,
		}

		if _, err := g.PlayCard(card); err != nil {
			panic(fmt.Sprintf("PlayCard error: %v", err))
		}

		if len(g.Trick) == 3 {
			scoresBefore := [2]int{g.DeclarerScore, g.OpponentScore}
			if _, err := g.ResolveTrick(); err != nil {
				panic(fmt.Sprintf("ResolveTrick error: %v", err))
			}
			scoresAfter := [2]int{g.DeclarerScore, g.OpponentScore}
			trickReward := computeTrickReward(scoresBefore, scoresAfter)

			for i := 0; i < 3; i++ {
				if trickExperiences[i] != nil {
					playerIsDeclarer := trickExperiences[i].IsDeclarer
					trickExperiences[i].Experience.Reward = trickReward[playerIsDeclarer]

					if g.Phase != game.PhasePlaying {
						trickExperiences[i].Experience.Done = true
						trainer.StoreExperience(trickExperiences[i].Experience, trickExperiences[i].IsDeclarer)
					} else {
						previousTrickExps[i] = trickExperiences[i]
					}
				}
			}
			trickExperiences = [3]*GameExperience{}
		}
	}

	// Record game results
	if g.Phase == game.PhaseComplete && g.Declarer != nil {
		playerResults := g.PlayerResults()
		if playerResults != nil {
			for pos := 0; pos < 3; pos++ {
				dqnAgent.RecordGameResult(g, playerResults[pos])
			}
		}
	}
}
