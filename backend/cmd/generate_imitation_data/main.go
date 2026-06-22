package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"sync"

	"skat/agent"
	"skat/agent/strategies"
	"skat/agent/strategies/encoding"
	"skat/game"
)

// ImitationExample represents a single (state, action) pair for supervised learning
type ImitationExample struct {
	State      [encoding.StateFeatureSize]float32 // DQN state encoding
	ValidMask  [32]float32                        // Valid moves at this state
	Action     int                                // Card index that expert chose
	IsDeclarer bool                               // Role (for separate networks)
	Policy     [32]float32                        // Soft target policy from expert scores
}

func newSearchTeacherAgent(name string, depth int, biddingThreshold float64) *agent.SkatAgent {
	config := strategies.DefaultContractEvaluatorConfig()
	config.MinWinProbability = biddingThreshold

	return agent.NewAgentWithStrategies(
		name,
		strategies.NewHeuristicBiddingStrategyWithConfig(config),
		strategies.NewHeuristicGameChoiceStrategyWithConfig(config),
		strategies.NewPerfectInfoMinimaxStrategyWithDepth(depth),
	)
}

func main() {
	numExamples := flag.Int("examples", 100000, "Number of examples to collect (per role: declarer and defender)")
	outputFile := flag.String("output", ".data/imitation_dataset.csv", "Output file for dataset")
	role := flag.String("role", "all", "Role to collect: all, declarer, or defender")
	searchDepth := flag.Int("depth", 7, "Minimax search depth for expert card-play labels (default: 7)")
	biddingThreshold := flag.Float64("bidding-threshold", 0.55, "Heuristic bidding threshold for contract generation; higher means stronger declarer hands")
	minHandStrength := flag.Float64("min-hand-strength", 0.55, "Minimum estimated declarer win probability to collect")
	maxHandStrength := flag.Float64("max-hand-strength", 0.75, "Maximum estimated declarer win probability to collect")
	workers := flag.Int("workers", runtime.NumCPU(), "Number of parallel workers")
	flag.Parse()

	if *role != "all" && *role != "declarer" && *role != "defender" {
		fmt.Fprintf(os.Stderr, "Unknown role %q (use all, declarer, or defender)\n", *role)
		os.Exit(1)
	}
	if *minHandStrength < 0 || *maxHandStrength > 1 || *minHandStrength > *maxHandStrength {
		fmt.Fprintf(os.Stderr, "Invalid hand-strength range %.2f-%.2f (expected 0 <= min <= max <= 1)\n", *minHandStrength, *maxHandStrength)
		os.Exit(1)
	}

	fmt.Printf("Generating imitation learning dataset with %d examples for role %s...\n", *numExamples, *role)
	fmt.Printf("  Declarer strategy: Minimax (depth %d)\n", *searchDepth)
	fmt.Printf("  Defender strategy: Minimax (depth %d)\n", *searchDepth)
	fmt.Printf("  Contract bidding: heuristic threshold %.2f\n", *biddingThreshold)
	fmt.Printf("  Filtering: won games with declarer hand strength %.2f-%.2f; excluding overbids\n", *minHandStrength, *maxHandStrength)
	fmt.Printf("Using %d parallel workers\n", *workers)

	// Channel for collecting results
	examplesChan := make(chan []ImitationExample, *workers)
	stopChan := make(chan bool) // Signal workers to stop
	var wg sync.WaitGroup

	// Progress tracking
	type ProgressUpdate struct {
		GamesPlayed      int
		DeclarerExamples int
		DefenderExamples int
	}
	progressChan := make(chan ProgressUpdate, *workers)
	doneChan := make(chan bool)

	// Progress reporter goroutine
	go func() {
		gamesPlayed := 0
		declarerCount := 0
		defenderCount := 0
		for update := range progressChan {
			gamesPlayed += update.GamesPlayed
			declarerCount += update.DeclarerExamples
			defenderCount += update.DefenderExamples
			if gamesPlayed%100 == 0 {
				fmt.Printf("  Played %d games -> %d declarer, %d defender examples\n",
					gamesPlayed, declarerCount, defenderCount)
			}
		}
		doneChan <- true
	}()

	// Worker function - collect until we have enough examples
	worker := func() {
		defer wg.Done()

		// Create search agent for expert card-play labels.
		searchAgent := newSearchTeacherAgent("SearchExpert", *searchDepth, *biddingThreshold)

		config := strategies.DefaultContractEvaluatorConfig()
		config.MinWinProbability = *biddingThreshold

		// Heuristic opponents face the Minimax teacher in each role-specific replay.
		heuristicAgent := agent.NewAgentWithStrategies(
			"HeuristicDefender",
			strategies.NewHeuristicBiddingStrategyWithConfig(config),
			strategies.NewHeuristicGameChoiceStrategyWithConfig(config),
			agent.NewHeuristicCardPlayStrategy(),
		)

		// Keep generating games until we signal stop
		for {
			select {
			case <-stopChan:
				return
			default:
				examples := playGameAndCollectExamples(searchAgent, heuristicAgent, *role, *minHandStrength, *maxHandStrength)
				if len(examples) > 0 {
					// Try to send, but stop if channel is closed
					select {
					case examplesChan <- examples:
						// Count declarer vs defender examples
						declCount := 0
						defCount := 0
						for _, ex := range examples {
							if ex.IsDeclarer {
								declCount++
							} else {
								defCount++
							}
						}
						select {
						case progressChan <- ProgressUpdate{
							GamesPlayed:      1,
							DeclarerExamples: declCount,
							DefenderExamples: defCount,
						}:
						case <-stopChan:
							return
						}
					case <-stopChan:
						return
					}
				} else {
					// Deal or both replays were filtered out.
					select {
					case progressChan <- ProgressUpdate{GamesPlayed: 1}:
					case <-stopChan:
						return
					}
				}
			}
		}
	}

	// Start workers
	for i := 0; i < *workers; i++ {
		wg.Add(1)
		go worker()
	}

	// Collect results until we have enough examples
	var declarerExamples []ImitationExample
	var defenderExamples []ImitationExample

	for examples := range examplesChan {
		for _, ex := range examples {
			if ex.IsDeclarer && len(declarerExamples) < *numExamples {
				declarerExamples = append(declarerExamples, ex)
			} else if !ex.IsDeclarer && len(defenderExamples) < *numExamples {
				defenderExamples = append(defenderExamples, ex)
			}

			// Check after each example if we have enough of both
			if doneCollecting(*role, len(declarerExamples), len(defenderExamples), *numExamples) {
				break
			}
		}

		// Double-check after processing the batch
		if doneCollecting(*role, len(declarerExamples), len(defenderExamples), *numExamples) {
			break
		}
	}

	// Signal all workers to stop
	close(stopChan)

	// Wait for workers to finish
	wg.Wait()

	// Close channels
	close(examplesChan)
	close(progressChan)

	// Wait for progress reporter to finish
	<-doneChan

	// Create balanced dataset (should already be at exact count)
	dataset := make([]ImitationExample, 0, len(declarerExamples)+len(defenderExamples))
	dataset = append(dataset, declarerExamples...)
	dataset = append(dataset, defenderExamples...)

	actualDeclarer := len(declarerExamples)
	actualDefender := len(defenderExamples)

	fmt.Printf("\nCollected dataset: %d declarer + %d defender = %d total examples\n",
		actualDeclarer, actualDefender, len(dataset))

	// Save dataset to CSV file
	fmt.Printf("\nSaving %d examples to %s...\n", len(dataset), *outputFile)

	// Ensure directory exists
	os.MkdirAll(".data", 0755)

	file, err := os.Create(*outputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create output file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := make([]string, 0, encoding.StateFeatureSize+32+2+32)
	for i := 0; i < encoding.StateFeatureSize; i++ {
		header = append(header, fmt.Sprintf("s%d", i))
	}
	for i := 0; i < 32; i++ {
		header = append(header, fmt.Sprintf("m%d", i))
	}
	header = append(header, "action", "is_declarer")
	for i := 0; i < 32; i++ {
		header = append(header, fmt.Sprintf("p%d", i))
	}
	if err := writer.Write(header); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write header: %v\n", err)
		os.Exit(1)
	}

	// Write examples
	for _, ex := range dataset {
		record := make([]string, 0, encoding.StateFeatureSize+32+2+32)

		// State features
		for _, val := range ex.State {
			record = append(record, strconv.FormatFloat(float64(val), 'f', 6, 32))
		}

		// Valid mask (32)
		for _, val := range ex.ValidMask {
			record = append(record, strconv.FormatFloat(float64(val), 'f', 0, 32))
		}

		// Action and role
		record = append(record, strconv.Itoa(ex.Action))
		if ex.IsDeclarer {
			record = append(record, "1")
		} else {
			record = append(record, "0")
		}
		for _, val := range ex.Policy {
			record = append(record, strconv.FormatFloat(float64(val), 'f', 6, 32))
		}

		if err := writer.Write(record); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write record: %v\n", err)
			os.Exit(1)
		}
	}

	// Print statistics
	declarerPct := float64(actualDeclarer) / float64(len(dataset)) * 100.0
	defenderPct := float64(actualDefender) / float64(len(dataset)) * 100.0

	fmt.Printf("\nDataset Statistics:\n")
	fmt.Printf("  Total examples: %d\n", len(dataset))
	fmt.Printf("  Declarer examples: %d (%.1f%%) - trained with Minimax\n", actualDeclarer, declarerPct)
	fmt.Printf("  Defender examples: %d (%.1f%%) - trained with Minimax\n", actualDefender, defenderPct)
	fmt.Printf("\n✓ Dataset generation complete!\n")
}

func doneCollecting(role string, declarers, defenders, target int) bool {
	switch role {
	case "declarer":
		return declarers >= target
	case "defender":
		return defenders >= target
	default:
		return declarers >= target && defenders >= target
	}
}

// setupGame creates a game, runs bidding, and returns the game state ready for card play
func setupGame(heuristicAgent *agent.SkatAgent) (*game.GameState, bool) {
	config := agent.NewThreeWayConfig(
		heuristicAgent,
		heuristicAgent.CachedClone(),
		heuristicAgent.CachedClone().CachedClone())
	// Create game
	g := game.NewGame()
	g = agent.WithAgentPlayers(g, config)
	g = g.WithCardsDealt()
	g = agent.WithAgentBidding(g, config)
	g = agent.WithAgentSkatDecision(g)
	return agent.WithAgentGameChoice(g)
}

// collectDeclarerExamples plays a game with search-teacher declarer vs heuristic defenders.
func collectDeclarerExamples(g *game.GameState, searchAgent, heuristicAgent *agent.SkatAgent) []ImitationExample {
	var examples []ImitationExample

	if g.Declarer == nil {
		return examples
	}

	declarer := *g.Declarer
	// Search-teacher declarer vs heuristic defenders
	agent.SetAgentForPlayer(g.GetPlayerByPosition(declarer), searchAgent)
	agent.SetAgentForPlayer(g.GetPlayerByPosition((declarer+1)%3), heuristicAgent)
	agent.SetAgentForPlayer(g.GetPlayerByPosition((declarer+2)%3), heuristicAgent)

	// Card play phase
	for g.Phase == game.PhasePlaying {
		validMoves := g.GetValidMoves()
		currentPlayer := g.CurrentPlayer
		currentAgent := agent.GetAgentForPlayer(g.GetCurrentPlayer())

		if currentPlayer == declarer {
			// Encode state
			enc := encoding.EncodeNeuralCardPlay(g, currentPlayer, validMoves)
			state := enc.ToSlice()
			validMask := enc.GetValidMask()

			// Get expert action
			card := currentAgent.SelectMove(g, validMoves)
			action := encoding.CardToIndex(card)
			policy := oneHotPolicy(action)

			// Store declarer example
			examples = append(examples, ImitationExample{
				State:      state,
				ValidMask:  validMask,
				Action:     action,
				IsDeclarer: true,
				Policy:     policy,
			})

			// Play card
			if _, err := g.PlayCard(card); err != nil {
				panic(fmt.Sprintf("PlayCard error: %v", err))
			}
		} else {
			// Opponent defender plays
			card := currentAgent.SelectMove(g, validMoves)
			if _, err := g.PlayCard(card); err != nil {
				panic(fmt.Sprintf("PlayCard error: %v", err))
			}
		}

		// Resolve trick if complete
		if len(g.Trick) == 3 {
			resolveTrickAndNotify(g)
		}
	}

	// Only imitate successful play. Keeping the examples buffered until the
	// result is known avoids teaching attractive-looking lines that lost.
	if !g.Result().DeclarerWon {
		return nil
	}
	return examples
}

// collectDefenderExamples plays a game with a heuristic declarer vs Minimax defenders.
func collectDefenderExamples(g *game.GameState, defenderSearchAgent, heuristicAgent *agent.SkatAgent) []ImitationExample {
	var examples []ImitationExample

	if g.Declarer == nil {
		return examples
	}

	declarer := *g.Declarer
	// Heuristic declarer vs Minimax defenders.
	agent.SetAgentForPlayer(g.GetPlayerByPosition(declarer), heuristicAgent)
	agent.SetAgentForPlayer(g.GetPlayerByPosition((declarer+1)%3), defenderSearchAgent)
	agent.SetAgentForPlayer(g.GetPlayerByPosition((declarer+2)%3), defenderSearchAgent.CachedClone())

	// Card play phase
	for g.Phase == game.PhasePlaying {
		validMoves := g.GetValidMoves()
		currentPlayer := g.CurrentPlayer

		if currentPlayer != declarer {
			// Encode state
			enc := encoding.EncodeNeuralCardPlay(g, currentPlayer, validMoves)
			state := enc.ToSlice()
			validMask := enc.GetValidMask()

			// Get expert defender action
			currentAgent := agent.GetAgentForPlayer(g.GetCurrentPlayer())
			card := currentAgent.SelectMove(g, validMoves)
			action := encoding.CardToIndex(card)
			policy := oneHotPolicy(action)

			// Store defender example
			examples = append(examples, ImitationExample{
				State:      state,
				ValidMask:  validMask,
				Action:     action,
				IsDeclarer: false,
				Policy:     policy,
			})

			// Play card
			if _, err := g.PlayCard(card); err != nil {
				panic(fmt.Sprintf("PlayCard error: %v", err))
			}
		} else {
			// Opponent declarer plays
			currentAgent := agent.GetAgentForPlayer(g.GetCurrentPlayer())
			card := currentAgent.SelectMove(g, validMoves)
			if _, err := g.PlayCard(card); err != nil {
				panic(fmt.Sprintf("PlayCard error: %v", err))
			}
		}

		// Resolve trick if complete
		if len(g.Trick) == 3 {
			resolveTrickAndNotify(g)
		}
	}

	// Defenders win as a team when the declarer loses.
	if g.Result().DeclarerWon {
		return nil
	}
	return examples
}

func resolveTrickAndNotify(g *game.GameState) {
	trick := append([]game.Card{}, g.Trick...)
	if _, err := g.ResolveTrick(); err != nil {
		panic(fmt.Sprintf("ResolveTrick error: %v", err))
	}
	for i := range g.Players {
		if g.Players[i].IsAgent {
			if agent := agent.GetAgentForPlayer(g.Players[i]); agent != nil {
				agent.OnTrickComplete(trick)
			}
		}
	}
}

func oneHotPolicy(action int) [32]float32 {
	var policy [32]float32
	if action >= 0 && action < len(policy) {
		policy[action] = 1.0
	}
	return policy
}

// playGameAndCollectExamples plays games twice: once for declarer examples, once for defender examples.
func playGameAndCollectExamples(searchAgent, heuristicAgent *agent.SkatAgent, role string, minHandStrength, maxHandStrength float64) []ImitationExample {
	var examples []ImitationExample

	// Setup game and run bidding once
	g, overbid := setupGame(heuristicAgent)

	if g.Declarer == nil || overbid {
		// No declarer or overbid, skip
		return examples
	}

	declarer := *g.Declarer
	handStrength := strategies.EstimateContractWinProbability(
		g.GetPlayerByPosition(declarer).Hand,
		g.Mode,
		g.TrumpSuit,
	)
	if handStrength < minHandStrength || handStrength > maxHandStrength {
		return examples
	}

	if role == "all" || role == "declarer" {
		// Collect declarer examples: search-teacher declarer vs heuristic defenders
		gDeclarer := g.Clone()
		declarerExamples := collectDeclarerExamples(gDeclarer, searchAgent, heuristicAgent)
		examples = append(examples, declarerExamples...)
	}

	if role == "all" || role == "defender" {
		// Collect defender examples: heuristic declarer vs Minimax defenders.
		gDefender := g.Clone()
		defenderExamples := collectDefenderExamples(gDefender, searchAgent, heuristicAgent)
		examples = append(examples, defenderExamples...)
	}

	return examples
}
