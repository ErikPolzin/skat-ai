package training

import (
	"fmt"
	"runtime"
	"skat/agent"
	"skat/game"
	"sync"
	"sync/atomic"
)

// PlayFullGame plays a complete game from deal to finish using the provided agents
// Returns the declarer position and player points
func PlayFullGame(agent1, agent2, agent3 *agent.SkatAgent) *game.GameState {
	g := game.NewGame()
	g = g.WithTestPlayers()
	g = g.WithCardsDealt()
	PlayGameToCompletion(g, [3]*agent.SkatAgent{agent1, agent2, agent3})
	return g
}

// PlayGameToCompletion plays out a game using the provided agents
func PlayGameToCompletion(g *game.GameState, agents [3]*agent.SkatAgent) {
	// Bidding phase
	for g.Phase == game.PhaseBidding {
		currentAgent := agents[g.CurrentPlayer]
		accept := currentAgent.Bid(g)
		_, err := g.Bid(accept)
		if err != nil {
			panic(fmt.Sprintf("Bid error: %v", err))
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
