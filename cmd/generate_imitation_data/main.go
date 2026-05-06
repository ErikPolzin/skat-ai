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
	numGames := flag.Int("games", 10000, "Number of games to generate")
	outputFile := flag.String("output", ".data/imitation_dataset.csv", "Output file for dataset")
	searchDepth := flag.Int("depth", 7, "Minimax search depth (default: 7)")
	workers := flag.Int("workers", runtime.NumCPU(), "Number of parallel workers")
	agentType := flag.String("agent", "minimax", "Agent type to collect data from (minimax or heuristic)")
	flag.Parse()

	if *agentType != "minimax" && *agentType != "heuristic" {
		fmt.Fprintf(os.Stderr, "Invalid agent type: %s (must be 'minimax' or 'heuristic')\n", *agentType)
		os.Exit(1)
	}

	fmt.Printf("Generating imitation learning dataset from %d %s games...\n", *numGames, *agentType)
	if *agentType == "minimax" {
		fmt.Printf("Using search depth: %d\n", *searchDepth)
	}
	fmt.Printf("Using %d parallel workers\n", *workers)

	// Channel for collecting results
	examplesChan := make(chan []ImitationExample, *workers)
	var wg sync.WaitGroup

	// Progress tracking
	progressChan := make(chan int, *workers)
	doneChan := make(chan bool)

	// Progress reporter goroutine
	go func() {
		gamesCompleted := 0
		for range progressChan {
			gamesCompleted++
			if gamesCompleted%100 == 0 {
				fmt.Printf("  Generated %d games...\n", gamesCompleted)
			}
		}
		doneChan <- true
	}()

	// Worker function
	worker := func(numGamesToGenerate int) {
		defer wg.Done()

		var expertAgent *agent.SkatAgent
		var opponentAgent *agent.SkatAgent

		if *agentType == "minimax" {
			// Create minimax agent for optimal examples
			expertAgent = agent.NewAgentWithStrategies(
				"MinimaxExpert",
				strategies.NewWeightedHeuristicBiddingStrategy(),
				strategies.NewWeightedHeuristicGameChoiceStrategy(),
				strategies.NewPerfectInfoMinimaxStrategyWithDepth(*searchDepth),
			)

			// Create heuristic agent for opponent simulation
			opponentAgent = agent.NewAgentWithStrategies(
				"HeuristicOpponent",
				&agent.HeuristicBiddingStrategy{},
				&agent.HeuristicGameChoiceStrategy{},
				&agent.HeuristicCardPlayStrategy{},
			)
		} else {
			// Create heuristic agent for data collection
			expertAgent = agent.NewAgentWithStrategies(
				"HeuristicExpert",
				&agent.HeuristicBiddingStrategy{},
				&agent.HeuristicGameChoiceStrategy{},
				&agent.HeuristicCardPlayStrategy{},
			)

			// Use same heuristic for opponents
			opponentAgent = expertAgent
		}

		for i := 0; i < numGamesToGenerate; i++ {
			examples := playGameAndCollectExamples(expertAgent, opponentAgent)
			examplesChan <- examples
			progressChan <- 1
		}
	}

	// Distribute work across workers
	gamesPerWorker := *numGames / *workers
	remainder := *numGames % *workers

	for i := 0; i < *workers; i++ {
		wg.Add(1)
		gamesToGenerate := gamesPerWorker
		if i < remainder {
			gamesToGenerate++
		}
		go worker(gamesToGenerate)
	}

	// Close channels when all workers are done
	go func() {
		wg.Wait()
		close(examplesChan)
		close(progressChan)
	}()

	// Collect results with balanced declarer/defender split
	var declarerExamples []ImitationExample
	var defenderExamples []ImitationExample

	for examples := range examplesChan {
		for _, ex := range examples {
			if ex.IsDeclarer {
				declarerExamples = append(declarerExamples, ex)
			} else {
				defenderExamples = append(defenderExamples, ex)
			}
		}
	}

	// Wait for progress reporter to finish
	<-doneChan

	// Balance the dataset to 50/50 declarer/defender
	minCount := len(declarerExamples)
	if len(defenderExamples) < minCount {
		minCount = len(defenderExamples)
	}

	// Create balanced dataset
	dataset := make([]ImitationExample, 0, minCount*2)
	dataset = append(dataset, declarerExamples[:minCount]...)
	dataset = append(dataset, defenderExamples[:minCount]...)

	fmt.Printf("\nBalanced dataset: %d declarer + %d defender = %d total examples\n",
		minCount, minCount, len(dataset))

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
	fmt.Printf("\nDataset Statistics:\n")
	fmt.Printf("  Total examples: %d\n", len(dataset))
	fmt.Printf("  Declarer examples: %d (%.1f%%)\n", minCount, 50.0)
	fmt.Printf("  Defender examples: %d (%.1f%%)\n", minCount, 50.0)
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

// collectDeclarerExamples plays a game with expert declarer vs opponent defenders
func collectDeclarerExamples(g *game.GameState, expertAgent, opponentAgent *agent.SkatAgent) []ImitationExample {
	var examples []ImitationExample

	if g.Declarer == nil {
		return examples
	}

	declarer := *g.Declarer
	// Replace declarer with expert agent
	agent.SetAgentForPlayer(g.GetPlayerByPosition(declarer), expertAgent)
	agent.SetAgentForPlayer(g.GetPlayerByPosition((declarer+1)%3), opponentAgent)
	agent.SetAgentForPlayer(g.GetPlayerByPosition((declarer+2)%3), opponentAgent)

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

// collectDefenderExamples plays a game with expert defenders vs opponent declarer
func collectDefenderExamples(g *game.GameState, expertAgent, opponentAgent *agent.SkatAgent) []ImitationExample {
	var examples []ImitationExample

	if g.Declarer == nil {
		return examples
	}

	declarer := *g.Declarer
	agent.SetAgentForPlayer(g.GetPlayerByPosition(declarer), opponentAgent)
	agent.SetAgentForPlayer(g.GetPlayerByPosition((declarer+1)%3), expertAgent)
	agent.SetAgentForPlayer(g.GetPlayerByPosition((declarer+2)%3), expertAgent)

	// Card play phase
	for g.Phase == game.PhasePlaying {
		validMoves := g.GetValidMoves()
		currentPlayer := g.CurrentPlayer

		if currentPlayer != declarer {
			// Encode state
			enc := encoding.EncodeNeuralCardPlay(g, currentPlayer, validMoves)
			state := enc.ToSlice()
			validMask := enc.GetValidMask()

			// Get expert action
			card := expertAgent.SelectMove(g, validMoves)
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
func playGameAndCollectExamples(expertAgent *agent.SkatAgent, opponentAgent *agent.SkatAgent) []ImitationExample {
	var examples []ImitationExample

	// Setup game and run bidding once
	g, overbid := setupGame(opponentAgent)

	if g.Declarer == nil || overbid {
		// Game was passed, no examples
		return examples
	}

	// Collect declarer examples: expert declarer vs opponent defenders
	gDeclarer := g.Clone()
	declarerExamples := collectDeclarerExamples(gDeclarer, expertAgent, opponentAgent)
	examples = append(examples, declarerExamples...)

	// Collect defender examples: opponent declarer vs expert defenders
	gDefender := g.Clone()
	defenderExamples := collectDefenderExamples(gDefender, expertAgent, opponentAgent)
	examples = append(examples, defenderExamples...)

	return examples
}
