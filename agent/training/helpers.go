package training

import (
	"fmt"
	"runtime"
	"skat/agent"
	"skat/game"
	"sync"
	"sync/atomic"
)

// PlayFullGame plays a complete game from deal to finish.
// The game is played with proper agent positioning:
// - During bidding, test agent is at testPos, baseline agents at other positions
// - If baseline becomes declarer, agents are repositioned so test agents defend
// This ensures test and baseline agents are never on the same team.
func PlayFullGame(testAgent, baselineAgent *agent.SkatAgent, testPos int) *game.GameState {
	g := game.NewGame()
	g = g.WithTestPlayers()
	g = g.WithCardsDealt()

	var agents [3]*agent.SkatAgent
	agents[testPos] = testAgent
	agents[(testPos+1)%3] = baselineAgent.Clone()
	agents[(testPos+2)%3] = baselineAgent.Clone()

	playGameToCompletionInternal(g, testAgent, baselineAgent, agents)
	return g
}

// PlayGameWithMode plays a game with a pre-configured declarer, hand, and game mode.
// This is useful for testing specific scenarios. The test agent is the declarer.
func PlayGameWithMode(testAgent, baselineAgent *agent.SkatAgent, declarerHand game.Cards, mode game.GameMode, trumpSuit game.Suit) *game.GameState {
	g := game.NewGame()
	g = g.WithTestPlayers()
	g = g.WithPlayerHand(game.Speaker, declarerHand)
	g = g.WithDeclarer(game.Speaker, 0)
	g = g.WithSkatPickedUp(false)
	g = g.WithGame(mode, trumpSuit)

	playGameToCompletionInternal(g, testAgent, baselineAgent, [3]*agent.SkatAgent{testAgent, baselineAgent, baselineAgent})
	return g
}

// PlayGameToCompletion plays out a game ensuring test and baseline agents are never teammates.
// If testAgent and baselineAgent are both non-nil, agents are repositioned after bidding if needed.
// If either is nil, the provided agents array is used as-is without repositioning.
func PlayGameToCompletion(g *game.GameState, testAgent, baselineAgent *agent.SkatAgent, agents [3]*agent.SkatAgent) {
	playGameToCompletionInternal(g, testAgent, baselineAgent, agents)
}

// playGameToCompletionInternal is the internal implementation that handles agent repositioning.
// If testAgent and baselineAgent are provided, it ensures they're never on the same team.
func playGameToCompletionInternal(g *game.GameState, testAgent, baselineAgent *agent.SkatAgent, agents [3]*agent.SkatAgent) {
	// Bidding phase
	for g.Phase == game.PhaseBidding {
		currentAgent := agents[g.CurrentPlayer]
		accept := currentAgent.Bid(g)
		_, err := g.Bid(accept)
		if err != nil {
			panic(fmt.Sprintf("Bid error: %v", err))
		}
	}

	// After bidding, check if we need to reposition agents
	// If baseline became declarer, we need 2 test agents as defenders
	// Only do this if testAgent and baselineAgent are provided (not nil)
	if testAgent != nil && baselineAgent != nil && g.Phase == game.PhaseSkatExchange && g.Declarer != nil {
		declarerPos := int(*g.Declarer)
		if agents[declarerPos] == baselineAgent {
			// Baseline is declarer - need to swap in test agents as defenders
			agents[(declarerPos+1)%3] = testAgent
			agents[(declarerPos+2)%3] = testAgent.Clone()
		}
	}

	// Skat exchange and game choice
	if g.Phase == game.PhaseSkatExchange && g.Declarer != nil {
		declarerAgent := agents[*g.Declarer]
		_, err := g.SkatDecision(true)
		if err != nil {
			panic(fmt.Sprintf("SkatDecision error: %v", err))
		}
		mode, trumpSuit := declarerAgent.ChooseGame(g)
		card1, card2 := declarerAgent.ChooseSkatDiscard(g.Players[*g.Declarer].Hand, mode, trumpSuit)
		_, err = g.Discard(card1, card2)
		if err != nil {
			panic(fmt.Sprintf("Discard error: %v", err))
		}
		_, err = g.DeclareGame(mode, trumpSuit, false, false) // No announcements in training
		if err != nil {
			panic(fmt.Sprintf("DeclareGame error: %v", err))
		}
	}

	// Playing phase
	for g.Phase == game.PhasePlaying {
		validMoves := g.GetValidMoves()
		if len(validMoves) == 0 {
			panic("Cannot play game, no valid moves")
		}
		currentAgent := agents[g.CurrentPlayer]
		move := currentAgent.SelectMove(g, validMoves)
		if _, err := g.PlayCard(move); err != nil {
			panic(fmt.Sprintf("PlayCard error: %v", err))
		}
		// Resolve trick if complete
		if len(g.Trick) == 3 {
			if _, err := g.ResolveTrick(); err != nil {
				panic(fmt.Sprintf("ResolveTrick error: %v", err))
			}
		}
	}

	if g.Phase != game.PhaseComplete {
		panic(fmt.Sprintf("Tried to play game to completion but phase is: %s", g.Phase))
	}

	// Record metrics if agents have them enabled
	if testAgent != nil && baselineAgent != nil && g.Declarer != nil && !(g.SpeakerPassed && g.ListenerPassed && g.DealerPassed) {
		playerResults := g.PlayerResults()
		if playerResults != nil {
			// Count how many positions have test vs baseline agents
			testCount := 0
			baselineCount := 0
			for pos := 0; pos < 3; pos++ {
				if agents[pos] == testAgent {
					testCount++
				} else {
					baselineCount++
				}
			}

			// Record metrics based on who is at each position
			for pos := 0; pos < 3; pos++ {
				if agents[pos] == testAgent {
					testAgent.RecordGameResult(g, playerResults[pos])
				} else {
					// This is a baseline agent or clone
					baselineAgent.RecordGameResult(g, playerResults[pos])
				}
			}
		}
	}
}

// EvaluationStats holds the statistics collected during agent evaluation
type EvaluationStats struct {
	TestWins           int64
	TestGames          int64
	TestPoints         int64
	TestOverbid        int64
	BaselineWins       int64
	BaselineGames      int64
	BaselinePoints     int64
	BaselineOverbid    int64
	PassedGames        int64
	GamesCompleted     int64
	TestGrandGames     int64
	TestGrandWins      int64
	TestSuitGames      int64
	TestSuitWins       int64
	TestNullGames      int64
	TestNullWins       int64
	BaselineGrandGames int64
	BaselineGrandWins  int64
	BaselineSuitGames  int64
	BaselineSuitWins   int64
	BaselineNullGames  int64
	BaselineNullWins   int64
}

// EvaluateAgents runs evaluation games in parallel comparing test agent against baseline.
// Each game is played once, with agents properly positioned based on who becomes declarer.
// Agents collect their own metrics internally.
func EvaluateAgents(testAgent, baselineAgent *agent.SkatAgent, games int) *EvaluationStats {
	numWorkers := runtime.GOMAXPROCS(0)

	// Enable metrics on agents if not already enabled
	testAgent.EnableMetrics()
	baselineAgent.EnableMetrics()

	var passedGamesAtomic atomic.Int64

	// Worker pool
	var wg sync.WaitGroup
	gameChan := make(chan int, games)

	for i := 0; i < games; i++ {
		gameChan <- i
	}
	close(gameChan)

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Clone agents for this worker to avoid mutex contention on neural networks
			localTestAgent := testAgent.Clone()
			localTestAgent.EnableMetrics()
			localBaselineAgent := baselineAgent.Clone()
			localBaselineAgent.EnableMetrics()

			for gameNum := range gameChan {
				// Rotate test agent through positions
				testPos := gameNum % 3

				// Play game with test agent in this position
				g := PlayFullGame(localTestAgent, localBaselineAgent, testPos)

				// Track passed games
				if g.Declarer == nil || (g.SpeakerPassed && g.ListenerPassed && g.DealerPassed) {
					passedGamesAtomic.Add(1)
				}
			}

			// Merge local agent metrics back to main agents
			testAgent.MergeMetrics(localTestAgent.GetMetrics())
			baselineAgent.MergeMetrics(localBaselineAgent.GetMetrics())
		}()
	}

	wg.Wait()

	// Get final metrics snapshots
	testMetrics := testAgent.GetMetrics()
	baselineMetrics := baselineAgent.GetMetrics()

	return &EvaluationStats{
		TestWins:           testMetrics.Wins,
		TestGames:          testMetrics.Games,
		TestPoints:         testMetrics.Points,
		TestOverbid:        testMetrics.Overbid,
		BaselineWins:       baselineMetrics.Wins,
		BaselineGames:      baselineMetrics.Games,
		BaselinePoints:     baselineMetrics.Points,
		BaselineOverbid:    baselineMetrics.Overbid,
		PassedGames:        passedGamesAtomic.Load(),
		GamesCompleted:     int64(games),
		TestGrandGames:     testMetrics.GrandGames,
		TestGrandWins:      testMetrics.GrandWins,
		TestSuitGames:      testMetrics.SuitGames,
		TestSuitWins:       testMetrics.SuitWins,
		TestNullGames:      testMetrics.NullGames,
		TestNullWins:       testMetrics.NullWins,
		BaselineGrandGames: baselineMetrics.GrandGames,
		BaselineGrandWins:  baselineMetrics.GrandWins,
		BaselineSuitGames:  baselineMetrics.SuitGames,
		BaselineSuitWins:   baselineMetrics.SuitWins,
		BaselineNullGames:  baselineMetrics.NullGames,
		BaselineNullWins:   baselineMetrics.NullWins,
	}
}

// runParallelTraining runs training episodes in parallel across all CPU cores
// episodeFunc is called for each episode and should be thread-safe
func RunInParallel(episodes int, episodeFunc func()) {
	numWorkers := runtime.GOMAXPROCS(0)
	fmt.Printf("Training for %d episodes using %d CPU cores...\n", episodes, numWorkers)

	var episodesCompletedAtomic atomic.Int64

	// Progress reporting goroutine
	done := make(chan struct{})
	go func() {
		lastReported := int64(0)
		for {
			select {
			case <-done:
				return
			default:
				completed := episodesCompletedAtomic.Load()
				if completed-lastReported >= 10000 {
					fmt.Printf(".")
					if completed%100000 == 0 {
						fmt.Printf(" %d\n", completed)
					}
					lastReported = completed
				}
				runtime.Gosched()
			}
		}
	}()

	// Worker pool
	var wg sync.WaitGroup
	episodeChan := make(chan int, episodes)

	// Fill work queue
	for ep := 0; ep < episodes; ep++ {
		episodeChan <- ep
	}
	close(episodeChan)

	// Start workers
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range episodeChan {
				episodeFunc()
				episodesCompletedAtomic.Add(1)
			}
		}()
	}

	wg.Wait()
	close(done)
	fmt.Println()
}
