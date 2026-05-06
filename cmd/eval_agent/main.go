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
)

func main() {
	agentType := flag.String("agent-type", "neural", "Agent type: neural, mcts, minimax, or heuristic")
	component := flag.String("component", "bidding", "Component to test: bidding, game-choice, card-play, or combined")
	games := flag.Int("games", 500, "Number of evaluation games")
	cardplayWeights := flag.String("cardplay-weights", ".data/models/cardplay_weights.bin", "Path to card play neural network weights")
	threshold := flag.Float64("threshold", 0.6, "Weighted heuristic bidding threshold (0.5-0.7)")
	minimaxDepth := flag.Int("minimax-depth", 10, "Minimax search depth for perfect-info minimax")
	flag.Parse()

	runEvaluation(*agentType, *component, *games, *cardplayWeights, *threshold, *minimaxDepth)
}

func runEvaluation(agentType, component string, games int, cardplayWeights string, threshold float64, minimaxDepth int) {
	// Validate agent type
	if agentType != "qlearning" && agentType != "neural" && agentType != "weighted" && agentType != "heuristic" && agentType != "mcts" && agentType != "minimax" {
		fmt.Printf("Unknown agent type: %s\n", agentType)
		fmt.Println("Valid options: qlearning, neural, weighted, heuristic, mcts, minimax")
		os.Exit(1)
	}

	// Validate component
	if component != "bidding" && component != "game-choice" && component != "card-play" && component != "combined" {
		fmt.Printf("Unknown component: %s\n", component)
		fmt.Println("Valid options: bidding, game-choice, card-play, combined")
		os.Exit(1)
	}

	// Print evaluation header
	printEvaluationHeader(agentType, component)

	// Build agent configuration based on type and component
	config := buildAgentConfig(agentType, component, threshold, cardplayWeights, minimaxDepth)

	testAgent, err := agent.NewHybridAgent("Test", config)
	if err != nil {
		fmt.Printf("Error creating test agent: %v\n", err)
		os.Exit(1)
	}

	testDescription := buildAgentDescription(agentType, component, threshold)

	fmt.Printf("Test agent: %s\n", testDescription)

	// Baseline agent: All heuristic
	baselineAgent := agent.NewHeuristicAgent("Baseline")
	fmt.Println("Baseline agent: All heuristic")

	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Printf("Running %d games on %d CPU cores...\n", games, runtime.GOMAXPROCS(0))
	fmt.Println(strings.Repeat("=", 50) + "\n")

	evalConfig := agent.NewFiftyFiftySplitConfig(testAgent, baselineAgent)
	training.EvaluateAgents(evalConfig, games)

	// Get agent metrics for bidding distribution
	testMetrics := testAgent.GetMetrics()
	baselineMetrics := baselineAgent.GetMetrics()

	testGames := testMetrics.Games
	testWins := testMetrics.Wins
	testPoints := testMetrics.Points
	testOverbid := testMetrics.Overbid
	baselineGames := baselineMetrics.Games
	baselineWins := baselineMetrics.Wins
	baselinePoints := baselineMetrics.Points
	baselineOverbid := baselineMetrics.Overbid

	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("FINAL RESULTS")
	fmt.Println(strings.Repeat("=", 50))

	if testGames > 0 || testMetrics.PassedGames > 0 {
		fmt.Printf("\nTest (%s):\n", testDescription)
		if testGames > 0 {
			fmt.Printf("  Declarer win rate: %.1f%% (%d/%d games as declarer)\n",
				float64(testWins)/float64(testGames)*100, testWins, testGames)
			fmt.Printf("  Avg points as declarer: %.1f\n", float64(testPoints)/float64(testGames))
			fmt.Printf("  Overbid rate: %.1f%% (%d/%d)\n",
				float64(testOverbid)/float64(testGames)*100, testOverbid, testGames)
		}
		if testMetrics.PassedGames > 0 {
			fmt.Printf("  Passed games: %d (all players passed)\n", testMetrics.PassedGames)
		}

		// Defender stats
		if testMetrics.DefenderGames > 0 {
			fmt.Printf("  Defender win rate: %.1f%% (%d/%d games as defender)\n",
				float64(testMetrics.DefenderWins)/float64(testMetrics.DefenderGames)*100,
				testMetrics.DefenderWins, testMetrics.DefenderGames)
		}

		// Game type breakdown
		testGrand := testMetrics.GrandGames
		testGrandW := testMetrics.GrandWins
		testSuit := testMetrics.SuitGames
		testSuitW := testMetrics.SuitWins
		testNull := testMetrics.NullGames
		testNullW := testMetrics.NullWins

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

		// Bidding distribution
		totalBids := testMetrics.GetTotalBids()
		if totalBids > 0 {
			maxBid := testMetrics.GetMaxBid()
			fmt.Printf("  Bidding distribution:\n")
			fmt.Printf("    Max bid accepted: %d\n", maxBid)
			fmt.Printf("    Total bidding decisions: %d\n", totalBids)
			displayBiddingDistribution(testMetrics.BiddingAccepts, testMetrics.BiddingRejects)
		}
	}

	if baselineGames > 0 || baselineMetrics.PassedGames > 0 {
		fmt.Printf("\nBaseline (Heuristic):\n")
		if baselineGames > 0 {
			fmt.Printf("  Declarer win rate: %.1f%% (%d/%d games as declarer)\n",
				float64(baselineWins)/float64(baselineGames)*100, baselineWins, baselineGames)
			fmt.Printf("  Avg points as declarer: %.1f\n", float64(baselinePoints)/float64(baselineGames))
			fmt.Printf("  Overbid rate: %.1f%% (%d/%d)\n",
				float64(baselineOverbid)/float64(baselineGames)*100, baselineOverbid, baselineGames)
		}
		if baselineMetrics.PassedGames > 0 {
			fmt.Printf("  Passed games: %d (all players passed)\n", baselineMetrics.PassedGames)
		}

		// Defender stats
		if baselineMetrics.DefenderGames > 0 {
			fmt.Printf("  Defender win rate: %.1f%% (%d/%d games as defender)\n",
				float64(baselineMetrics.DefenderWins)/float64(baselineMetrics.DefenderGames)*100,
				baselineMetrics.DefenderWins, baselineMetrics.DefenderGames)
		}

		// Game type breakdown
		baseGrand := baselineMetrics.GrandGames
		baseGrandW := baselineMetrics.GrandWins
		baseSuit := baselineMetrics.SuitGames
		baseSuitW := baselineMetrics.SuitWins
		baseNull := baselineMetrics.NullGames
		baseNullW := baselineMetrics.NullWins

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

		// Bidding distribution
		totalBids := baselineMetrics.GetTotalBids()
		if totalBids > 0 {
			maxBid := baselineMetrics.GetMaxBid()
			fmt.Printf("  Bidding distribution:\n")
			fmt.Printf("    Max bid accepted: %d\n", maxBid)
			fmt.Printf("    Total bidding decisions: %d\n", totalBids)
			displayBiddingDistribution(baselineMetrics.BiddingAccepts, baselineMetrics.BiddingRejects)
		}
	}

	if testGames > 0 && baselineGames > 0 {
		improvement := (float64(testWins)/float64(testGames) - float64(baselineWins)/float64(baselineGames)) * 100
		pointDiff := float64(testPoints)/float64(testGames) - float64(baselinePoints)/float64(baselineGames)
		fmt.Printf("\nImprovement: %+.1f percentage points\n", improvement)
		fmt.Printf("Point difference: %+.1f points per game\n", pointDiff)
	}

	// Show example hand decisions for Q-learning strategies
	if component == "bidding" || component == "combined" {
		fmt.Println("\n" + strings.Repeat("=", 50))
		fmt.Println("EXAMPLE BIDDING DECISIONS")
		fmt.Println(strings.Repeat("=", 50))
		testExampleBiddingHands(testAgent)
	}

	if component == "game-choice" || component == "combined" {
		fmt.Println("\n" + strings.Repeat("=", 50))
		fmt.Println("EXAMPLE GAME CHOICE DECISIONS")
		fmt.Println(strings.Repeat("=", 50))
		testExampleGameChoiceHands(testAgent)
	}

	if (component == "card-play" || component == "combined") && agentType != "minimax" {
		// Run game-play test with known winning games (skip for minimax - too slow)
		fmt.Println("\n" + strings.Repeat("=", 50))
		fmt.Println("EXAMPLE GAME PLAY RESULTS")
		fmt.Println(strings.Repeat("=", 50))
		runGamePlayTest(testAgent)
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
			// Test neural or Q-learning strategy
			if biddingStrat.ShouldBid(g, hand, bid) {
				qAccepts = append(qAccepts, bid)
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

		fmt.Printf("  Weighted bids up to: %d\n", qMax)
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

		testMode, testSuit := gameChoice.ChooseGame(hand, tc.bidValue)
		hMode, hSuit := heuristic.ChooseGame(hand, tc.bidValue)

		testChoice := formatGameChoice(testMode, testSuit)
		hChoice := formatGameChoice(hMode, hSuit)

		fmt.Printf("\n%s:\n", tc.name)
		fmt.Printf("  %s\n", tc.reason)
		fmt.Printf("  Test agent: %s\n", testChoice)
		fmt.Printf("  Heuristic:  %s", hChoice)
		if testChoice != hChoice {
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

// printEvaluationHeader prints the appropriate header for the evaluation
func printEvaluationHeader(agentType, component string) {
	switch agentType {
	case "heuristic":
		fmt.Println("Heuristic Agent Evaluation")
		fmt.Println("===========================")
	case "mcts":
		if component == "card-play" {
			fmt.Println("MCTS Card Play Strategy Evaluation")
			fmt.Println("====================================")
		}
	case "neural":
		if component == "card-play" {
			fmt.Println("Neural Card Play Strategy Evaluation")
			fmt.Println("======================================")
		}
	}
}

// buildAgentConfig creates agent configuration based on type and component
func buildAgentConfig(agentType, component string, threshold float64, cardplayWeights string, minimaxDepth int) agent.HybridAgentConfig {
	config := agent.HybridAgentConfig{
		BiddingType:      "weighted",
		BiddingThreshold: threshold,
		GameChoiceType:   "heuristic",
		CardPlayType:     "heuristic",
	}

	// Configure based on agent type and component
	switch agentType {
	case "heuristic":
		// All heuristic - defaults are already set correctly
		config.BiddingType = "heuristic"

	case "weighted":
		// Weighted strategies based on component
		if component == "bidding" || component == "combined" {
			config.BiddingType = "weighted"
			config.BiddingThreshold = threshold
		}
		if component == "combined" {
			config.GameChoiceType = "weighted"
		}

	case "mcts":
		if component == "card-play" || component == "combined" {
			config.CardPlayType = "mcts"
			config.MCTSSimulations = 500
		}
		if component == "combined" {
			config.GameChoiceType = "weighted"
		}

	case "minimax":
		if component == "card-play" || component == "combined" {
			config.CardPlayType = "minimax"
			config.MinimaxDepth = minimaxDepth
		}
		if component == "combined" {
			config.GameChoiceType = "weighted"
		}

	case "neural":
		if component == "card-play" || component == "combined" {
			config.CardPlayType = "neural"
			config.NeuralWeightsPath = cardplayWeights
		}
		if component == "combined" {
			config.GameChoiceType = "weighted"
		}
	}

	return config
}

// buildAgentDescription creates a human-readable description of the agent
func buildAgentDescription(agentType, component string, threshold float64) string {
	switch agentType {
	case "heuristic":
		return "All heuristic (baseline vs baseline)"
	case "weighted":
		if component == "bidding" {
			return fmt.Sprintf("Weighted heuristic bidding (threshold=%.2f) + Heuristic game choice/play", threshold)
		} else if component == "combined" {
			return fmt.Sprintf("Weighted heuristic bidding+game choice (threshold=%.2f) + Heuristic play", threshold)
		}
	case "mcts":
		if component == "card-play" {
			return "Weighted bidding + Heuristic game choice + MCTS card play"
		} else if component == "combined" {
			return fmt.Sprintf("Weighted bidding+game choice (threshold=%.2f) + MCTS card play", threshold)
		}
	case "minimax":
		if component == "card-play" {
			return "Weighted bidding + Heuristic game choice + Minimax card play"
		} else if component == "combined" {
			return fmt.Sprintf("Weighted bidding+game choice (threshold=%.2f) + Minimax card play", threshold)
		}
	case "neural":
		if component == "card-play" {
			return "Weighted bidding + Heuristic game choice + DQN card play"
		} else if component == "combined" {
			return fmt.Sprintf("Weighted bidding+game choice (threshold=%.2f) + DQN card play", threshold)
		}
	}
	return fmt.Sprintf("%s agent testing %s", agentType, component)
}

func loadQLearningBiddingQTable() map[int]map[int]float64 {
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

	return data.QTable
}

func displayBiddingDistribution(accepts map[int]int, rejects map[int]int) {
	// Collect all bid values
	allBids := make(map[int]bool)
	for bid := range accepts {
		allBids[bid] = true
	}
	for bid := range rejects {
		allBids[bid] = true
	}

	// Standard Skat bid sequence
	bidSequence := []int{18, 20, 22, 23, 24, 27, 30, 33, 35, 36, 40, 44, 45, 46, 48, 50, 54, 55, 59, 60}

	// Create sorted list of bids
	type bidData struct {
		bid        int
		accepts    int
		rejects    int
		acceptRate float64
	}

	var bids []bidData
	totalDecisions := 0

	for _, bid := range bidSequence {
		if allBids[bid] {
			acc := accepts[bid]
			rej := rejects[bid]
			total := acc + rej
			acceptRate := 0.0
			if total > 0 {
				acceptRate = float64(acc) / float64(total) * 100
			}
			bids = append(bids, bidData{bid, acc, rej, acceptRate})
			totalDecisions += total
		}
	}

	if len(bids) == 0 {
		return
	}

	// Display horizontal bar chart
	const barWidth = 30
	fmt.Printf("    %-3s  %-30s  %7s  %7s  %6s\n", "Bid", "Distribution", "Accept", "Reject", "Rate")

	for _, b := range bids {
		total := b.accepts + b.rejects
		pct := float64(total) / float64(totalDecisions) * 100

		// Calculate bar length (scale to barWidth based on total decisions)
		barLen := int(pct / 100.0 * barWidth)
		if barLen < 1 && total > 0 {
			barLen = 1
		}

		// Create bar - use different characters for accept vs reject
		acceptLen := 0
		rejectLen := 0
		if total > 0 {
			acceptLen = int(float64(barLen) * float64(b.accepts) / float64(total))
			rejectLen = barLen - acceptLen
		}

		bar := strings.Repeat("█", acceptLen) + strings.Repeat("░", rejectLen)
		// Pad bar to fixed width for alignment
		bar = fmt.Sprintf("%-30s", bar)

		// Format with aligned columns
		fmt.Printf("    %-3d  %s  %7d  %7d  %5.1f%%\n",
			b.bid, bar, b.accepts, b.rejects, b.acceptRate)
	}
}

func loadQLearningGameChoiceQTable() map[int]map[int]float64 {
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

	return data.QTable
}

// runGamePlayTest tests that agents win known games with correct suit choices
func runGamePlayTest(testAgent *agent.SkatAgent) {
	testHands := []struct {
		name        string
		handStr     string
		bestMode    game.GameMode
		bestSuit    game.Suit
		description string
	}{
		{
			name:        "Strong Clubs Hand",
			handStr:     "J.♣-A.♣-10.♣-K.♣-Q.♣-9.♣-8.♣-A.♠-10.♥-K.♦",
			bestMode:    game.ModeSuit,
			bestSuit:    game.Clubs,
			description: "7 Clubs trumps - should win with Clubs, lose with others",
		},
		{
			name:        "Strong Diamonds Hand",
			handStr:     "J.♣-J.♠-A.♦-10.♦-K.♦-Q.♦-9.♦-8.♦-A.♥-10.♠",
			bestMode:    game.ModeSuit,
			bestSuit:    game.Diamonds,
			description: "8 Diamonds trumps - should win with Diamonds, lose with others",
		},
		{
			name:        "Strong Grand Hand",
			handStr:     "J.♣-J.♠-J.♥-J.♦-A.♠-A.♥-A.♦-10.♣-10.♠-10.♥",
			bestMode:    game.ModeGrand,
			bestSuit:    game.Clubs, // Doesn't matter for Grand
			description: "All 4 Jacks + 3 Aces - ideal for Grand",
		},
		{
			name:        "Medium Hearts Hand",
			handStr:     "J.♣-J.♠-A.♥-10.♥-K.♥-A.♦-K.♠-Q.♠-9.♣-8.♦",
			bestMode:    game.ModeSuit,
			bestSuit:    game.Hearts,
			description: "2 Jacks + 3 Hearts with A+10 - 5 trumps for Hearts, should win Hearts but struggle with Grand/others",
		},
	}

	numGames := 100

	for _, testHand := range testHands {
		hand, err := game.ParseCards(testHand.handStr)
		if err != nil {
			fmt.Printf("Error parsing hand: %v\n", err)
			continue
		}

		fmt.Printf("\n%s: %s\n", testHand.name, testHand.handStr)
		fmt.Printf("%s\n", testHand.description)
		fmt.Println()

		// Test Grand
		wins, totalPoints, gamesPlayed := playGamesWithMode(testAgent, hand, game.ModeGrand, game.Clubs, numGames)
		winRate := float64(wins) / float64(gamesPlayed) * 100
		avgPoints := float64(totalPoints) / float64(gamesPlayed)

		marker := " "
		if testHand.bestMode == game.ModeGrand {
			marker = "✓"
		}
		fmt.Printf("  %s Grand   : %3d wins (%.0f%%), avg %+.1f points (%d games)\n", marker, wins, winRate, avgPoints, gamesPlayed)

		// Test all suits
		suits := []game.Suit{game.Clubs, game.Spades, game.Hearts, game.Diamonds}
		for _, suit := range suits {
			wins, totalPoints, gamesPlayed := playGamesWithMode(testAgent, hand, game.ModeSuit, suit, numGames)
			winRate := float64(wins) / float64(gamesPlayed) * 100
			avgPoints := float64(totalPoints) / float64(gamesPlayed)

			marker := " "
			if testHand.bestMode == game.ModeSuit && suit == testHand.bestSuit {
				marker = "✓"
			}

			fmt.Printf("  %s %-8s: %3d wins (%.0f%%), avg %+.1f points\n",
				marker, suit.String(), wins, winRate, avgPoints)
		}
	}
}

func playGamesWithMode(testAgent *agent.SkatAgent, declarerHand game.Cards, mode game.GameMode, trumpSuit game.Suit, numGames int) (wins, totalPoints, gamesPlayed int) {
	baselineAgent := agent.NewHeuristicAgent("Baseline")

	config := agent.NewFiftyFiftySplitConfig(testAgent, baselineAgent)
	g := game.NewGame()
	g = agent.WithAgentPlayers(g, config)

	for i := 0; i < numGames; i++ {
		agent.PlayGameWithMode(g, config, declarerHand, mode, trumpSuit)
		g.NextGame()
	}

	// Get metrics from the local agent
	metrics := testAgent.GetMetrics()
	return int(metrics.Wins), int(metrics.Points), int(metrics.Games)
}
