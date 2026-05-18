package training

import (
	"runtime"
	"skat/agent"
	"skat/game"
	"sync"
)

// EvaluateAgents runs evaluation games in parallel with the specified agent configuration.
// Each game is played once, with agents properly positioned based on configuration.
// Agents collect their own metrics internally.
func EvaluateAgents(config agent.AgentConfig, games int) {
	numWorkers := runtime.GOMAXPROCS(0)

	// Enable metrics on all agents
	config.EnableMetrics()

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
			localConfig := config.CloneAll()
			localConfig.EnableMetrics()
			g := game.NewGame()
			g = agent.WithAgentPlayers(g, localConfig)

			for range gameChan {
				// Play game with the specified configuration
				agent.PlayFullGame(g, localConfig)
				g.NextGame()
			}
			// Merge local agent metrics back to main agents
			config.MergeMetrics(localConfig)
		}()
	}

	wg.Wait()
}
