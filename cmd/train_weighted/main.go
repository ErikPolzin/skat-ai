package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"skat/agent"
	"skat/agent/strategies"
	"skat/agent/training/weighted"
	"skat/game"
)

func main() {
	// Parse flags
	episodes := flag.Int("episodes", 10000, "Number of self-play games to collect data")
	learningRate := flag.Float64("lr", 0.001, "Learning rate for gradient descent")
	epochs := flag.Int("epochs", 100, "Number of training epochs")
	outputWeights := flag.String("output", ".data/models/weighted_heuristic.json", "Output weights file")

	flag.Parse()

	fmt.Println("============================================================")
	fmt.Println("Weighted Heuristic Training")
	fmt.Println("============================================================")
	fmt.Printf("\nConfiguration:\n")
	fmt.Printf("  Self-play episodes: %d\n", *episodes)
	fmt.Printf("  Learning rate: %.5f\n", *learningRate)
	fmt.Printf("  Training epochs: %d\n", *epochs)
	fmt.Printf("  Output: %s\n\n", *outputWeights)

	// Collect training data from self-play
	fmt.Println("Collecting training data from self-play...")
	examples := collectTrainingData(*episodes)
	fmt.Printf("Collected %d training examples\n\n", len(examples))

	// Train weights
	fmt.Println("Training weights...")
	trainer := weighted.NewWeightTrainer(*learningRate, *epochs)
	weights := trainer.TrainWeights(examples)

	// Save weights to file
	fmt.Printf("\nSaving weights to %s...\n", *outputWeights)
	if err := saveWeights(weights, *outputWeights); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save weights: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n✓ Training complete!")
	fmt.Println("\nTrained weights:")
	printWeights(weights)
}

// collectTrainingData generates training examples by playing games with heuristic agents
func collectTrainingData(episodes int) []weighted.BiddingExample {
	var examples []weighted.BiddingExample

	// Create heuristic agents for self-play
	agent1 := agent.NewHeuristicAgent("Heuristic-1")
	agent2 := agent.NewHeuristicAgent("Heuristic-2")
	agent3 := agent.NewHeuristicAgent("Heuristic-3")

	config := agent.NewThreeWayConfig(agent1, agent2, agent3)
	g := game.NewGame()
	g = agent.WithAgentPlayers(g, config)

	for i := 0; i < episodes; i++ {
		if (i+1)%100 == 0 {
			fmt.Printf("  Played %d/%d games\r", i+1, episodes)
		}

		// Create game and save initial hands before playing
		g = g.WithCardsDealt()

		// Save initial hands before the game modifies them
		initialHands := [3][]game.Card{
			append([]game.Card{}, g.Players[0].Hand...),
			append([]game.Card{}, g.Players[1].Hand...),
			append([]game.Card{}, g.Players[2].Hand...),
		}

		// Play the game with three-way config
		g = agent.WithAgentBidding(g, config)
		g = agent.WithAgentSkatDecision(g)
		g, overbid := agent.WithAgentGameChoice(g)
		if overbid {
			g.NextGame()
			continue
		}
		g = agent.WithAgentCardPlay(g)

		// Collect training examples from declarers only
		if g.Declarer != nil && g.Phase == game.PhaseComplete {
			declarerIdx := int(*g.Declarer)
			results := g.PlayerResults()
			if results != nil {
				result := results[declarerIdx]

				example := weighted.BiddingExample{
					Hand:    initialHands[declarerIdx],
					DidWin:  result.IsWinner,
					Quality: calculateQuality(result),
				}
				examples = append(examples, example)
			}
		}
		g.NextGame()
	}

	fmt.Printf("  Played %d/%d games\n", episodes, episodes)
	return examples
}

// calculateQuality computes a quality score for a game result
func calculateQuality(result game.PlayerResultState) float64 {
	if result.IsWinner {
		// Won: quality is 1.0, scaled by how well we did
		margin := float64(result.PlayerPoints - 61)
		return 0.5 + (margin / 120.0) // 0.5 to 1.0
	}
	// Lost: quality is based on how close we got
	closeness := float64(result.PlayerPoints) / 61.0
	return closeness * 0.5 // 0.0 to 0.5
}

// saveWeights saves bid weights to a JSON file
func saveWeights(weights strategies.BidWeights, filename string) error {
	// Create directory if it doesn't exist
	dir := filename[:len(filename)-len("/weighted_heuristic.json")]
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(weights)
}

// printWeights displays the trained weights
func printWeights(w strategies.BidWeights) {
	fmt.Println("\nGrand weights:")
	fmt.Printf("  Bias:          %7.3f\n", w.GrandBias)
	fmt.Printf("  Jacks:         %7.3f\n", w.GrandJacks)
	fmt.Printf("  Aces:          %7.3f\n", w.GrandAces)
	fmt.Printf("  Tens:          %7.3f\n", w.GrandTens)
	fmt.Printf("  AceTenPairs:   %7.3f\n", w.GrandAceTenPairs)
	fmt.Printf("  TotalWinners:  %7.3f\n", w.GrandTotalWinners)

	fmt.Println("\nSuit weights:")
	fmt.Printf("  Bias:          %7.3f\n", w.SuitBias)
	fmt.Printf("  TrumpLength:   %7.3f\n", w.SuitTrumpLength)
	fmt.Printf("  TrumpLengthSq: %7.3f\n", w.SuitTrumpLengthSq)
	fmt.Printf("  TopTrumps:     %7.3f\n", w.SuitTopTrumps)
	fmt.Printf("  SideAces:      %7.3f\n", w.SuitSideAces)
	fmt.Printf("  VoidSuits:     %7.3f\n", w.SuitVoidSuits)
	fmt.Printf("  ShortSuits:    %7.3f\n", w.SuitShortSuits)
	fmt.Printf("  AceTenPairs:   %7.3f\n", w.SuitAceTenPairs)

	fmt.Println("\nShared weights:")
	fmt.Printf("  Matadors:      %7.3f\n", w.Matadors)
	fmt.Printf("  TotalPoints:   %7.3f\n", w.TotalPoints)
}
