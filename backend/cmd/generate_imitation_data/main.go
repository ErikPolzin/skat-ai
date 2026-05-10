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
	State      [114]float32 // DQN state encoding
	ValidMask  [32]float32  // Valid moves at this state
	Action     int          // Card index that expert chose
	IsDeclarer bool         // Role (for separate networks)
}

func main() {
	numExamples := flag.Int("examples", 100000, "Number of examples to collect (per role: declarer and defender)")
	outputFile := flag.String("output", ".data/imitation_dataset.csv", "Output file for dataset")
	searchDepth := flag.Int("depth", 7, "Minimax search depth for declarer (default: 7)")
	workers := flag.Int("workers", runtime.NumCPU(), "Number of parallel workers")
	flag.Parse()

	fmt.Printf("Generating imitation learning dataset with %d examples per role...\n", *numExamples)
	fmt.Printf("  Declarer strategy: Minimax (depth %d)\n", *searchDepth)
	fmt.Printf("  Defender strategy: Heuristic\n")
	fmt.Printf("  Filtering: Excluding Zwangsspiel and overbid games\n")
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

		// Create minimax agent for declarer examples
		minimaxAgent := agent.NewAgentWithStrategies(
			"MinimaxDeclarer",
			strategies.NewWeightedHeuristicBiddingStrategy(),
			strategies.NewWeightedHeuristicGameChoiceStrategy(),
			strategies.NewPerfectInfoMinimaxStrategyWithDepth(*searchDepth),
		)

		// Create heuristic agent for defender examples and opponent simulation
		heuristicAgent := agent.NewAgentWithStrategies(
			"HeuristicDefender",
			&agent.HeuristicBiddingStrategy{},
			&agent.HeuristicGameChoiceStrategy{},
			&agent.HeuristicCardPlayStrategy{},
		)

		// Keep generating games until we signal stop
		for {
			select {
			case <-stopChan:
				return
			default:
				examples := playGameAndCollectExamples(minimaxAgent, heuristicAgent)
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
					// Game was filtered out (Zwangsspiel or overbid)
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
	header := make([]string, 0, 114+32+2)
	for i := 0; i < 114; i++ {
		header = append(header, fmt.Sprintf("s%d", i))
	}
	for i := 0; i < 32; i++ {
		header = append(header, fmt.Sprintf("m%d", i))
	}
	header = append(header, "action", "is_declarer")
	if err := writer.Write(header); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write header: %v\n", err)
		os.Exit(1)
	}

	// Write examples
	for _, ex := range dataset {
		record := make([]string, 0, 114+32+2)

		// State features (114)
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
	fmt.Printf("  Defender examples: %d (%.1f%%) - trained with Heuristic\n", actualDefender, defenderPct)
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

// collectDeclarerExamples plays a game with minimax declarer vs heuristic defenders
func collectDeclarerExamples(g *game.GameState, minimaxAgent, heuristicAgent *agent.SkatAgent) []ImitationExample {
	var examples []ImitationExample

	if g.Declarer == nil {
		return examples
	}

	declarer := *g.Declarer
	// Minimax declarer vs heuristic defenders
	agent.SetAgentForPlayer(g.GetPlayerByPosition(declarer), minimaxAgent)
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

			// Store declarer example
			examples = append(examples, ImitationExample{
				State:      state,
				ValidMask:  validMask,
				Action:     action,
				IsDeclarer: true,
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
			if _, err := g.ResolveTrick(); err != nil {
				panic(fmt.Sprintf("ResolveTrick error: %v", err))
			}
		}
	}

	return examples
}

// collectDefenderExamples plays a game with heuristic defenders vs heuristic declarer
func collectDefenderExamples(g *game.GameState, heuristicAgent *agent.SkatAgent) []ImitationExample {
	var examples []ImitationExample

	if g.Declarer == nil {
		return examples
	}

	declarer := *g.Declarer
	// Heuristic declarer vs heuristic defenders
	agent.SetAgentForPlayer(g.GetPlayerByPosition(declarer), heuristicAgent)
	agent.SetAgentForPlayer(g.GetPlayerByPosition((declarer+1)%3), heuristicAgent)
	agent.SetAgentForPlayer(g.GetPlayerByPosition((declarer+2)%3), heuristicAgent)

	// Card play phase
	for g.Phase == game.PhasePlaying {
		validMoves := g.GetValidMoves()
		currentPlayer := g.CurrentPlayer

		if currentPlayer != declarer {
			// Encode state
			enc := encoding.EncodeNeuralCardPlay(g, currentPlayer, validMoves)
			state := enc.ToSlice()
			validMask := enc.GetValidMask()

			// Get heuristic defender action
			card := heuristicAgent.SelectMove(g, validMoves)
			action := encoding.CardToIndex(card)

			// Store defender example
			examples = append(examples, ImitationExample{
				State:      state,
				ValidMask:  validMask,
				Action:     action,
				IsDeclarer: false,
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
			if _, err := g.ResolveTrick(); err != nil {
				panic(fmt.Sprintf("ResolveTrick error: %v", err))
			}
		}
	}

	return examples
}

// playGameAndCollectExamples plays games twice: once for declarer examples, once for defender examples
func playGameAndCollectExamples(minimaxAgent *agent.SkatAgent, heuristicAgent *agent.SkatAgent) []ImitationExample {
	var examples []ImitationExample

	// Setup game and run bidding once
	g, overbid := setupGame(heuristicAgent)

	if g.Declarer == nil || overbid {
		// No declarer or overbid, skip
		return examples
	}

	// Skip Zwangsspiel games (all players passed - forced game with weak hand)
	if g.IsZwangsspiel() {
		return examples
	}

	// Collect declarer examples: minimax declarer vs heuristic defenders
	gDeclarer := g.Clone()
	declarerExamples := collectDeclarerExamples(gDeclarer, minimaxAgent, heuristicAgent)
	examples = append(examples, declarerExamples...)

	// Collect defender examples: heuristic declarer vs heuristic defenders
	gDefender := g.Clone()
	defenderExamples := collectDefenderExamples(gDefender, heuristicAgent)
	examples = append(examples, defenderExamples...)

	return examples
}
