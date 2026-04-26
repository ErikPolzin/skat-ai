package main

import (
	"flag"
	"fmt"
	"runtime"
	"skat/agent"
	"skat/agent/training"
	"strings"
	"sync"
	"sync/atomic"
)

func main() {
	games := flag.Int("games", 500, "Number of evaluation games")
	simulations := flag.Int("simulations", 100, "MCTS simulations per move")
	flag.Parse()

	fmt.Println("Card Play Strategy Evaluation")
	fmt.Println("==============================")
	fmt.Println("Tests MCTS card play vs Heuristic card play")
	fmt.Println("All agents use heuristic bidding and game choice")
	fmt.Println()

	// Print agent info
	fmt.Printf("✓ Test agent: Heuristic bidding/game choice + MCTS play (%d sims)\n", *simulations)
	fmt.Println("✓ Baseline agent: All heuristic")

	numWorkers := runtime.GOMAXPROCS(0)

	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Printf("Running %d games on %d CPU cores...\n", *games, numWorkers)
	fmt.Println(strings.Repeat("=", 50) + "\n")

	var testPointsAtomic atomic.Int64
	var baselinePointsAtomic atomic.Int64
	var gamesCompletedAtomic atomic.Int64

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
					testPts := testPointsAtomic.Load()
					basePts := baselinePointsAtomic.Load()

					fmt.Printf("Game %d: Test %d pts (%.1f avg) | Baseline %d pts (%.1f avg)\n",
						completed, testPts, float64(testPts)/float64(completed),
						basePts, float64(basePts)/float64(completed))
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
			// Create per-worker agents to avoid sharing MCTS state
			workerTestAgent := agent.NewHybridAgent("Test", "heuristic", "heuristic", "mcts", *simulations)
			workerBaselineAgent := agent.NewHeuristicAgent("Baseline")

			for i := range gameChan {
				// Rotate test agent through positions
				var agents [3]*agent.SkatAgent
				testPos := i % 3

				agents[testPos] = workerTestAgent
				agents[(testPos+1)%3] = workerBaselineAgent
				agents[(testPos+2)%3] = workerBaselineAgent

				g := training.PlayFullGame(agents[0], agents[1], agents[2])
				pr := g.PlayerResults()
				points := [3]int64{
					int64(pr[0].PlayerPoints),
					int64(pr[1].PlayerPoints),
					int64(pr[2].PlayerPoints),
				}
				// Award all points (declarer gets their points, defenders split theirs)
				testPointsAtomic.Add(points[testPos])
				// Baseline gets points from the other two positions
				baselinePoints := points[(testPos+1)%3] + points[(testPos+2)%3]
				baselinePointsAtomic.Add(baselinePoints)
				gamesCompletedAtomic.Add(1)
			}
		}()
	}

	wg.Wait()
	close(done)

	testPoints := testPointsAtomic.Load()
	baselinePoints := baselinePointsAtomic.Load()
	totalGames := gamesCompletedAtomic.Load()

	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("FINAL RESULTS")
	fmt.Println(strings.Repeat("=", 50))

	fmt.Printf("\nTest (MCTS play):\n")
	fmt.Printf("  Total points: %d\n", testPoints)
	fmt.Printf("  Avg points per game: %.1f\n", float64(testPoints)/float64(totalGames))

	fmt.Printf("\nBaseline (Heuristic play):\n")
	fmt.Printf("  Total points: %d\n", baselinePoints)
	fmt.Printf("  Avg points per game: %.1f\n", float64(baselinePoints)/float64(totalGames*2))

	pointDiff := float64(testPoints)/float64(totalGames) - float64(baselinePoints)/float64(totalGames*2)
	fmt.Printf("\nCard play improvement: %+.1f points per game\n", pointDiff)
}
