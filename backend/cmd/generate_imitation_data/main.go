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

func isSearchTeacher(name string) bool {
	return name == "minimax" || name == "minimax-heuristic"
}

func teacherDisplayName(name string) string {
	switch name {
	case "minimax":
		return "Minimax"
	case "minimax-heuristic":
		return "MinimaxVsHeuristic"
	default:
		return name
	}
}

func newSearchTeacherAgent(name, teacher string, depth int, biddingThreshold float64) *agent.SkatAgent {
	var cardPlay agent.CardPlayStrategy
	switch teacher {
	case "minimax-heuristic":
		cardPlay = strategies.NewPerfectInfoMinimaxVsHeuristicStrategyWithDepth(depth)
	default:
		cardPlay = strategies.NewPerfectInfoMinimaxStrategyWithDepth(depth)
	}

	config := strategies.DefaultContractEvaluatorConfig()
	config.MinWinProbability = biddingThreshold

	return agent.NewAgentWithStrategies(
		name,
		strategies.NewHeuristicBiddingStrategyWithConfig(config),
		strategies.NewHeuristicGameChoiceStrategyWithConfig(config),
		cardPlay,
	)
}

func main() {
	numExamples := flag.Int("examples", 100000, "Number of examples to collect (per role: declarer and defender)")
	outputFile := flag.String("output", ".data/imitation_dataset.csv", "Output file for dataset")
	searchDepth := flag.Int("depth", 7, "Minimax search depth for expert card-play labels (default: 7)")
	teacher := flag.String("teacher", "minimax", "Declarer label teacher: minimax or minimax-heuristic")
	defenderTeacher := flag.String("defender-teacher", "heuristic", "Defender label teacher: heuristic, minimax, or minimax-heuristic")
	biddingThreshold := flag.Float64("bidding-threshold", 0.55, "Heuristic bidding threshold for contract generation; higher means stronger declarer hands")
	workers := flag.Int("workers", runtime.NumCPU(), "Number of parallel workers")
	flag.Parse()

	if !isSearchTeacher(*teacher) {
		fmt.Fprintf(os.Stderr, "Unknown teacher %q (use minimax or minimax-heuristic)\n", *teacher)
		os.Exit(1)
	}
	if *defenderTeacher != "heuristic" && !isSearchTeacher(*defenderTeacher) {
		fmt.Fprintf(os.Stderr, "Unknown defender teacher %q (use heuristic, minimax, or minimax-heuristic)\n", *defenderTeacher)
		os.Exit(1)
	}

	fmt.Printf("Generating imitation learning dataset with %d examples per role...\n", *numExamples)
	fmt.Printf("  Declarer strategy: %s (depth %d)\n", teacherDisplayName(*teacher), *searchDepth)
	fmt.Printf("  Defender strategy: %s", teacherDisplayName(*defenderTeacher))
	if isSearchTeacher(*defenderTeacher) {
		fmt.Printf(" (depth %d)", *searchDepth)
	}
	fmt.Printf("\n")
	fmt.Printf("  Contract bidding: heuristic threshold %.2f\n", *biddingThreshold)
	fmt.Printf("  Filtering: Excluding overbid games\n")
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
		searchAgent := newSearchTeacherAgent("SearchExpert", *teacher, *searchDepth, *biddingThreshold)
		defenderSearchAgent := searchAgent
		if isSearchTeacher(*defenderTeacher) && *defenderTeacher != *teacher {
			defenderSearchAgent = newSearchTeacherAgent("DefenderSearchExpert", *defenderTeacher, *searchDepth, *biddingThreshold)
		}

		config := strategies.DefaultContractEvaluatorConfig()
		config.MinWinProbability = *biddingThreshold

		// Create heuristic agent for defender examples and opponent simulation
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
				examples := playGameAndCollectExamples(searchAgent, defenderSearchAgent, heuristicAgent, *defenderTeacher)
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
					// Game was filtered out (overbid)
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
			if len(declarerExamples) >= *numExamples && len(defenderExamples) >= *numExamples {
				break
			}
		}

		// Double-check after processing the batch
		if len(declarerExamples) >= *numExamples && len(defenderExamples) >= *numExamples {
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
	fmt.Printf("  Declarer examples: %d (%.1f%%) - trained with %s\n", actualDeclarer, declarerPct, teacherDisplayName(*teacher))
	fmt.Printf("  Defender examples: %d (%.1f%%) - trained with %s\n", actualDefender, defenderPct, teacherDisplayName(*defenderTeacher))
	fmt.Printf("\n✓ Dataset generation complete!\n")
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

	return examples
}

// collectDefenderExamples plays a game with heuristic declarer vs the selected defender teacher.
func collectDefenderExamples(g *game.GameState, defenderSearchAgent, heuristicAgent *agent.SkatAgent, defenderTeacher string) []ImitationExample {
	var examples []ImitationExample

	if g.Declarer == nil {
		return examples
	}

	declarer := *g.Declarer
	defenderAgent := heuristicAgent
	if isSearchTeacher(defenderTeacher) {
		defenderAgent = defenderSearchAgent
	}

	// Heuristic declarer vs selected defender teacher
	agent.SetAgentForPlayer(g.GetPlayerByPosition(declarer), heuristicAgent)
	agent.SetAgentForPlayer(g.GetPlayerByPosition((declarer+1)%3), defenderAgent)
	agent.SetAgentForPlayer(g.GetPlayerByPosition((declarer+2)%3), defenderAgent.CachedClone())

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
func playGameAndCollectExamples(searchAgent, defenderSearchAgent, heuristicAgent *agent.SkatAgent, defenderTeacher string) []ImitationExample {
	var examples []ImitationExample

	// Setup game and run bidding once
	g, overbid := setupGame(heuristicAgent)

	if g.Declarer == nil || overbid {
		// No declarer or overbid, skip
		return examples
	}

	// Collect declarer examples: search-teacher declarer vs heuristic defenders
	gDeclarer := g.Clone()
	declarerExamples := collectDeclarerExamples(gDeclarer, searchAgent, heuristicAgent)
	examples = append(examples, declarerExamples...)

	// Collect defender examples: heuristic declarer vs selected defender teacher
	gDefender := g.Clone()
	defenderExamples := collectDefenderExamples(gDefender, defenderSearchAgent, heuristicAgent, defenderTeacher)
	examples = append(examples, defenderExamples...)

	return examples
}
