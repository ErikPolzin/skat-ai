package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"runtime"
	"skat/agent"
	"skat/agent/training"
	"skat/game"
	"strings"
)

func main() {
	agentType := flag.String("agent-type", "", "Agent type: heuristic, mcts, minimax, neural, or random (if not set, uses component flags)")
	biddingType := flag.String("bidding-type", "heuristic", "Bidding & game choice strategy: heuristic or weighted")
	cardPlayType := flag.String("card-play-type", "heuristic", "Card play strategy: heuristic, mcts, minimax, or neural")
	biddingMode := flag.String("bidding-mode", "5050", "Bidding mode: 5050 (all test agents bid, alternate declarer) or 2v1 (test vs 2 baseline)")
	games := flag.Int("games", 500, "Number of evaluation games")
	cardplayWeights := flag.String("cardplay-weights", ".data/models/cardplay.weights", "Path to card play neural network weights")
	biddingWeights := flag.String("bidding-weights", "", "Path to weighted bidding weights JSON file (optional)")
	threshold := flag.Float64("threshold", 0.0, "Bidding threshold (0=use strategy default, weighted default=0.70, heuristic default=0.45)")
	minimaxDepth := flag.Int("minimax-depth", 10, "Minimax search depth for perfect-info minimax")
	mctsSimulations := flag.Int("mcts-simulations", 500, "MCTS simulation count")
	ignoreZwangsspiel := flag.Bool("ignore-zwangsspiel", false, "Exclude Zwangsspiel (passed) games from evaluation")
	flag.Parse()

	runEvaluation(*agentType, *biddingType, *cardPlayType, *biddingMode, *games, *cardplayWeights, *biddingWeights, *threshold, *minimaxDepth, *mctsSimulations, *ignoreZwangsspiel)
}

func runEvaluation(agentType, biddingType, cardPlayType, biddingMode string, totalRounds int, cardplayWeights, biddingWeightsFile string, threshold float64, minimaxDepth, mctsSimulations int, ignoreZwangsspiel bool) {
	var testAgent *agent.SkatAgent
	var err error

	// Create agent based on type or component configuration
	if agentType != "" {
		// Use predefined agent type
		testAgent, err = createAgentByType(agentType, cardplayWeights, threshold, minimaxDepth, mctsSimulations)
		if err != nil {
			fmt.Printf("Error creating agent: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Build hybrid agent from component flags
		// Game choice type always matches bidding type
		config := agent.HybridAgentConfig{
			BiddingType:        biddingType,
			BiddingThreshold:   threshold,
			BiddingWeightsPath: biddingWeightsFile,
			GameChoiceType:     biddingType, // Always same as bidding type
			CardPlayType:       cardPlayType,
			NeuralWeightsPath:  cardplayWeights,
			MinimaxDepth:       minimaxDepth,
			MCTSSimulations:    mctsSimulations,
		}
		testAgent, err = agent.NewHybridAgent("Test", config)
		if err != nil {
			fmt.Printf("Error creating hybrid agent: %v\n", err)
			os.Exit(1)
		}
	}

	testDescription := buildAgentDescription(testAgent)

	fmt.Printf("Test agent: %s\n", testDescription)

	// Baseline agent: All heuristic
	baselineAgent := agent.NewHeuristicAgent("Baseline")
	fmt.Println("Baseline agent: All heuristic")

	// Choose evaluation config based on bidding mode
	var evalConfig agent.AgentConfig
	switch biddingMode {
	case "5050":
		evalConfig = agent.NewFiftyFiftySplitConfig(testAgent, baselineAgent)
		fmt.Println("Bidding mode: All 3 test agents bid, alternate declarer/defender")
	case "2v1":
		evalConfig = agent.NewTwoVsOneConfig(testAgent, baselineAgent)
		fmt.Println("Bidding mode: 1 test agent vs 2 baseline agents")
	default:
		fmt.Printf("Unknown bidding mode: %s (use 5050 or 2v1)\n", biddingMode)
		os.Exit(1)
	}

	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Printf("Running %d games on %d CPU cores...\n", totalRounds, runtime.GOMAXPROCS(0))
	if ignoreZwangsspiel {
		fmt.Println("Ignoring Zwangsspiel (passed) games")
	}
	fmt.Println(strings.Repeat("=", 50) + "\n")

	training.EvaluateAgents(evalConfig, totalRounds)

	// Get agent metrics for bidding distribution
	testMetrics := testAgent.GetMetrics()
	baselineMetrics := baselineAgent.GetMetrics()

	// Track passed games for display
	passedGames := testMetrics.PassedGames

	var testGames, testWins, testPoints, testOverbid int64
	var baselineGames, baselineWins, baselinePoints int64

	if ignoreZwangsspiel {
		// Exclude Zwangsspiel games and wins from metrics
		// passedGames is tracked globally, but passedGamesWon is per-agent
		testGames = testMetrics.Games - testMetrics.PassedGamesWon
		testWins = testMetrics.Wins - testMetrics.PassedGamesWon
		testPoints = testMetrics.Points
		testOverbid = testMetrics.Overbid
		baselineGames = baselineMetrics.Games - baselineMetrics.PassedGamesWon
		baselineWins = baselineMetrics.Wins - baselineMetrics.PassedGamesWon
		baselinePoints = baselineMetrics.Points
	} else {
		testGames = testMetrics.Games
		testWins = testMetrics.Wins
		testPoints = testMetrics.Points
		testOverbid = testMetrics.Overbid
		baselineGames = baselineMetrics.Games
		baselineWins = baselineMetrics.Wins
		baselinePoints = baselineMetrics.Points
	}

	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("FINAL RESULTS")
	fmt.Println(strings.Repeat("=", 50))

	// Use raw Games (not adjusted for Zwangsspiel) since DefenderGames isn't adjusted either
	declarerGamesTotal := testMetrics.Games
	defenderGamesTotal := testMetrics.DefenderGames
	totalGamesPlayed := defenderGamesTotal + declarerGamesTotal

	defenderPct := float64(defenderGamesTotal) / float64(totalGamesPlayed) * 100
	declarerPct := float64(declarerGamesTotal) / float64(totalGamesPlayed) * 100

	fmt.Printf("\nTest (%s):\n", testDescription)
	fmt.Printf("    Declarer   %4d/%4d (%3.0f%%) %s Defender       %4d/%4d (%3.0f%%)\n",
		declarerGamesTotal, totalGamesPlayed, declarerPct, makeWinRateBar(declarerPct),
		defenderGamesTotal, totalGamesPlayed, defenderPct)

	if testGames > 0 {
		declarerWinRate := float64(testWins) / float64(testGames) * 100
		baselineDefWinRate := 100.0 - declarerWinRate
		fmt.Printf("    Test Decl. %4d/%4d (%3.0f%%) %s Baseline Def.  %4d/%4d (%3.0f%%)\n",
			testWins, testGames, declarerWinRate, makeWinRateBar(declarerWinRate),
			testGames-testWins, testGames, baselineDefWinRate)

		// Defender stats
		if testMetrics.DefenderGames > 0 {
			defenderWinRate := float64(testMetrics.DefenderWins) / float64(testMetrics.DefenderGames) * 100
			baselineDeclWinRate := 100.0 - defenderWinRate
			fmt.Printf("    Test Def.  %4d/%4d (%3.0f%%) %s Baseline Decl. %4d/%4d (%3.0f%%)\n",
				testMetrics.DefenderWins, testMetrics.DefenderGames, defenderWinRate, makeWinRateBar(defenderWinRate),
				testMetrics.DefenderGames-testMetrics.DefenderWins, testMetrics.DefenderGames, baselineDeclWinRate)
		}

		// Points comparison using sigmoid: 0 diff = 50%, larger diff = closer to 0 or 100%
		// Sigmoid function: 1 / (1 + exp(-x/temp)) where x is point difference
		// Temperature controls steepness - higher temp = more gradual change
		pointsDiff := float64(testPoints - baselinePoints)
		temperature := float64(totalRounds * 2.0) // Adjust this to control sensitivity
		testPointsPct := 100.0 / (1.0 + math.Exp(-pointsDiff/temperature))
		baselinePointsPct := 100.0 - testPointsPct

		fmt.Printf("    Test Pts.  %9d (%3.0f%%) %s Baseline Pts.  %9d (%3.0f%%)\n",
			testPoints, testPointsPct, makeWinRateBar(testPointsPct),
			baselinePoints, baselinePointsPct)

		fmt.Printf("  Avg points as declarer: %.1f\n", float64(testPoints)/float64(testGames))
		fmt.Printf("  Overbid rate: %.1f%% (%d/%d)\n",
			float64(testOverbid)/float64(testGames)*100, testOverbid, testGames)

		// Calculate passed games percentage
		passedPct := float64(passedGames) / float64(totalRounds) * 100
		fmt.Printf("  Passed games: %.1f%% (%d/%d) - all players passed (Zwangsspiel)\n",
			passedPct, passedGames, totalRounds)

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

	if testGames > 0 && baselineGames > 0 {
		improvement := (float64(testWins)/float64(testGames) - float64(baselineWins)/float64(baselineGames)) * 100
		pointDiff := float64(testPoints)/float64(testGames) - float64(baselinePoints)/float64(baselineGames)
		fmt.Printf("\nImprovement: %+.1f percentage points\n", improvement)
		fmt.Printf("Point difference: %+.1f points per game\n", pointDiff)
	}

	// Show calibration statistics if available
	if len(testMetrics.PredictedProbability) > 0 {
		fmt.Println("\n" + strings.Repeat("=", 50))
		fmt.Println("CALIBRATION STATISTICS")
		fmt.Println(strings.Repeat("=", 50))
		displayCalibration(testMetrics.PredictedProbability, testMetrics.ActualOutcomes)
	}

	// Show example bidding decisions
	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("EXAMPLE BIDDING DECISIONS")
	fmt.Println(strings.Repeat("=", 50))
	testExampleBiddingHands(testAgent)

	// Show example game choice decisions
	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("EXAMPLE GAME CHOICE DECISIONS")
	fmt.Println(strings.Repeat("=", 50))
	testExampleGameChoiceHands(testAgent)

	// Run game-play test with known winning games (skip for minimax - too slow)
	if agentType != "minimax" {
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

// createAgentByType creates an agent using predefined agent type
func createAgentByType(agentType, cardplayWeights string, threshold float64, minimaxDepth, mctsSimulations int) (*agent.SkatAgent, error) {
	switch agentType {
	case "heuristic":
		config := agent.HybridAgentConfig{
			BiddingType:      "heuristic",
			BiddingThreshold: threshold,
			GameChoiceType:   "heuristic",
			CardPlayType:     "heuristic",
		}
		return agent.NewHybridAgent("Test", config)
	case "random":
		return agent.NewRandomAgent("Test"), nil
	case "mcts":
		config := agent.HybridAgentConfig{
			BiddingType:      "heuristic",
			BiddingThreshold: threshold,
			GameChoiceType:   "heuristic",
			CardPlayType:     "mcts",
			MCTSSimulations:  mctsSimulations,
		}
		return agent.NewHybridAgent("Test", config)
	case "minimax":
		config := agent.HybridAgentConfig{
			BiddingType:      "heuristic",
			BiddingThreshold: threshold,
			GameChoiceType:   "heuristic",
			CardPlayType:     "minimax",
			MinimaxDepth:     minimaxDepth,
		}
		return agent.NewHybridAgent("Test", config)
	case "neural":
		config := agent.HybridAgentConfig{
			BiddingType:       "heuristic",
			BiddingThreshold:  threshold,
			GameChoiceType:    "heuristic",
			CardPlayType:      "neural",
			NeuralWeightsPath: cardplayWeights,
		}
		return agent.NewHybridAgent("Test", config)
	default:
		return nil, fmt.Errorf("unknown agent type: %s", agentType)
	}
}

// buildAgentDescription creates a human-readable description of the agent
func buildAgentDescription(a *agent.SkatAgent) string {
	bidding := a.GetBiddingStrategy().GetName()
	gameChoice := a.GetGameChoiceStrategy().GetName()
	cardPlay := a.GetCardPlayStrategy().GetName()

	return fmt.Sprintf("%s bidding + %s game choice + %s card play", bidding, gameChoice, cardPlay)
}

func displayCalibration(predicted []float64, actual []bool) {
	if len(predicted) == 0 || len(predicted) != len(actual) {
		fmt.Println("No calibration data available")
		return
	}

	// Bin predictions into buckets [0.5-0.6), [0.6-0.7), [0.7-0.8), [0.8-0.9), [0.9-1.0]
	type bucket struct {
		minProb    float64
		maxProb    float64
		count      int
		wins       int
		sumProb    float64
		avgProb    float64
		actualRate float64
	}

	buckets := []bucket{
		{0.5, 0.6, 0, 0, 0, 0, 0},
		{0.6, 0.7, 0, 0, 0, 0, 0},
		{0.7, 0.8, 0, 0, 0, 0, 0},
		{0.8, 0.9, 0, 0, 0, 0, 0},
		{0.9, 1.0, 0, 0, 0, 0, 0},
	}

	for i, prob := range predicted {
		for j := range buckets {
			if prob >= buckets[j].minProb && prob < buckets[j].maxProb || (prob == 1.0 && j == len(buckets)-1) {
				buckets[j].count++
				buckets[j].sumProb += prob
				if actual[i] {
					buckets[j].wins++
				}
				break
			}
		}
	}

	// Calculate averages
	totalCount := 0
	totalWins := 0
	for i := range buckets {
		if buckets[i].count > 0 {
			buckets[i].avgProb = buckets[i].sumProb / float64(buckets[i].count)
			buckets[i].actualRate = float64(buckets[i].wins) / float64(buckets[i].count)
			totalCount += buckets[i].count
			totalWins += buckets[i].wins
		}
	}

	fmt.Printf("Total games with predictions: %d\n", totalCount)
	fmt.Printf("Overall win rate: %.1f%%\n\n", float64(totalWins)/float64(totalCount)*100)
	fmt.Printf("%-15s %-10s %-10s %-10s %s\n", "Predicted", "Games", "Actual", "Avg Pred", "Calibration")
	fmt.Println(strings.Repeat("-", 70))

	for _, b := range buckets {
		if b.count == 0 {
			continue
		}
		rangeStr := fmt.Sprintf("%.0f%%-%.0f%%", b.minProb*100, b.maxProb*100)
		diff := b.actualRate - b.avgProb
		calibStr := ""
		if diff > 0.05 {
			calibStr = fmt.Sprintf("▲ %.1f%%", diff*100)
		} else if diff < -0.05 {
			calibStr = fmt.Sprintf("▼ %.1f%%", -diff*100)
		} else {
			calibStr = "✓"
		}

		fmt.Printf("%-15s %-10d %-10.1f%% %-10.1f%% %s\n",
			rangeStr, b.count, b.actualRate*100, b.avgProb*100, calibStr)
	}
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
		{
			name:        "Perfect Null Hand",
			handStr:     "7.♣-8.♣-9.♣-7.♠-8.♠-9.♠-7.♥-8.♥-7.♦-8.♦",
			bestMode:    game.ModeNull,
			bestSuit:    game.NoSuit,
			description: "All low cards (7s, 8s, 9s) - perfect for Null, impossible to win tricks",
		},
		{
			name:        "Good Null Hand",
			handStr:     "7.♣-8.♣-9.♣-7.♠-9.♠-10.♠-7.♥-8.♥-9.♥-7.♦",
			bestMode:    game.ModeNull,
			bestSuit:    game.NoSuit,
			description: "Mostly low cards with one 10 - should win Null, but might struggle with suit/grand",
		},
		{
			name:        "Marginal Null Hand",
			handStr:     "7.♣-9.♣-10.♣-7.♠-9.♠-J.♠-7.♥-8.♥-9.♥-8.♦",
			bestMode:    game.ModeNull,
			bestSuit:    game.NoSuit,
			description: "Low cards with J and 10 - risky Null, defenders should be able to force wins sometimes",
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

		// Test Null
		nullWins, nullTotalPoints, nullGamesPlayed := playGamesWithMode(testAgent, hand, game.ModeNull, game.NoSuit, numGames)
		nullWinRate := float64(nullWins) / float64(nullGamesPlayed) * 100
		nullAvgPoints := float64(nullTotalPoints) / float64(nullGamesPlayed)

		nullMarker := " "
		if testHand.bestMode == game.ModeNull {
			nullMarker = "✓"
		}

		fmt.Printf("  %s Null    : %3d wins (%.0f%%), avg %+.1f points\n",
			nullMarker, nullWins, nullWinRate, nullAvgPoints)
	}
}

func makeWinRateBar(winRate float64) string {
	const barWidth = 40
	filled := int(winRate / 100.0 * barWidth)
	if filled < 0 {
		filled = 0
	}
	if filled > barWidth {
		filled = barWidth
	}
	empty := barWidth - filled
	return "[" + strings.Repeat("█", filled) + strings.Repeat("░", empty) + "]"
}

func playGamesWithMode(testAgent *agent.SkatAgent, declarerHand game.Cards, mode game.GameMode, trumpSuit game.Suit, numGames int) (wins, totalPoints, gamesPlayed int) {
	// Clone the test agent to avoid polluting the main eval metrics, but use its strategy
	testAgentLocal := testAgent.Clone()
	testAgentLocal.EnableMetrics() // Enable metrics tracking on the clone
	defender1 := agent.NewHeuristicAgent("Defender1")
	defender2 := agent.NewHeuristicAgent("Defender2")

	for i := 0; i < numGames; i++ {
		// Create a fresh game each iteration to avoid rotation issues
		g := game.NewGame()
		// Use three-way config to set up all players
		config := agent.NewThreeWayConfig(defender1, defender2, testAgentLocal)
		g = agent.WithAgentPlayers(g, config)
		agent.PlayGameWithMode(g, config, declarerHand, mode, trumpSuit)
	}

	// Get collected stats from the agent
	metrics := testAgentLocal.GetMetrics()
	return int(metrics.Wins), int(metrics.Points), int(metrics.Games)
}
