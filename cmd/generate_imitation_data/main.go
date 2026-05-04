package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"strconv"

	"skat/agent"
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
	flag.Parse()

	fmt.Printf("Generating imitation learning dataset from %d heuristic games...\n", *numGames)

	// Create heuristic agent
	heuristicAgent := agent.NewHeuristicAgent("Heuristic")

	var dataset []ImitationExample

	for gameNum := 0; gameNum < *numGames; gameNum++ {
		// Create and play game with all heuristic agents
		examples := playGameAndCollectExamples(heuristicAgent)
		dataset = append(dataset, examples...)

		if (gameNum+1)%100 == 0 {
			fmt.Printf("  Generated %d games (%d examples)...\n", gameNum+1, len(dataset))
		}
	}

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
	declarerExamples := 0
	defenderExamples := 0
	for _, ex := range dataset {
		if ex.IsDeclarer {
			declarerExamples++
		} else {
			defenderExamples++
		}
	}

	fmt.Printf("\nDataset Statistics:\n")
	fmt.Printf("  Total examples: %d\n", len(dataset))
	fmt.Printf("  Declarer examples: %d (%.1f%%)\n", declarerExamples, float64(declarerExamples)/float64(len(dataset))*100)
	fmt.Printf("  Defender examples: %d (%.1f%%)\n", defenderExamples, float64(defenderExamples)/float64(len(dataset))*100)
	fmt.Printf("\n✓ Dataset generation complete!\n")
}

// playGameAndCollectExamples plays one game with heuristic agents and collects all (state, action) pairs
func playGameAndCollectExamples(heuristicAgent *agent.SkatAgent) []ImitationExample {
	var examples []ImitationExample

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
		// Game was passed, no card play examples
		return examples
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

	// Card play phase - collect (state, action) examples
	for g.Phase == game.PhasePlaying {
		validMoves := g.GetValidMoves()
		currentPlayer := g.CurrentPlayer

		// Get state encoding
		enc := encoding.EncodeNeuralCardPlay(g, currentPlayer, validMoves)
		state := enc.ToSlice()
		validMask := enc.GetValidMask()

		// Get action from heuristic
		card := agents[currentPlayer].SelectMove(g, validMoves)
		action := encoding.CardToIndex(card)

		// Determine role
		isDeclarer := currentPlayer == *g.Declarer

		// Store example
		examples = append(examples, ImitationExample{
			State:      state,
			ValidMask:  validMask,
			Action:     action,
			IsDeclarer: isDeclarer,
		})

		// Play card
		if _, err := g.PlayCard(card); err != nil {
			panic(fmt.Sprintf("PlayCard error: %v", err))
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
