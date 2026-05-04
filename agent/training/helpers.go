package training

import (
	"fmt"
	"runtime"
	"skat/agent"
	"skat/game"
	"sync"
	"sync/atomic"
)

type BiddingConfiguration int

const (
	TestAgainstTwoAgents BiddingConfiguration = iota
	FiftyFiftySplit
	ThreeWay
)

// AgentConfig specifies how agents should be positioned during a game
type AgentConfig struct {
	// For TestAgainstTwoAgents and FiftyFiftySplit modes
	TestAgent     *agent.SkatAgent
	BaselineAgent *agent.SkatAgent

	// For ThreeWay mode
	Agent1 *agent.SkatAgent
	Agent2 *agent.SkatAgent
	Agent3 *agent.SkatAgent

	// Configuration mode
	Mode BiddingConfiguration

	// Game number for rotation/alternation
	GameNum int
}

// NewTestAgainstTwoConfig creates a config for testing one agent against two baseline agents
func NewTestAgainstTwoConfig(testAgent, baselineAgent *agent.SkatAgent, gameNum int) AgentConfig {
	return AgentConfig{
		TestAgent:     testAgent,
		BaselineAgent: baselineAgent,
		Mode:          TestAgainstTwoAgents,
		GameNum:       gameNum,
	}
}

// NewFiftyFiftySplitConfig creates a config for 50/50 declarer/defender split
func NewFiftyFiftySplitConfig(testAgent, baselineAgent *agent.SkatAgent, gameNum int) AgentConfig {
	return AgentConfig{
		TestAgent:     testAgent,
		BaselineAgent: baselineAgent,
		Mode:          FiftyFiftySplit,
		GameNum:       gameNum,
	}
}

// NewThreeWayConfig creates a config for three different agents
func NewThreeWayConfig(agent1, agent2, agent3 *agent.SkatAgent, gameNum int) AgentConfig {
	return AgentConfig{
		Agent1:  agent1,
		Agent2:  agent2,
		Agent3:  agent3,
		Mode:    ThreeWay,
		GameNum: gameNum,
	}
}

// CloneAll creates a new AgentConfig with all agents cloned
func (c AgentConfig) CloneAll() AgentConfig {
	cloned := AgentConfig{
		Mode:    c.Mode,
		GameNum: c.GameNum,
	}

	switch c.Mode {
	case TestAgainstTwoAgents, FiftyFiftySplit:
		if c.TestAgent != nil {
			cloned.TestAgent = c.TestAgent.Clone()
		}
		if c.BaselineAgent != nil {
			cloned.BaselineAgent = c.BaselineAgent.Clone()
		}
	case ThreeWay:
		if c.Agent1 != nil {
			cloned.Agent1 = c.Agent1.Clone()
		}
		if c.Agent2 != nil {
			cloned.Agent2 = c.Agent2.Clone()
		}
		if c.Agent3 != nil {
			cloned.Agent3 = c.Agent3.Clone()
		}
	}

	return cloned
}

// EnableMetrics enables metrics collection on all agents in the config
func (c AgentConfig) EnableMetrics() {
	switch c.Mode {
	case TestAgainstTwoAgents, FiftyFiftySplit:
		if c.TestAgent != nil {
			c.TestAgent.EnableMetrics()
		}
		if c.BaselineAgent != nil {
			c.BaselineAgent.EnableMetrics()
		}
	case ThreeWay:
		if c.Agent1 != nil {
			c.Agent1.EnableMetrics()
		}
		if c.Agent2 != nil {
			c.Agent2.EnableMetrics()
		}
		if c.Agent3 != nil {
			c.Agent3.EnableMetrics()
		}
	}
}

// MergeMetrics merges metrics from another config into this config
func (c AgentConfig) MergeMetrics(other AgentConfig) {
	switch c.Mode {
	case TestAgainstTwoAgents, FiftyFiftySplit:
		if c.TestAgent != nil && other.TestAgent != nil {
			c.TestAgent.MergeMetrics(other.TestAgent.GetMetrics())
		}
		if c.BaselineAgent != nil && other.BaselineAgent != nil {
			c.BaselineAgent.MergeMetrics(other.BaselineAgent.GetMetrics())
		}
	case ThreeWay:
		if c.Agent1 != nil && other.Agent1 != nil {
			c.Agent1.MergeMetrics(other.Agent1.GetMetrics())
		}
		if c.Agent2 != nil && other.Agent2 != nil {
			c.Agent2.MergeMetrics(other.Agent2.GetMetrics())
		}
		if c.Agent3 != nil && other.Agent3 != nil {
			c.Agent3.MergeMetrics(other.Agent3.GetMetrics())
		}
	}
}

// PlayFullGame plays a complete game from deal to finish.
// The game is played with proper agent positioning based on the AgentConfig.
func PlayFullGame(config AgentConfig) *game.GameState {
	g := game.NewGame()
	g = g.WithTestPlayers()
	g = g.WithCardsDealt()
	PlayGameToCompletion(g, config)
	return g
}

// PlayGameWithMode plays a game with a pre-configured declarer, hand, and game mode.
// This is useful for testing specific scenarios.
func PlayGameWithMode(config AgentConfig, declarerHand game.Cards, mode game.GameMode, trumpSuit game.Suit) *game.GameState {
	g := game.NewGame()
	g = g.WithTestPlayers()
	g = g.WithPlayerHand(game.Speaker, declarerHand)
	g = g.WithDeclarer(game.Speaker, 0)
	g = g.WithSkatPickedUp(false)
	g = g.WithGame(mode, trumpSuit)
	PlayGameToCompletion(g, config)
	return g
}

// PlayGameToCompletion plays out a game with the specified agent configuration.
func PlayGameToCompletion(g *game.GameState, config AgentConfig) {
	// Initialize agents array based on configuration
	agents := make([]*agent.SkatAgent, 3)
	testPos := config.GameNum % 3

	switch config.Mode {
	case TestAgainstTwoAgents:
		// Test agent bids against two baseline agents, rotated by gameNum
		agents[testPos] = config.TestAgent
		agents[(testPos+1)%3] = config.BaselineAgent
		agents[(testPos+2)%3] = config.BaselineAgent.CachedClone()
	case FiftyFiftySplit:
		// All three test agents bid (no baseline during bidding)
		agents[0] = config.TestAgent
		agents[1] = config.TestAgent.CachedClone()
		agents[2] = config.TestAgent.CachedClone().CachedClone()
	case ThreeWay:
		// Three different agents
		agents[0] = config.Agent1
		agents[1] = config.Agent2
		agents[2] = config.Agent3
	}

	// Reset stateful strategies on all agents for a fresh game
	for _, a := range agents {
		a.OnGameStart()
	}

	// Bidding phase
	for g.Phase == game.PhaseBidding {
		currentAgent := agents[g.CurrentPlayer]
		accept := currentAgent.Bid(g)
		_, err := g.Bid(accept)
		if err != nil {
			panic(fmt.Sprintf("Bid error: %v", err))
		}
	}

	// After bidding, set up agents for cardplay based on configuration
	if g.Phase == game.PhaseSkatExchange && g.Declarer != nil {
		declarerPos := int(*g.Declarer)

		switch config.Mode {
		case TestAgainstTwoAgents:
			// If baseline became declarer, swap in test agents as defenders
			if agents[declarerPos] == config.BaselineAgent || agents[declarerPos] == config.BaselineAgent.CachedClone() {
				agents[(declarerPos+1)%3] = config.TestAgent
				agents[(declarerPos+2)%3] = config.TestAgent.CachedClone()
			}
		case FiftyFiftySplit:
			// Alternate based on gameNum: even games = test as declarer, odd games = baseline as declarer
			if config.GameNum%2 == 0 {
				// Want test agent as declarer - fill defenders with baseline
				agents[declarerPos] = config.TestAgent
				agents[(declarerPos+1)%3] = config.BaselineAgent
				agents[(declarerPos+2)%3] = config.BaselineAgent.CachedClone()
			} else {
				// Want baseline as declarer, test as defender
				agents[declarerPos] = config.BaselineAgent
				agents[(declarerPos+1)%3] = config.TestAgent
				agents[(declarerPos+2)%3] = config.TestAgent.CachedClone()
			}
		case ThreeWay:
			// No repositioning needed - agents stay as they are
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
	if g.Declarer != nil && !(g.SpeakerPassed && g.ListenerPassed && g.DealerPassed) {
		playerResults := g.PlayerResults()
		if playerResults != nil {
			// Record metrics based on configuration mode
			switch config.Mode {
			case TestAgainstTwoAgents, FiftyFiftySplit:
				if config.TestAgent != nil && config.BaselineAgent != nil {
					for pos := 0; pos < 3; pos++ {
						agent := agents[pos]
						// Check if this is the test agent or a clone of it
						if agent == config.TestAgent || agent == config.TestAgent.CachedClone() || agent == config.TestAgent.CachedClone().CachedClone() {
							config.TestAgent.RecordGameResult(g, playerResults[pos])
						} else {
							// This is a baseline agent or clone
							config.BaselineAgent.RecordGameResult(g, playerResults[pos])
						}
					}
				}
			case ThreeWay:
				// Record for each agent separately
				if config.Agent1 != nil {
					config.Agent1.RecordGameResult(g, playerResults[0])
				}
				if config.Agent2 != nil {
					config.Agent2.RecordGameResult(g, playerResults[1])
				}
				if config.Agent3 != nil {
					config.Agent3.RecordGameResult(g, playerResults[2])
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

// EvaluateAgents runs evaluation games in parallel with the specified agent configuration.
// Each game is played once, with agents properly positioned based on configuration.
// Agents collect their own metrics internally.
func EvaluateAgents(config AgentConfig, games int) *EvaluationStats {
	numWorkers := runtime.GOMAXPROCS(0)

	// Enable metrics on all agents
	config.EnableMetrics()

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

			// Clone all agents for this worker to avoid mutex contention on neural networks
			localConfig := config.CloneAll()
			localConfig.EnableMetrics()

			for gameNum := range gameChan {
				// Set game number in config
				localConfig.GameNum = gameNum

				// Play game with the specified configuration
				g := PlayFullGame(localConfig)

				// Track passed games
				if g.Declarer == nil || (g.SpeakerPassed && g.ListenerPassed && g.DealerPassed) {
					passedGamesAtomic.Add(1)
				}
			}

			// Merge local agent metrics back to main agents
			config.MergeMetrics(localConfig)
		}()
	}

	wg.Wait()

	// Get final metrics snapshots based on mode
	stats := &EvaluationStats{
		PassedGames:    passedGamesAtomic.Load(),
		GamesCompleted: int64(games),
	}

	switch config.Mode {
	case TestAgainstTwoAgents, FiftyFiftySplit:
		testMetrics := config.TestAgent.GetMetrics()
		baselineMetrics := config.BaselineAgent.GetMetrics()

		stats.TestWins = testMetrics.Wins
		stats.TestGames = testMetrics.Games
		stats.TestPoints = testMetrics.Points
		stats.TestOverbid = testMetrics.Overbid
		stats.BaselineWins = baselineMetrics.Wins
		stats.BaselineGames = baselineMetrics.Games
		stats.BaselinePoints = baselineMetrics.Points
		stats.BaselineOverbid = baselineMetrics.Overbid
		stats.TestGrandGames = testMetrics.GrandGames
		stats.TestGrandWins = testMetrics.GrandWins
		stats.TestSuitGames = testMetrics.SuitGames
		stats.TestSuitWins = testMetrics.SuitWins
		stats.TestNullGames = testMetrics.NullGames
		stats.TestNullWins = testMetrics.NullWins
		stats.BaselineGrandGames = baselineMetrics.GrandGames
		stats.BaselineGrandWins = baselineMetrics.GrandWins
		stats.BaselineSuitGames = baselineMetrics.SuitGames
		stats.BaselineSuitWins = baselineMetrics.SuitWins
		stats.BaselineNullGames = baselineMetrics.NullGames
		stats.BaselineNullWins = baselineMetrics.NullWins
	case ThreeWay:
		// For three-way mode, we don't have test/baseline distinction
		// Could extend EvaluationStats to support this if needed
	}

	return stats
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
