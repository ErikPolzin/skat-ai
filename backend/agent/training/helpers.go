package training

import (
	"math/rand"
	"runtime"
	"skat/agent"
	"skat/game"
	"sync"
)

// EvaluateAgents runs evaluation games in parallel with the specified agent configuration.
// Each game is played once, with agents properly positioned based on configuration.
// Agents collect their own metrics internally.
func EvaluateAgents(config agent.AgentConfig, games int) {
	evaluateAgents(config, games, 0, false)
}

// EvaluateAgentsWithSeed assigns each game its own deterministic RNG. Results
// are reproducible regardless of worker scheduling.
func EvaluateAgentsWithSeed(config agent.AgentConfig, games int, seed int64) {
	evaluateAgents(config, games, seed, true)
}

func evaluateAgents(config agent.AgentConfig, games int, seed int64, deterministic bool) {
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
			for gameIndex := range gameChan {
				g := agent.WithAgentPlayers(game.NewGame(), localConfig)
				if deterministic {
					g.GameNumber = gameIndex
					agent.PlayFullGameWithRand(g, localConfig, rand.New(rand.NewSource(seed+int64(gameIndex))))
				} else {
					agent.PlayFullGame(g, localConfig)
				}
			}
			// Merge local agent metrics back to main agents
			config.MergeMetrics(localConfig)
		}()
	}

	wg.Wait()
}
