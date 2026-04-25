package training

import (
	"fmt"
	"runtime"
	"skat/agent"
	"skat/game"
	"sync"
	"sync/atomic"
)

// initializeGameWithDeal creates a new game state and deals cards
func initializeGameWithDeal() *game.GameState {
	g := game.NewGame()

	// Initialize players for training
	for i := 0; i < 3; i++ {
		g.Players[i] = &game.PlayerState{
			ID:      fmt.Sprintf("player-%d", i),
			Name:    fmt.Sprintf("Player %d", i),
			Hand:    []game.Card{},
			IsAgent: true,
		}
	}

	// Set phase to dealing so Deal() works
	g.Phase = game.PhaseDealing

	if _, err := g.Deal(); err != nil {
		panic(fmt.Sprintf("Deal error: %v", err))
	}
	return g
}

// PlayFullGame plays a complete game from deal to finish using the provided agents
// Returns the declarer position and player points
func PlayFullGame(agent1, agent2, agent3 *agent.SkatAgent) (game.GamePosition, [3]int) {
	g := game.NewGame()
	for p := 0; p < 3; p++ {
		g.Players[p] = &game.PlayerState{
			ID:      fmt.Sprintf("player-%d", p),
			Name:    fmt.Sprintf("Player %d", p),
			Hand:    []game.Card{},
			IsAgent: true,
		}
	}

	g.Phase = game.PhaseDealing
	g.Deal()

	agents := [3]*agent.SkatAgent{agent1, agent2, agent3}

	// Bidding phase
	for g.Phase == game.PhaseBidding {
		currentAgent := agents[g.CurrentPlayer]
		accept := currentAgent.Bid(g)
		g.Bid(accept)
	}

	// Skat exchange and game choice
	if g.Phase == game.PhaseSkatExchange {
		declarerAgent := agents[g.Declarer]
		g.SkatDecision(true)
		mode, trumpSuit := declarerAgent.ChooseGame(g)
		card1, card2 := declarerAgent.ChooseSkatDiscard(g.Players[g.Declarer].Hand, mode, trumpSuit)
		g.Discard(card1, card2)
		g.DeclareGame(mode, trumpSuit, false, false) // No announcements in training
	}

	// Playing phase
	PlayGameToCompletion(g, agents)

	points := g.CalculatePlayerPoints()
	return g.Declarer, [3]int{points[0], points[1], points[2]}
}

// PlayGameToCompletion plays out a game using the provided agents
func PlayGameToCompletion(g *game.GameState, agents [3]*agent.SkatAgent) {
	maxTricks := 10
	tricksPlayed := 0

	for g.Phase == game.PhasePlaying && tricksPlayed < maxTricks {
		validMoves := g.GetValidMoves()
		if len(validMoves) == 0 {
			break
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
			tricksPlayed++
		}
	}
}

// runParallelTraining runs training episodes in parallel across all CPU cores
// episodeFunc is called for each episode and should be thread-safe
func runParallelTraining(episodes int, episodeFunc func()) {
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
				if completed-lastReported >= 100 {
					fmt.Printf(".")
					if completed%1000 == 0 {
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
