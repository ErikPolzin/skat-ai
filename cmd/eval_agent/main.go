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
	testType := flag.String("test", "bidding", "Agent type to test: bidding, game-choice, combined, or all")
	games := flag.Int("games", 500, "Number of evaluation games")
	flag.Parse()

	// Handle "all" flag - run all three evaluations
	if *testType == "all" {
		runEvaluation("bidding", *games)
		fmt.Println()
		runEvaluation("game-choice", *games)
		fmt.Println()
		runEvaluation("combined", *games)
		return
	}

	runEvaluation(*testType, *games)
}

func runEvaluation(testType string, games int) {
	var testAgent *agent.SkatAgent
	var testDescription string

	switch testType {
	case "bidding":
		fmt.Println("Bidding Strategy Evaluation")
		fmt.Println("============================")
		testAgent = createBiddingAgent()
		testDescription = "Q-learning bidding + Heuristic game choice/play"

	case "game-choice":
		fmt.Println("Game Choice Strategy Evaluation")
		fmt.Println("================================")
		testAgent = createGameChoiceAgent()
		testDescription = "Heuristic bidding + Q-learning game choice + Heuristic play"

	case "combined":
		fmt.Println("Combined Q-Learning Agent Evaluation")
		fmt.Println("=====================================")
		testAgent = createCombinedAgent()
		testDescription = "Q-learning bidding + Q-learning game choice + Heuristic play"

	default:
		fmt.Printf("Unknown test type: %s\n", testType)
		fmt.Println("Valid options: bidding, game-choice, combined, all")
		os.Exit(1)
	}

	fmt.Printf("Test agent: %s\n", testDescription)

	// Baseline agent: All heuristic
	baselineAgent := agent.NewHeuristicAgent("Baseline")
	fmt.Println("Baseline agent: All heuristic")

	numWorkers := runtime.GOMAXPROCS(0)

	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Printf("Running %d games on %d CPU cores...\n", games, numWorkers)
	fmt.Println(strings.Repeat("=", 50) + "\n")

	var testWinsAtomic atomic.Int64
	var testGamesAtomic atomic.Int64
	var testPointsAtomic atomic.Int64
	var testOverbidAtomic atomic.Int64
	var baselineWinsAtomic atomic.Int64
	var baselineGamesAtomic atomic.Int64
	var baselinePointsAtomic atomic.Int64
	var baselineOverbidAtomic atomic.Int64
	var gamesCompletedAtomic atomic.Int64
	var passedGamesAtomic atomic.Int64

	// Game type tracking for test agent
	var testGrandGamesAtomic atomic.Int64
	var testGrandWinsAtomic atomic.Int64
	var testSuitGamesAtomic atomic.Int64
	var testSuitWinsAtomic atomic.Int64
	var testNullGamesAtomic atomic.Int64
	var testNullWinsAtomic atomic.Int64

	// Game type tracking for baseline agent
	var baselineGrandGamesAtomic atomic.Int64
	var baselineGrandWinsAtomic atomic.Int64
	var baselineSuitGamesAtomic atomic.Int64
	var baselineSuitWinsAtomic atomic.Int64
	var baselineNullGamesAtomic atomic.Int64
	var baselineNullWinsAtomic atomic.Int64

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
	gameChan := make(chan int, games)

	for i := 0; i < games; i++ {
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
					if g.Overbid {
						testOverbidAtomic.Add(1)
					}
					// Track game type
					switch g.Mode {
					case game.ModeGrand:
						testGrandGamesAtomic.Add(1)
						if result.DeclarerWon {
							testGrandWinsAtomic.Add(1)
						}
					case game.ModeSuit:
						testSuitGamesAtomic.Add(1)
						if result.DeclarerWon {
							testSuitWinsAtomic.Add(1)
						}
					case game.ModeNull:
						testNullGamesAtomic.Add(1)
						if result.DeclarerWon {
							testNullWinsAtomic.Add(1)
						}
					}
				} else {
					baselineGamesAtomic.Add(1)
					baselinePointsAtomic.Add(int64(result.Value))
					if result.DeclarerWon {
						baselineWinsAtomic.Add(1)
					}
					if g.Overbid {
						baselineOverbidAtomic.Add(1)
					}
					// Track game type
					switch g.Mode {
					case game.ModeGrand:
						baselineGrandGamesAtomic.Add(1)
						if result.DeclarerWon {
							baselineGrandWinsAtomic.Add(1)
						}
					case game.ModeSuit:
						baselineSuitGamesAtomic.Add(1)
						if result.DeclarerWon {
							baselineSuitWinsAtomic.Add(1)
						}
					case game.ModeNull:
						baselineNullGamesAtomic.Add(1)
						if result.DeclarerWon {
							baselineNullWinsAtomic.Add(1)
						}
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
	testOverbid := testOverbidAtomic.Load()
	baselineGames := baselineGamesAtomic.Load()
	baselineWins := baselineWinsAtomic.Load()
	baselinePoints := baselinePointsAtomic.Load()
	baselineOverbid := baselineOverbidAtomic.Load()
	passedGames := passedGamesAtomic.Load()

	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("FINAL RESULTS")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Printf("\nPassed games (everyone passed): %d/%d (%.1f%%)\n",
		passedGames, games, float64(passedGames)/float64(games)*100)

	if testGames > 0 {
		fmt.Printf("\nTest (%s):\n", testDescription)
		fmt.Printf("  Win rate: %.1f%% (%d/%d games as declarer)\n",
			float64(testWins)/float64(testGames)*100, testWins, testGames)
		fmt.Printf("  Avg points as declarer: %.1f\n", float64(testPoints)/float64(testGames))
		fmt.Printf("  Overbid rate: %.1f%% (%d/%d)\n",
			float64(testOverbid)/float64(testGames)*100, testOverbid, testGames)

		// Unseen states tracking for Q-learning strategies
		if qStrat, ok := testAgent.GetBiddingStrategy().(*agent.QLearningBiddingStrategy); ok {
			unseenStates, totalBids := qStrat.GetMetrics()
			if totalBids > 0 {
				fmt.Printf("  Bidding states not in Q-table: %d/%d (%.1f%% unseen)\n",
					unseenStates, totalBids, float64(unseenStates)/float64(totalBids)*100)
			}
		}
		if qStrat, ok := testAgent.GetGameChoiceStrategy().(*agent.QLearningGameChoiceStrategy); ok {
			unseenStates, totalChoices := qStrat.GetMetrics()
			if totalChoices > 0 {
				fmt.Printf("  Game choice states not in Q-table: %d/%d (%.1f%% unseen)\n",
					unseenStates, totalChoices, float64(unseenStates)/float64(totalChoices)*100)
			}
		}

		// Game type breakdown
		testGrand := testGrandGamesAtomic.Load()
		testGrandW := testGrandWinsAtomic.Load()
		testSuit := testSuitGamesAtomic.Load()
		testSuitW := testSuitWinsAtomic.Load()
		testNull := testNullGamesAtomic.Load()
		testNullW := testNullWinsAtomic.Load()

		fmt.Printf("  Game type breakdown:\n")
		if testGrand > 0 {
			fmt.Printf("    Grand: %d games, %.1f%% win rate (%d wins)\n",
				testGrand, float64(testGrandW)/float64(testGrand)*100, testGrandW)
		}
		if testSuit > 0 {
			fmt.Printf("    Suit:  %d games, %.1f%% win rate (%d wins)\n",
				testSuit, float64(testSuitW)/float64(testSuit)*100, testSuitW)
		}
		if testNull > 0 {
			fmt.Printf("    Null:  %d games, %.1f%% win rate (%d wins)\n",
				testNull, float64(testNullW)/float64(testNull)*100, testNullW)
		}
	}

	if baselineGames > 0 {
		fmt.Printf("\nBaseline (Heuristic):\n")
		fmt.Printf("  Win rate: %.1f%% (%d/%d games as declarer)\n",
			float64(baselineWins)/float64(baselineGames)*100, baselineWins, baselineGames)
		fmt.Printf("  Avg points as declarer: %.1f\n", float64(baselinePoints)/float64(baselineGames))
		fmt.Printf("  Overbid rate: %.1f%% (%d/%d)\n",
			float64(baselineOverbid)/float64(baselineGames)*100, baselineOverbid, baselineGames)

		// Game type breakdown
		baseGrand := baselineGrandGamesAtomic.Load()
		baseGrandW := baselineGrandWinsAtomic.Load()
		baseSuit := baselineSuitGamesAtomic.Load()
		baseSuitW := baselineSuitWinsAtomic.Load()
		baseNull := baselineNullGamesAtomic.Load()
		baseNullW := baselineNullWinsAtomic.Load()

		fmt.Printf("  Game type breakdown:\n")
		if baseGrand > 0 {
			fmt.Printf("    Grand: %d games, %.1f%% win rate (%d wins)\n",
				baseGrand, float64(baseGrandW)/float64(baseGrand)*100, baseGrandW)
		}
		if baseSuit > 0 {
			fmt.Printf("    Suit:  %d games, %.1f%% win rate (%d wins)\n",
				baseSuit, float64(baseSuitW)/float64(baseSuit)*100, baseSuitW)
		}
		if baseNull > 0 {
			fmt.Printf("    Null:  %d games, %.1f%% win rate (%d wins)\n",
				baseNull, float64(baseNullW)/float64(baseNull)*100, baseNullW)
		}
	}

	if testGames > 0 && baselineGames > 0 {
		improvement := (float64(testWins)/float64(testGames) - float64(baselineWins)/float64(baselineGames)) * 100
		pointDiff := float64(testPoints)/float64(testGames) - float64(baselinePoints)/float64(baselineGames)
		fmt.Printf("\nImprovement: %+.1f percentage points\n", improvement)
		fmt.Printf("Point difference: %+.1f points per game\n", pointDiff)
	}

	// Show example hand decisions for Q-learning strategies
	if strings.Contains(testDescription, "bidding") || strings.Contains(testDescription, "Q-learning bidding") || strings.Contains(testDescription, "combined") {
		fmt.Println("\n" + strings.Repeat("=", 50))
		fmt.Println("EXAMPLE BIDDING DECISIONS")
		fmt.Println(strings.Repeat("=", 50))
		testExampleBiddingHands(testAgent)
	}

	if strings.Contains(testDescription, "game choice") || strings.Contains(testDescription, "Q-learning game choice") || strings.Contains(testDescription, "combined") {
		fmt.Println("\n" + strings.Repeat("=", 50))
		fmt.Println("EXAMPLE GAME CHOICE DECISIONS")
		fmt.Println(strings.Repeat("=", 50))
		testExampleGameChoiceHands(testAgent)
	}
}

func testExampleBiddingHands(testAgent *agent.SkatAgent) {
	testCases := []struct {
		name        string
		handStr     string
		expectedBid string
		reason      string
	}{
		{
			name:        "Strong Hand - All 4 Jacks",
			handStr:     "J.♣-J.♠-J.♥-J.♦-A.♥-10.♠-A.♦-K.♣-Q.♥-9.♣",
			expectedBid: "High (60+)",
			reason:      "4 jacks + 2 aces - can play Grand with 5",
		},
		{
			name:        "Medium Hand - 2 Jacks + Strong Clubs",
			handStr:     "J.♣-J.♠-A.♣-10.♣-K.♣-Q.♣-9.♣-7.♥-8.♦-9.♠",
			expectedBid: "Medium (30-40)",
			reason:      "7 clubs with A+10 - safe Clubs game",
		},
		{
			name:        "Weak Hand - 1 Jack + Short Suits",
			handStr:     "J.♣-K.♥-Q.♥-9.♣-8.♣-Q.♠-9.♠-7.♥-8.♥-7.♦",
			expectedBid: "Low (18-23)",
			reason:      "Only 1 jack, no long suit - risky",
		},
		{
			name:        "Borderline - 3 Jacks but weak",
			handStr:     "J.♣-J.♠-J.♥-K.♦-Q.♣-9.♣-8.♣-7.♠-8.♠-7.♥",
			expectedBid: "Medium (30-40)",
			reason:      "3 jacks but no aces/tens - moderate",
		},
		{
			name:        "Strong Suit - Long Hearts",
			handStr:     "J.♥-J.♦-A.♥-10.♥-K.♥-Q.♥-9.♥-A.♣-10.♠-8.♦",
			expectedBid: "High (40-50)",
			reason:      "7 hearts with A+10+K+Q - very strong",
		},
	}

	biddingStrat := testAgent.GetBiddingStrategy()
	heuristic := &agent.HeuristicBiddingStrategy{}

	// Create a mock game state for testing
	g := game.NewGame()
	for i := 0; i < 3; i++ {
		g.Players[i] = &game.PlayerState{
			ID:      fmt.Sprintf("player-%d", i),
			Name:    fmt.Sprintf("Player %d", i),
			Hand:    []game.Card{},
			IsAgent: true,
		}
	}
	g.Phase = game.PhaseBidding
	g.CurrentPlayer = 0

	for _, tc := range testCases {
		hand, err := game.ParseCards(tc.handStr)
		if err != nil || len(hand) != 10 {
			continue
		}

		g.Players[0].Hand = hand

		fmt.Printf("\n%s:\n", tc.name)
		fmt.Printf("  %s\n", tc.reason)
		fmt.Printf("  Expected: %s\n", tc.expectedBid)

		// Test various bid levels
		bidLevels := []int{18, 20, 23, 24, 27, 30, 33, 36, 40, 44, 48, 50, 55, 59, 60}
		qAccepts := []int{}
		hAccepts := []int{}

		for _, bid := range bidLevels {
			if qStrat, ok := biddingStrat.(*agent.QLearningBiddingStrategy); ok {
				if qStrat.ShouldBid(g, hand, bid) {
					qAccepts = append(qAccepts, bid)
				}
			}
			if heuristic.ShouldBid(g, hand, bid) {
				hAccepts = append(hAccepts, bid)
			}
		}

		qMax := 0
		if len(qAccepts) > 0 {
			qMax = qAccepts[len(qAccepts)-1]
		}
		hMax := 0
		if len(hAccepts) > 0 {
			hMax = hAccepts[len(hAccepts)-1]
		}

		fmt.Printf("  Q-learning bids up to: %d\n", qMax)
		fmt.Printf("  Heuristic bids up to:  %d", hMax)
		if qMax == hMax {
			fmt.Printf(" ✓\n")
		} else {
			fmt.Printf(" (diff: %+d)\n", qMax-hMax)
		}
	}
	fmt.Println()
}

func testExampleGameChoiceHands(testAgent *agent.SkatAgent) {
	testCases := []struct {
		name     string
		handStr  string
		bidValue int
		reason   string
	}{
		{
			name:     "Strong Clubs Suit",
			handStr:  "J.♣-J.♠-A.♣-10.♣-K.♣-Q.♣-9.♣-7.♥-8.♦-9.♠",
			bidValue: 24,
			reason:   "7 clubs with A+10+K+Q - should prefer Clubs over Grand",
		},
		{
			name:     "All Four Jacks",
			handStr:  "J.♣-J.♠-J.♥-J.♦-A.♥-10.♠-A.♦-K.♣-Q.♥-9.♣",
			bidValue: 48,
			reason:   "4 jacks + scattered aces - ideal for Grand",
		},
		{
			name:     "Long Hearts",
			handStr:  "J.♥-K.♥-Q.♥-9.♥-8.♥-7.♥-A.♣-10.♠-8.♦-7.♣",
			bidValue: 20,
			reason:   "6 hearts - length over high cards",
		},
		{
			name:     "Only Club Jack",
			handStr:  "J.♣-A.♦-10.♦-K.♦-Q.♦-9.♦-7.♥-8.♠-9.♣-7.♣",
			bidValue: 18,
			reason:   "5 diamonds with A+10 - suit over Grand despite low jacks",
		},
	}

	gameChoice := testAgent.GetGameChoiceStrategy()
	heuristic := &agent.HeuristicGameChoiceStrategy{}

	for _, tc := range testCases {
		hand, err := game.ParseCards(tc.handStr)
		if err != nil || len(hand) != 10 {
			continue
		}

		qMode, qSuit := gameChoice.ChooseGame(hand, tc.bidValue)
		hMode, hSuit := heuristic.ChooseGame(hand, tc.bidValue)

		qChoice := formatGameChoice(qMode, qSuit)
		hChoice := formatGameChoice(hMode, hSuit)

		fmt.Printf("\n%s:\n", tc.name)
		fmt.Printf("  %s\n", tc.reason)
		fmt.Printf("  Q-learning: %s\n", qChoice)
		fmt.Printf("  Heuristic:  %s", hChoice)
		if qChoice != hChoice {
			fmt.Printf(" ✗\n")
		} else {
			fmt.Printf(" ✓\n")
		}
	}
	fmt.Println()
}

func formatGameChoice(mode game.GameMode, suit game.Suit) string {
	if mode == game.ModeGrand {
		return "Grand"
	} else if mode == game.ModeNull {
		return "Null"
	}
	return suit.String()
}

func createBiddingAgent() *agent.SkatAgent {
	qtablePath := "bidding_qtable.gob"
	fmt.Printf("Loading bidding Q-table from %s...\n", qtablePath)

	if _, err := os.Stat(qtablePath); os.IsNotExist(err) {
		fmt.Printf("Error: Q-table file not found: %s\n", qtablePath)
		fmt.Println("Please train the agent first using: go run cmd/train_bidding/main.go")
		os.Exit(1)
	}

	data, err := agent.LoadQTableData(qtablePath, true)
	if err != nil {
		fmt.Printf("Error loading Q-table: %v\n", err)
		os.Exit(1)
	}

	// Analyze Q-table
	analyzeQTable(data.QTable, "Bidding")

	// Create Q-learning bidding strategy and load trained Q-table
	qBidding := agent.NewQLearningBiddingStrategy(0.0)
	qBidding.SetQTable(data.QTable)
	qBidding.SetEpsilon(0.0)
	qBidding.EnableMetrics() // Enable metrics for evaluation

	return agent.NewAgentWithStrategies(
		"Test",
		qBidding,
		&agent.HeuristicGameChoiceStrategy{},
		&agent.HeuristicCardPlayStrategy{},
	)
}

func binomialCoeff(n, k int) int {
	if k > n {
		return 0
	}
	if k == 0 || k == n {
		return 1
	}
	result := 1
	for i := 0; i < k; i++ {
		result *= (n - i)
		result /= (i + 1)
	}
	return result
}

func analyzeQTable(qtable map[int]map[int]float64, name string) {
	if len(qtable) == 0 {
		fmt.Printf("⚠ Q-table is empty!\n\n")
		return
	}

	// Count states and state-action pairs
	numStates := len(qtable)
	numStateActions := 0
	minQ := 999999.0
	maxQ := -999999.0
	sumQ := 0.0
	actionCounts := make(map[int]int)

	for _, actions := range qtable {
		numStateActions += len(actions)
		for action, qval := range actions {
			actionCounts[action]++
			if qval < minQ {
				minQ = qval
			}
			if qval > maxQ {
				maxQ = qval
			}
			sumQ += qval
		}
	}

	avgQ := sumQ / float64(numStateActions)

	fmt.Printf("\n%s Q-table Statistics:\n", name)
	fmt.Printf("  States learned: %d\n", numStates)

	// Calculate theoretical and practical state space
	if name == "Game Choice" {
		// Theoretical: 8×8×8×8×5×9 (if all combinations were possible)
		theoreticalMax := 8 * 8 * 8 * 8 * 5 * 9

		// Practical: ways to distribute 8 cards into 4 suits × 5 jack counts (0-4) × 9 high card counts
		// C(8+4-1, 4-1) = C(11, 3) = 165 ways to distribute 8 cards
		practicalMax := binomialCoeff(11, 3) * 5 * 9

		fmt.Printf("  Theoretical state space: %d\n", theoreticalMax)
		fmt.Printf("  Practical state space: %d (accounting for constraints)\n", practicalMax)
		fmt.Printf("  Coverage (theoretical): %.1f%%\n", float64(numStates)/float64(theoreticalMax)*100)
		fmt.Printf("  Coverage (practical): %.1f%%\n", float64(numStates)/float64(practicalMax)*100)
	} else if name == "Bidding" {
		// Bidding has 16,500 theoretical states
		theoreticalMax := 16500
		fmt.Printf("  Theoretical state space: %d\n", theoreticalMax)
		fmt.Printf("  Coverage: %.1f%%\n", float64(numStates)/float64(theoreticalMax)*100)
	}

	fmt.Printf("  State-action pairs: %d\n", numStateActions)
	fmt.Printf("  Avg actions per state: %.1f\n", float64(numStateActions)/float64(numStates))
	fmt.Printf("  Q-value range: [%.3f, %.3f]\n", minQ, maxQ)
	fmt.Printf("  Average Q-value: %.3f\n", avgQ)

	// Show action distribution
	fmt.Printf("  Action distribution:\n")
	for action, count := range actionCounts {
		fmt.Printf("    Action %d: %d times (%.1f%%)\n",
			action, count, float64(count)/float64(numStateActions)*100)
	}
	fmt.Println()
}

func createGameChoiceAgent() *agent.SkatAgent {
	qtablePath := "game_choice_qtable.gob"
	fmt.Printf("Loading game choice Q-table from %s...\n", qtablePath)

	if _, err := os.Stat(qtablePath); os.IsNotExist(err) {
		fmt.Printf("Error: Q-table file not found: %s\n", qtablePath)
		fmt.Println("Please train the agent first using: go run cmd/train_game_choice/main.go")
		os.Exit(1)
	}

	data, err := agent.LoadQTableData(qtablePath, true)
	if err != nil {
		fmt.Printf("Error loading Q-table: %v\n", err)
		os.Exit(1)
	}

	// Analyze Q-table
	analyzeQTable(data.QTable, "Game Choice")

	// Create Q-learning game choice strategy and load trained Q-table
	qGameChoice := agent.NewQLearningGameChoiceStrategy(0.0)
	qGameChoice.SetQTable(data.QTable)
	qGameChoice.SetEpsilon(0.0)
	qGameChoice.EnableMetrics() // Enable metrics for evaluation

	return agent.NewAgentWithStrategies(
		"Test",
		&agent.HeuristicBiddingStrategy{},
		qGameChoice,
		&agent.HeuristicCardPlayStrategy{},
	)
}

func createCombinedAgent() *agent.SkatAgent {
	// Load bidding Q-table
	biddingPath := "bidding_qtable.gob"
	fmt.Printf("Loading bidding Q-table from %s...\n", biddingPath)

	if _, err := os.Stat(biddingPath); os.IsNotExist(err) {
		fmt.Printf("Error: Q-table file not found: %s\n", biddingPath)
		fmt.Println("Please train the agent first using: go run cmd/train_bidding/main.go")
		os.Exit(1)
	}

	biddingData, err := agent.LoadQTableData(biddingPath, true)
	if err != nil {
		fmt.Printf("Error loading bidding Q-table: %v\n", err)
		os.Exit(1)
	}

	// Analyze bidding Q-table
	analyzeQTable(biddingData.QTable, "Bidding")

	// Load game choice Q-table
	gameChoicePath := "game_choice_qtable.gob"
	fmt.Printf("Loading game choice Q-table from %s...\n", gameChoicePath)

	if _, err := os.Stat(gameChoicePath); os.IsNotExist(err) {
		fmt.Printf("Error: Q-table file not found: %s\n", gameChoicePath)
		fmt.Println("Please train the agent first using: go run cmd/train_game_choice/main.go")
		os.Exit(1)
	}

	gameChoiceData, err := agent.LoadQTableData(gameChoicePath, true)
	if err != nil {
		fmt.Printf("Error loading game choice Q-table: %v\n", err)
		os.Exit(1)
	}

	// Analyze game choice Q-table
	analyzeQTable(gameChoiceData.QTable, "Game Choice")

	// Create Q-learning strategies and load trained Q-tables
	qBidding := agent.NewQLearningBiddingStrategy(0.0)
	qBidding.SetQTable(biddingData.QTable)
	qBidding.SetEpsilon(0.0)
	qBidding.EnableMetrics() // Enable metrics for evaluation

	qGameChoice := agent.NewQLearningGameChoiceStrategy(0.0)
	qGameChoice.SetQTable(gameChoiceData.QTable)
	qGameChoice.SetEpsilon(0.0)
	qGameChoice.EnableMetrics() // Enable metrics for evaluation

	return agent.NewAgentWithStrategies(
		"Test",
		qBidding,
		qGameChoice,
		&agent.HeuristicCardPlayStrategy{},
	)
}
