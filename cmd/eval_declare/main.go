package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"skat/agent"
	"skat/agent/training"
	"skat/game"
	"strings"
	"sync"
	"sync/atomic"
)

func main() {
	qtablePath := flag.String("qtable", "game_choice_qtable.gob", "Path to game choice Q-table file")
	games := flag.Int("games", 500, "Number of evaluation games")
	flag.Parse()

	fmt.Println("Game Choice (Declaration) Strategy Evaluation")
	fmt.Println("==============================================")
	fmt.Println("Tests Q-learning game choice vs Heuristic game choice")
	fmt.Println("All agents use heuristic bidding and card play")
	fmt.Println()

	// Load pre-trained game choice Q-table
	fmt.Printf("Loading Q-table from %s...\n", *qtablePath)
	if _, err := os.Stat(*qtablePath); os.IsNotExist(err) {
		fmt.Printf("Error: Q-table file not found: %s\n", *qtablePath)
		fmt.Println("Please train the agent first using: go run cmd/train_game_choice/main.go")
		os.Exit(1)
	}

	data, err := agent.LoadQTableData(*qtablePath, true)
	if err != nil {
		fmt.Printf("Error loading Q-table: %v\n", err)
		os.Exit(1)
	}

	// Create test agent: Heuristic bidding + Q-learning game choice + Heuristic play
	testAgent := agent.NewHeuristicAgent("Test")
	if qStrat, ok := testAgent.GetGameChoiceStrategy().(*agent.QLearningGameChoiceStrategy); ok {
		qStrat.SetQTable(data.QTable)
		qStrat.SetEpsilon(0.0) // No exploration during eval
	}
	fmt.Println("✓ Test agent: Heuristic bidding + Q-learning game choice + Heuristic play")

	// Baseline agent: All heuristic
	baselineAgent := agent.NewHeuristicAgent("Baseline")
	fmt.Println("✓ Baseline agent: All heuristic")

	numWorkers := runtime.GOMAXPROCS(0)

	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Printf("Running %d games on %d CPU cores...\n", *games, numWorkers)
	fmt.Println(strings.Repeat("=", 50) + "\n")

	var testWinsAtomic atomic.Int64
	var testGamesAtomic atomic.Int64
	var testPointsAtomic atomic.Int64
	var baselineWinsAtomic atomic.Int64
	var baselineGamesAtomic atomic.Int64
	var baselinePointsAtomic atomic.Int64
	var gamesCompletedAtomic atomic.Int64
	var passedGamesAtomic atomic.Int64

	// Progress reporting
	done := make(chan struct{})
	go func() {
		lastReported := int64(0)
		for {
			select {
			case <-done:
				return
			default:
				completed := gamesCompletedAtomic.Load()
				if completed-lastReported >= 100 {
					testGames := testGamesAtomic.Load()
					testWins := testWinsAtomic.Load()
					baseGames := baselineGamesAtomic.Load()
					baseWins := baselineWinsAtomic.Load()

					testWR := 0.0
					if testGames > 0 {
						testWR = float64(testWins) / float64(testGames) * 100
					}
					baseWR := 0.0
					if baseGames > 0 {
						baseWR = float64(baseWins) / float64(baseGames) * 100
					}

					fmt.Printf("Game %d: Test %.1f%% (%d/%d) | Baseline %.1f%% (%d/%d)\n",
						completed, testWR, testWins, testGames, baseWR, baseWins, baseGames)
					lastReported = completed
				}
				runtime.Gosched()
			}
		}
	}()

	// Worker pool
	var wg sync.WaitGroup
	gameChan := make(chan int, *games)

	for i := 0; i < *games; i++ {
		gameChan <- i
	}
	close(gameChan)

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for i := range gameChan {
				// Rotate test agent through positions
				var agents [3]*agent.SkatAgent
				testPos := i % 3

				agents[testPos] = testAgent
				agents[(testPos+1)%3] = baselineAgent
				agents[(testPos+2)%3] = baselineAgent

				g := training.PlayFullGame(agents[0], agents[1], agents[2])
				result := g.Result()

				if g.Declarer == nil {
					passedGamesAtomic.Add(1)
				} else if *g.Declarer == game.GamePosition(testPos) {
					testGamesAtomic.Add(1)
					testPointsAtomic.Add(int64(result.Value))
					if result.DeclarerWon {
						testWinsAtomic.Add(1)
					}
				} else {
					baselineGamesAtomic.Add(1)
					baselinePointsAtomic.Add(int64(result.Value))
					if result.DeclarerWon {
						baselineWinsAtomic.Add(1)
					}
				}

				gamesCompletedAtomic.Add(1)
			}
		}()
	}

	wg.Wait()
	close(done)

	testGames := testGamesAtomic.Load()
	testWins := testWinsAtomic.Load()
	testPoints := testPointsAtomic.Load()
	baselineGames := baselineGamesAtomic.Load()
	baselineWins := baselineWinsAtomic.Load()
	baselinePoints := baselinePointsAtomic.Load()
	passedGames := passedGamesAtomic.Load()

	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("FINAL RESULTS")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Printf("\nPassed games (everyone passed): %d/%d (%.1f%%)\n",
		passedGames, *games, float64(passedGames)/float64(*games)*100)

	if testGames > 0 {
		fmt.Printf("\nTest (Q-learning game choice):\n")
		fmt.Printf("  Win rate: %.1f%% (%d/%d games as declarer)\n",
			float64(testWins)/float64(testGames)*100, testWins, testGames)
		fmt.Printf("  Avg points as declarer: %.1f\n", float64(testPoints)/float64(testGames))
	}

	if baselineGames > 0 {
		fmt.Printf("\nBaseline (Heuristic):\n")
		fmt.Printf("  Win rate: %.1f%% (%d/%d games as declarer)\n",
			float64(baselineWins)/float64(baselineGames)*100, baselineWins, baselineGames)
		fmt.Printf("  Avg points as declarer: %.1f\n", float64(baselinePoints)/float64(baselineGames))
	}

	if testGames > 0 && baselineGames > 0 {
		improvement := (float64(testWins)/float64(testGames) - float64(baselineWins)/float64(baselineGames)) * 100
		pointDiff := float64(testPoints)/float64(testGames) - float64(baselinePoints)/float64(baselineGames)
		fmt.Printf("\nGame choice improvement: %+.1f percentage points\n", improvement)
		fmt.Printf("Point difference: %+.1f points per game\n", pointDiff)
	}
}
