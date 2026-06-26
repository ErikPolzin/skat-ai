package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"

	"skat/agent"
	"skat/agent/strategies"
	"skat/agent/strategies/encoding"
	"skat/game"
)

// CardPlayExample represents a single (state, action) pair for supervised learning
type CardPlayExample struct {
	State          [encoding.CardPlayFeatureSize]float32 // DQN state encoding
	ValidMask      [32]float32                           // Valid moves at this state
	Action         int                                   // Card index that expert chose
	Role           int                                   // 0=defender, 1=declarer, 2=Ramsch
	GameMode       game.GameMode
	WinProbability float64
	Policy         [32]float32 // Soft target policy from expert scores
}

const (
	roleDefender = iota
	roleDeclarer
	roleRamsch
)

var bucketOrder = []string{"suit_declarer", "suit_defender", "grand_declarer", "grand_defender", "null_declarer", "null_defender", "ramsch"}

const (
	needSuitDeclarer uint32 = 1 << iota
	needSuitDefender
	needGrandDeclarer
	needGrandDefender
	needNullDeclarer
	needNullDefender
	needRamsch
	needAllBuckets = (1 << iota) - 1
)

const needNormalBuckets = needAllBuckets &^ needRamsch

func exampleBucket(ex CardPlayExample) string {
	if ex.Role == roleRamsch {
		return "ramsch"
	}
	role := "defender"
	if ex.Role == roleDeclarer {
		role = "declarer"
	}
	return string(ex.GameMode) + "_" + role
}

func newSearchTeacherAgent(name string, depth int, biddingThreshold float64) *agent.SkatAgent {
	config := strategies.DefaultContractEvaluatorConfig()
	config.MinWinProbability = biddingThreshold

	return agent.NewAgentWithStrategies(
		name,
		strategies.NewHeuristicBiddingStrategyWithConfig(config),
		strategies.NewHeuristicGameChoiceStrategyWithConfig(config),
		strategies.NewPerfectInfoMinimaxStrategyWithDepth(depth),
	)
}

func main() {
	numExamples := flag.Int("examples", 100000, "Number of examples to collect in each game-type/role bucket")
	outputFile := flag.String("output", ".data/cardplay_dataset.csv", "Output file for dataset")
	searchDepth := flag.Int("depth", 7, "Minimax search depth for expert labels; uses move ordering, transposition table, and LMR 3/1")
	biddingThreshold := flag.Float64("bidding-threshold", 0.55, "Heuristic bidding threshold used for natural contract generation")
	minWinProbability := flag.Float64("min-win-probability", 0.10, "Minimum pre-game win probability to collect")
	maxWinProbability := flag.Float64("max-win-probability", 0.65, "Maximum pre-game win probability to collect")
	acceptableGap := flag.Float64("acceptable-gap", 0.05, "Maximum minimax probability gap from the best move to treat as an equally good target")
	workers := flag.Int("workers", runtime.NumCPU(), "Number of parallel workers")
	flag.Parse()

	if *minWinProbability < 0 || *maxWinProbability > 1 || *minWinProbability > *maxWinProbability {
		fmt.Fprintf(os.Stderr, "Invalid win-probability range %.2f-%.2f (expected 0 <= min <= max <= 1)\n", *minWinProbability, *maxWinProbability)
		os.Exit(1)
	}
	if *acceptableGap < 0 {
		fmt.Fprintf(os.Stderr, "Invalid acceptable gap %.2f (expected >= 0)\n", *acceptableGap)
		os.Exit(1)
	}

	fmt.Printf("Generating %d examples in each of 7 game-type/role buckets...\n", *numExamples)
	fmt.Printf("  Declarer strategy: Minimax (depth %d)\n", *searchDepth)
	fmt.Printf("  Defender strategy: Minimax (depth %d)\n", *searchDepth)
	fmt.Printf("  Ramsch strategy: Minimax (depth %d)\n", *searchDepth)
	fmt.Printf("  Contracts: natural bidding and game choice (threshold %.2f)\n", *biddingThreshold)
	fmt.Printf("  Contract filtering: Minimax wins from %.2f-%.2f pre-game win probability; excluding overbids\n", *minWinProbability, *maxWinProbability)
	fmt.Printf("  Ramsch filtering: winners from any starting hand\n")
	fmt.Printf("  Multi-card targets: moves within %.2f minimax score/probability of best\n", *acceptableGap)
	fmt.Printf("Using %d parallel workers\n", *workers)

	// Channel for collecting results
	examplesChan := make(chan []CardPlayExample, *workers)
	stopChan := make(chan bool) // Signal workers to stop
	var wg sync.WaitGroup
	var neededBuckets atomic.Uint32
	neededBuckets.Store(needAllBuckets)

	// Worker function - collect until we have enough examples
	worker := func() {
		defer wg.Done()

		// Create search agent for expert card-play labels.
		searchAgent := newSearchTeacherAgent("SearchExpert", *searchDepth, *biddingThreshold)
		labelScorer := strategies.NewPerfectInfoMinimaxStrategyWithDepth(*searchDepth)

		config := strategies.DefaultContractEvaluatorConfig()
		config.MinWinProbability = *biddingThreshold

		// Heuristic opponents face the Minimax teacher in each role-specific replay.
		heuristicAgent := agent.NewAgentWithStrategies(
			"HeuristicDefender",
			strategies.NewHeuristicBiddingStrategyWithConfig(config),
			strategies.NewHeuristicGameChoiceStrategyWithConfig(config),
			agent.NewHeuristicCardPlayStrategy(),
		)

		// Keep generating games until we signal stop
		for {
			select {
			case <-stopChan:
				return
			default:
				needed := neededBuckets.Load()
				var examples []CardPlayExample
				if needed&needRamsch != 0 && (needed&needNormalBuckets == 0 || rand.Intn(4) == 0) {
					examples = playRamschAndCollectExamples(searchAgent, labelScorer, *acceptableGap)
				} else {
					examples = playGameAndCollectExamples(searchAgent, heuristicAgent, labelScorer, *acceptableGap, *minWinProbability, *maxWinProbability, needed)
				}
				// Every attempted game sends one batch, including filtered empty batches,
				// so the collector owns exact progress accounting.
				select {
				case examplesChan <- examples:
				case <-stopChan:
					return
				}
			}
		}
	}

	// Start workers
	for i := 0; i < *workers; i++ {
		wg.Add(1)
		go worker()
	}

	// Collect exactly n examples for every normal game/role pair plus Ramsch.
	buckets := make(map[string][]CardPlayExample, len(bucketOrder))
	gamesPlayed := 0

	for examples := range examplesChan {
		gamesPlayed++
		for _, ex := range examples {
			key := exampleBucket(ex)
			if len(buckets[key]) < *numExamples {
				buckets[key] = append(buckets[key], ex)
			}
		}
		neededBuckets.Store(neededBucketMask(buckets, *numExamples))
		if gamesPlayed%100 == 0 {
			printBucketProgress(gamesPlayed, buckets, *numExamples)
		}
		if bucketsComplete(buckets, *numExamples) {
			break
		}
	}

	// Signal all workers to stop
	close(stopChan)

	// Wait for workers to finish
	wg.Wait()

	// Close channels
	close(examplesChan)

	// Create balanced dataset (should already be at exact count)
	dataset := make([]CardPlayExample, 0, len(bucketOrder)*(*numExamples))
	for _, key := range bucketOrder {
		dataset = append(dataset, buckets[key]...)
		fmt.Printf("  %-16s %d\n", key, len(buckets[key]))
	}
	fmt.Printf("\nCollected dataset: %d total examples\n", len(dataset))
	printTargetStats(dataset)

	// Save dataset to CSV file
	fmt.Printf("\nSaving %d examples to %s...\n", len(dataset), *outputFile)

	// Ensure directory exists
	os.MkdirAll(".data", 0755)

	file, err := os.Create(*outputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create output file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := make([]string, 0, encoding.CardPlayFeatureSize+32+2+32)
	for i := 0; i < encoding.CardPlayFeatureSize; i++ {
		header = append(header, fmt.Sprintf("s%d", i))
	}
	for i := 0; i < 32; i++ {
		header = append(header, fmt.Sprintf("m%d", i))
	}
	header = append(header, "action", "role")
	for i := 0; i < 32; i++ {
		header = append(header, fmt.Sprintf("p%d", i))
	}
	header = append(header, "game_mode", "win_probability")
	if err := writer.Write(header); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write header: %v\n", err)
		os.Exit(1)
	}

	// Write examples
	for _, ex := range dataset {
		record := make([]string, 0, encoding.CardPlayFeatureSize+32+2+32)

		// State features
		for _, val := range ex.State {
			record = append(record, strconv.FormatFloat(float64(val), 'f', 6, 32))
		}

		// Valid mask (32)
		for _, val := range ex.ValidMask {
			record = append(record, strconv.FormatFloat(float64(val), 'f', 0, 32))
		}

		// Action and role
		record = append(record, strconv.Itoa(ex.Action))
		record = append(record, strconv.Itoa(ex.Role))
		for _, val := range ex.Policy {
			record = append(record, strconv.FormatFloat(float64(val), 'f', 6, 32))
		}
		record = append(record, string(ex.GameMode), strconv.FormatFloat(ex.WinProbability, 'f', 6, 64))

		if err := writer.Write(record); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write record: %v\n", err)
			os.Exit(1)
		}
	}

	// Print statistics
	fmt.Printf("\nDataset Statistics:\n")
	fmt.Printf("  Total examples: %d\n", len(dataset))
	fmt.Printf("\n✓ Dataset generation complete!\n")
}

func bucketsComplete(buckets map[string][]CardPlayExample, target int) bool {
	for _, key := range bucketOrder {
		if len(buckets[key]) < target {
			return false
		}
	}
	return true
}

func neededBucketMask(buckets map[string][]CardPlayExample, target int) uint32 {
	var mask uint32
	for i, key := range bucketOrder {
		if len(buckets[key]) < target {
			mask |= 1 << i
		}
	}
	return mask
}

func contractBucketBits(mode game.GameMode) (declarer, defender uint32) {
	switch mode {
	case game.ModeSuit:
		return needSuitDeclarer, needSuitDefender
	case game.ModeGrand:
		return needGrandDeclarer, needGrandDefender
	case game.ModeNull:
		return needNullDeclarer, needNullDefender
	default:
		return 0, 0
	}
}

func printBucketProgress(games int, buckets map[string][]CardPlayExample, target int) {
	fmt.Printf("  Played %d | Suit Dec %d/%d Def %d/%d | Grand Dec %d/%d Def %d/%d | Null Dec %d/%d Def %d/%d | Ramsch %d/%d\n",
		games,
		len(buckets["suit_declarer"]), target, len(buckets["suit_defender"]), target,
		len(buckets["grand_declarer"]), target, len(buckets["grand_defender"]), target,
		len(buckets["null_declarer"]), target, len(buckets["null_defender"]), target,
		len(buckets["ramsch"]), target)
}

// setupGame creates a naturally bid game ready for card play.
func setupGame(heuristicAgent *agent.SkatAgent) (*game.GameState, bool) {
	config := agent.NewThreeWayConfig(
		heuristicAgent,
		heuristicAgent.CachedClone(),
		heuristicAgent.CachedClone().CachedClone())
	// Create game
	g := game.NewGame()
	g = agent.WithAgentPlayers(g, config)
	g = g.WithCardsDealt()
	g = agent.WithAgentBidding(g, config)
	if g.Declarer == nil {
		return g, false
	}
	g = agent.WithAgentSkatDecision(g)
	return agent.WithAgentGameChoice(g)
}

// collectDeclarerExamples plays a game with search-teacher declarer vs heuristic defenders.
func collectDeclarerExamples(g *game.GameState, searchAgent, heuristicAgent *agent.SkatAgent, labelScorer *strategies.PerfectInfoMinimaxStrategy, acceptableGap float64) []CardPlayExample {
	var examples []CardPlayExample

	if g.Declarer == nil {
		return examples
	}

	declarer := *g.Declarer
	// Search-teacher declarer vs heuristic defenders
	agent.SetAgentForPlayer(g.GetPlayerByPosition(declarer), searchAgent)
	agent.SetAgentForPlayer(g.GetPlayerByPosition((declarer+1)%3), heuristicAgent)
	agent.SetAgentForPlayer(g.GetPlayerByPosition((declarer+2)%3), heuristicAgent)

	// Card play phase
	for g.Phase == game.PhasePlaying {
		validMoves := g.GetValidMoves()
		currentPlayer := g.CurrentPlayer
		currentAgent := agent.GetAgentForPlayer(g.GetCurrentPlayer())

		if currentPlayer == declarer {
			// Encode state
			enc := encoding.EncodeNeuralCardPlay(g, currentPlayer, validMoves)
			state := enc.ToSlice()
			validMask := enc.GetValidMask()

			// Score every root move independently so near-equivalent expert moves
			// can share the target probability.
			scores := labelScorer.ScoreMoves(g, validMoves)
			policy, best := acceptableMovePolicy(validMoves, scores, true, acceptableGap)
			card := validMoves[best]
			action := encoding.CardToIndex(card)

			// Store declarer example
			examples = append(examples, CardPlayExample{
				State:     state,
				ValidMask: validMask,
				Action:    action,
				Role:      roleDeclarer,
				GameMode:  g.Mode,
				Policy:    policy,
			})

			// Play card
			if _, err := g.PlayCard(card); err != nil {
				panic(fmt.Sprintf("PlayCard error: %v", err))
			}
		} else {
			// Opponent defender plays
			card := currentAgent.SelectMove(g, validMoves)
			if _, err := g.PlayCard(card); err != nil {
				panic(fmt.Sprintf("PlayCard error: %v", err))
			}
		}

		// Resolve trick if complete
		if len(g.Trick) == 3 {
			resolveTrickAndNotify(g)
		}
	}

	// Only imitate successful play. Keeping the examples buffered until the
	// result is known avoids teaching attractive-looking lines that lost.
	if !g.Result().DeclarerWon {
		return nil
	}
	return examples
}

// collectDefenderExamples plays a game with a heuristic declarer vs Minimax defenders.
func collectDefenderExamples(g *game.GameState, defenderSearchAgent, heuristicAgent *agent.SkatAgent, labelScorer *strategies.PerfectInfoMinimaxStrategy, acceptableGap float64) []CardPlayExample {
	var examples []CardPlayExample

	if g.Declarer == nil {
		return examples
	}

	declarer := *g.Declarer
	// Heuristic declarer vs Minimax defenders.
	agent.SetAgentForPlayer(g.GetPlayerByPosition(declarer), heuristicAgent)
	agent.SetAgentForPlayer(g.GetPlayerByPosition((declarer+1)%3), defenderSearchAgent)
	agent.SetAgentForPlayer(g.GetPlayerByPosition((declarer+2)%3), defenderSearchAgent.CachedClone())

	// Card play phase
	for g.Phase == game.PhasePlaying {
		validMoves := g.GetValidMoves()
		currentPlayer := g.CurrentPlayer

		if currentPlayer != declarer {
			// Encode state
			enc := encoding.EncodeNeuralCardPlay(g, currentPlayer, validMoves)
			state := enc.ToSlice()
			validMask := enc.GetValidMask()

			// Get expert defender action
			scores := labelScorer.ScoreMoves(g, validMoves)
			policy, best := acceptableMovePolicy(validMoves, scores, false, acceptableGap)
			card := validMoves[best]
			action := encoding.CardToIndex(card)

			// Store defender example
			examples = append(examples, CardPlayExample{
				State:     state,
				ValidMask: validMask,
				Action:    action,
				Role:      roleDefender,
				GameMode:  g.Mode,
				Policy:    policy,
			})

			// Play card
			if _, err := g.PlayCard(card); err != nil {
				panic(fmt.Sprintf("PlayCard error: %v", err))
			}
		} else {
			// Opponent declarer plays
			currentAgent := agent.GetAgentForPlayer(g.GetCurrentPlayer())
			card := currentAgent.SelectMove(g, validMoves)
			if _, err := g.PlayCard(card); err != nil {
				panic(fmt.Sprintf("PlayCard error: %v", err))
			}
		}

		// Resolve trick if complete
		if len(g.Trick) == 3 {
			resolveTrickAndNotify(g)
		}
	}

	// Defenders win as a team when the declarer loses.
	if g.Result().DeclarerWon {
		return nil
	}
	return examples
}

func resolveTrickAndNotify(g *game.GameState) {
	trick := append([]game.Card{}, g.Trick...)
	if _, err := g.ResolveTrick(); err != nil {
		panic(fmt.Sprintf("ResolveTrick error: %v", err))
	}
	for i := range g.Players {
		if g.Players[i].IsAgent {
			if agent := agent.GetAgentForPlayer(g.Players[i]); agent != nil {
				agent.OnTrickComplete(trick)
			}
		}
	}
}

func acceptableMovePolicy(moves []game.Card, scores []float64, maximize bool, gap float64) ([32]float32, int) {
	var policy [32]float32
	if len(moves) == 0 || len(scores) != len(moves) {
		return policy, -1
	}
	if gap < 0 {
		gap = 0
	}

	best := 0
	for i := 1; i < len(scores); i++ {
		if (maximize && scores[i] > scores[best]) || (!maximize && scores[i] < scores[best]) {
			best = i
		}
	}

	acceptable := make([]int, 0, len(moves))
	for i, score := range scores {
		difference := scores[best] - score
		if !maximize {
			difference = score - scores[best]
		}
		if difference <= gap {
			acceptable = append(acceptable, i)
		}
	}
	probability := float32(1.0 / float64(len(acceptable)))
	for _, i := range acceptable {
		policy[encoding.CardToIndex(moves[i])] = probability
	}
	return policy, best
}

func printTargetStats(dataset []CardPlayExample) {
	if len(dataset) == 0 {
		return
	}
	totalAcceptable, multiTarget := 0, 0
	for _, example := range dataset {
		count := 0
		for _, probability := range example.Policy {
			if probability > 0 {
				count++
			}
		}
		totalAcceptable += count
		if count > 1 {
			multiTarget++
		}
	}
	fmt.Printf("Acceptable targets: %.2f cards/example; %.1f%% have multiple cards\n",
		float64(totalAcceptable)/float64(len(dataset)),
		float64(multiTarget)*100/float64(len(dataset)))
}

// playGameAndCollectExamples plays games twice: once for declarer examples, once for defender examples.
func playGameAndCollectExamples(searchAgent, heuristicAgent *agent.SkatAgent, labelScorer *strategies.PerfectInfoMinimaxStrategy, acceptableGap, minWinProbability, maxWinProbability float64, needed uint32) []CardPlayExample {
	var examples []CardPlayExample

	g, overbid := setupGame(heuristicAgent)
	if g.Declarer == nil || overbid {
		return examples
	}
	declarerBucket, defenderBucket := contractBucketBits(g.Mode)
	if needed&(declarerBucket|defenderBucket) == 0 {
		return examples
	}

	declarer := *g.Declarer
	handStrength := strategies.EstimateContractWinProbability(
		g.GetPlayerByPosition(declarer).Hand,
		g.Mode,
		g.TrumpSuit,
	)
	if needed&declarerBucket != 0 && probabilityInRange(handStrength, minWinProbability, maxWinProbability) {
		// Collect an initially unfavored declarer only when Minimax converts the win.
		gDeclarer := g.Clone()
		batch := collectDeclarerExamples(gDeclarer, searchAgent, heuristicAgent, labelScorer, acceptableGap)
		for i := range batch {
			batch[i].WinProbability = handStrength
		}
		examples = append(examples, batch...)
	}

	defenderWinProbability := 1 - handStrength
	if needed&defenderBucket != 0 && probabilityInRange(defenderWinProbability, minWinProbability, maxWinProbability) {
		// Defenders qualify by their own estimated chance, not the declarer's.
		gDefender := g.Clone()
		batch := collectDefenderExamples(gDefender, searchAgent, heuristicAgent, labelScorer, acceptableGap)
		for i := range batch {
			batch[i].WinProbability = defenderWinProbability
		}
		examples = append(examples, batch...)
	}

	return examples
}

func playRamschAndCollectExamples(searchAgent *agent.SkatAgent, labelScorer *strategies.PerfectInfoMinimaxStrategy, acceptableGap float64) []CardPlayExample {
	config := agent.NewThreeWayConfig(searchAgent, searchAgent.CachedClone(), searchAgent.CachedClone().CachedClone())
	g := agent.WithAgentPlayers(game.NewGame(), config).WithCardsDealt()
	g.Mode = game.ModeRamsch
	g.TrumpSuit = game.NoSuit
	g.Declarer = nil
	g.Phase = game.PhasePlaying
	g.CurrentPlayer = game.Listener
	g.TrickStarter = game.Listener
	var hands [3][]game.Card
	for pos := range g.Players {
		hands[pos] = append([]game.Card(nil), g.Players[pos].Hand...)
	}
	winProbabilities := strategies.EstimateRamschWinProbabilities(hands)

	var byPlayer [3][]CardPlayExample
	for g.Phase == game.PhasePlaying {
		player := g.CurrentPlayer
		moves := g.GetValidMoves()
		enc := encoding.EncodeNeuralCardPlay(g, player, moves)
		scores := labelScorer.ScoreMoves(g, moves)
		policy, best := acceptableMovePolicy(moves, scores, true, acceptableGap)
		card := moves[best]
		action := encoding.CardToIndex(card)
		byPlayer[player] = append(byPlayer[player], CardPlayExample{State: enc.ToSlice(), ValidMask: enc.GetValidMask(), Action: action, Role: roleRamsch, GameMode: game.ModeRamsch, Policy: policy})
		if _, err := g.PlayCard(card); err != nil {
			panic(err)
		}
		if len(g.Trick) == 3 {
			resolveTrickAndNotify(g)
		}
	}
	minScore := g.PlayerScores[0]
	for _, score := range g.PlayerScores[1:] {
		if score < minScore {
			minScore = score
		}
	}
	var examples []CardPlayExample
	for pos, score := range g.PlayerScores {
		if score == minScore {
			for i := range byPlayer[pos] {
				byPlayer[pos][i].WinProbability = winProbabilities[pos]
			}
			examples = append(examples, byPlayer[pos]...)
		}
	}
	return examples
}

func probabilityInRange(probability, minimum, maximum float64) bool {
	return probability >= minimum && probability <= maximum
}
