package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	"skat/agent"
	"skat/agent/strategies"
	"skat/agent/strategies/encoding"
	"skat/game"
)

type improvementExample struct {
	state      [encoding.StateFeatureSize]float32
	validMask  [32]float32
	action     int
	isDeclarer bool
	policy     [32]float32
	weight     float32
}

type rolloutCardPlayStrategy struct {
	heuristic *strategies.HeuristicCardPlayStrategy
	epsilon   float64
}

func main() {
	examplesPerRole := flag.Int("examples", 20000, "Number of improved examples to collect per role")
	outputFile := flag.String("output", ".data/policy_improvement_dataset.csv", "Output CSV file")
	role := flag.String("role", "all", "Role to collect: all, declarer, or defender")
	biddingThreshold := flag.Float64("bidding-threshold", 0.65, "Heuristic bidding threshold for contract generation")
	temperature := flag.Float64("temperature", 12.0, "Softmax temperature for action scores; lower is greedier")
	minGap := flag.Float64("min-gap", 0.0, "Minimum best-vs-current action value gap required to include an example")
	rollouts := flag.Int("rollouts", 1, "Number of randomized rollout completions per legal move")
	rolloutEpsilon := flag.Float64("rollout-epsilon", 0.0, "Probability that rollout agents choose a random legal move instead of heuristic")
	workers := flag.Int("workers", runtime.NumCPU(), "Number of worker goroutines")
	flag.Parse()

	if *role != "all" && *role != "declarer" && *role != "defender" {
		fmt.Fprintf(os.Stderr, "unknown role %q (use all, declarer, or defender)\n", *role)
		os.Exit(1)
	}

	if err := os.MkdirAll(filepath.Dir(*outputFile), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "create output dir: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("============================================================")
	fmt.Println("Skat Policy Improvement Dataset")
	fmt.Println("============================================================")
	fmt.Printf("Examples per role: %d\n", *examplesPerRole)
	fmt.Printf("Role: %s\n", *role)
	fmt.Printf("Output: %s\n", *outputFile)
	fmt.Printf("Contract bidding threshold: %.2f\n", *biddingThreshold)
	fmt.Printf("Action score temperature: %.2f\n", *temperature)
	fmt.Printf("Minimum improvement gap: %.2f\n", *minGap)
	fmt.Printf("Rollouts per move: %d\n", *rollouts)
	fmt.Printf("Rollout epsilon: %.2f\n", *rolloutEpsilon)
	fmt.Printf("Workers: %d\n\n", *workers)

	examplesChan := make(chan []improvementExample, *workers)
	doneChan := make(chan struct{})

	for i := 0; i < *workers; i++ {
		go func() {
			for {
				examplesChan <- collectGameExamples(*biddingThreshold, *temperature, *minGap, *role, *rollouts, *rolloutEpsilon)
			}
		}()
	}

	var declarerExamples []improvementExample
	var defenderExamples []improvementExample
	gamesSeen := 0

	go func() {
		for batch := range examplesChan {
			gamesSeen++
			for _, ex := range batch {
				if ex.isDeclarer {
					if len(declarerExamples) < *examplesPerRole {
						declarerExamples = append(declarerExamples, ex)
					}
				} else if len(defenderExamples) < *examplesPerRole {
					defenderExamples = append(defenderExamples, ex)
				}
			}

			if gamesSeen%100 == 0 {
				fmt.Printf("  Scored %d games -> declarer=%d defender=%d\r", gamesSeen, len(declarerExamples), len(defenderExamples))
			}

			if doneCollecting(*role, len(declarerExamples), len(defenderExamples), *examplesPerRole) {
				close(doneChan)
				return
			}
		}
	}()

	<-doneChan

	dataset := make([]improvementExample, 0, len(declarerExamples)+len(defenderExamples))
	dataset = append(dataset, declarerExamples...)
	dataset = append(dataset, defenderExamples...)

	if err := writeDataset(*outputFile, dataset); err != nil {
		fmt.Fprintf(os.Stderr, "write dataset: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n\nCollected dataset: %d declarer + %d defender = %d total examples\n",
		len(declarerExamples), len(defenderExamples), len(dataset))
	fmt.Println("Policy-improvement dataset complete.")
}

func doneCollecting(role string, declarers, defenders, target int) bool {
	switch role {
	case "declarer":
		return declarers >= target
	case "defender":
		return defenders >= target
	default:
		return declarers >= target && defenders >= target
	}
}

func collectGameExamples(biddingThreshold, temperature, minGap float64, role string, rollouts int, rolloutEpsilon float64) []improvementExample {
	g, ok := generateContract(biddingThreshold)
	if !ok {
		return nil
	}

	var examples []improvementExample
	for g.Phase == game.PhasePlaying {
		validMoves := g.GetValidMoves()
		currentPlayer := g.CurrentPlayer
		currentAgent := agent.GetAgentForPlayer(g.GetCurrentPlayer())

		isDeclarer := g.Declarer != nil && currentPlayer == *g.Declarer
		wantRole := role == "all" || (role == "declarer" && isDeclarer) || (role == "defender" && !isDeclarer)

		if wantRole && len(validMoves) > 1 && g.Declarer != nil {
			ex, ok := scoreDecision(g, currentPlayer, validMoves, currentAgent, temperature, minGap, rollouts, rolloutEpsilon)
			if ok {
				examples = append(examples, ex)
			}
		}

		move := currentAgent.SelectMove(g, validMoves)
		playMoveAndNotify(g, move)
	}
	return examples
}

func generateContract(biddingThreshold float64) (*game.GameState, bool) {
	config := agent.NewThreeWayConfig(
		newHeuristicAgent("Dealer", biddingThreshold),
		newHeuristicAgent("Listener", biddingThreshold),
		newHeuristicAgent("Speaker", biddingThreshold),
	)
	g := game.NewGame()
	g = agent.WithAgentPlayers(g, config)
	g = g.WithCardsDealt()
	g = agent.WithAgentBidding(g, config)
	g = agent.WithAgentSkatDecision(g)
	g, overbid := agent.WithAgentGameChoice(g)
	if overbid || g.Declarer == nil || g.Phase != game.PhasePlaying {
		return nil, false
	}
	return g, true
}

func scoreDecision(g *game.GameState, player game.GamePosition, validMoves []game.Card, currentAgent *agent.SkatAgent, temperature, minGap float64, rollouts int, rolloutEpsilon float64) (improvementExample, bool) {
	enc := encoding.EncodeNeuralCardPlay(g, player, validMoves)
	scores := make([]float64, len(validMoves))
	bestIdx := 0
	currentIdx := 0
	if rollouts < 1 {
		rollouts = 1
	}

	currentMove := currentAgent.SelectMove(g.Clone(), append([]game.Card{}, validMoves...))
	currentAction := encoding.CardToIndex(currentMove)

	for i, move := range validMoves {
		totalScore := 0.0
		for r := 0; r < rollouts; r++ {
			rollout := g.Clone()
			assignRolloutAgents(rollout, rolloutEpsilon)
			playMoveAndNotify(rollout, move)
			if rollout.Phase == game.PhasePlaying {
				agent.WithAgentCardPlay(rollout)
			}
			totalScore += roleScore(rollout.Result(), player == *g.Declarer)
		}
		scores[i] = totalScore / float64(rollouts)
		if scores[i] > scores[bestIdx] {
			bestIdx = i
		}
		if encoding.CardToIndex(move) == currentAction {
			currentIdx = i
		}
	}

	if scores[bestIdx]-scores[currentIdx] < minGap {
		return improvementExample{}, false
	}

	policy := softmaxPolicy(validMoves, scores, temperature)
	bestAction := encoding.CardToIndex(validMoves[bestIdx])
	return improvementExample{
		state:      enc.ToSlice(),
		validMask:  enc.GetValidMask(),
		action:     bestAction,
		isDeclarer: player == *g.Declarer,
		policy:     policy,
		weight:     float32(1.0 + math.Max(0, scores[bestIdx]-scores[currentIdx])/60.0),
	}, true
}

func assignRolloutAgents(g *game.GameState, epsilon float64) {
	for _, pos := range game.AllPositions {
		a := agent.NewAgentWithStrategies(
			fmt.Sprintf("Rollout-%d", pos),
			strategies.NewHeuristicBiddingStrategy(),
			strategies.NewHeuristicGameChoiceStrategy(),
			newRolloutCardPlayStrategy(epsilon),
		)
		a.OnGameStart()
		for _, trick := range g.CardsPlayed {
			a.OnTrickComplete(trick)
		}
		agent.SetAgentForPlayer(g.GetPlayerByPosition(pos), a)
	}
}

func newRolloutCardPlayStrategy(epsilon float64) *rolloutCardPlayStrategy {
	return &rolloutCardPlayStrategy{
		heuristic: strategies.NewHeuristicCardPlayStrategy(),
		epsilon:   epsilon,
	}
}

func (s *rolloutCardPlayStrategy) GetName() string {
	return "RolloutHeuristic"
}

func (s *rolloutCardPlayStrategy) SelectMove(g *game.GameState, validMoves []game.Card) game.Card {
	if len(validMoves) == 1 {
		return validMoves[0]
	}
	if s.epsilon > 0 && rand.Float64() < s.epsilon {
		return validMoves[rand.Intn(len(validMoves))]
	}
	return s.heuristic.SelectMove(g, validMoves)
}

func (s *rolloutCardPlayStrategy) OnTrickComplete(trick []game.Card) {
	s.heuristic.OnTrickComplete(trick)
}

func (s *rolloutCardPlayStrategy) Reset() {
	s.heuristic.Reset()
}

func roleScore(result game.GameResult, isDeclarer bool) float64 {
	if isDeclarer {
		return float64(result.Value)
	}
	return float64(-result.Value)
}

func softmaxPolicy(validMoves []game.Card, scores []float64, temperature float64) [32]float32 {
	var policy [32]float32
	if temperature <= 0 {
		temperature = 1
	}
	maxScore := scores[0]
	for _, score := range scores[1:] {
		if score > maxScore {
			maxScore = score
		}
	}

	denom := 0.0
	values := make([]float64, len(scores))
	for i, score := range scores {
		values[i] = math.Exp((score - maxScore) / temperature)
		denom += values[i]
	}
	for i, move := range validMoves {
		policy[encoding.CardToIndex(move)] = float32(values[i] / denom)
	}
	return policy
}

func playMoveAndNotify(g *game.GameState, move game.Card) {
	if _, err := g.PlayCard(move); err != nil {
		panic(fmt.Sprintf("PlayCard error: %v", err))
	}
	if len(g.Trick) == 3 {
		trick := append([]game.Card{}, g.Trick...)
		if _, err := g.ResolveTrick(); err != nil {
			panic(fmt.Sprintf("ResolveTrick error: %v", err))
		}
		for i := range g.Players {
			if g.Players[i].IsAgent {
				if a := agent.GetAgentForPlayer(g.Players[i]); a != nil {
					a.OnTrickComplete(trick)
				}
			}
		}
	}
}

func newHeuristicAgent(role string, biddingThreshold float64) *agent.SkatAgent {
	return agent.NewAgentWithStrategies(
		"Heuristic "+role,
		contractBidding(biddingThreshold),
		contractGameChoice(biddingThreshold),
		agent.NewHeuristicCardPlayStrategy(),
	)
}

func contractBidding(threshold float64) agent.BiddingStrategy {
	config := strategies.DefaultContractEvaluatorConfig()
	config.MinWinProbability = threshold
	return strategies.NewHeuristicBiddingStrategyWithConfig(config)
}

func contractGameChoice(threshold float64) agent.GameChoiceStrategy {
	config := strategies.DefaultContractEvaluatorConfig()
	config.MinWinProbability = threshold
	return strategies.NewHeuristicGameChoiceStrategyWithConfig(config)
}

func writeDataset(path string, examples []improvementExample) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	header := make([]string, 0, encoding.StateFeatureSize+32+2+32+1)
	for i := 0; i < encoding.StateFeatureSize; i++ {
		header = append(header, fmt.Sprintf("s%d", i))
	}
	for i := 0; i < 32; i++ {
		header = append(header, fmt.Sprintf("m%d", i))
	}
	header = append(header, "action", "is_declarer")
	for i := 0; i < 32; i++ {
		header = append(header, fmt.Sprintf("p%d", i))
	}
	header = append(header, "weight")
	if err := writer.Write(header); err != nil {
		return err
	}

	for _, ex := range examples {
		record := make([]string, 0, len(header))
		for _, val := range ex.state {
			record = append(record, strconv.FormatFloat(float64(val), 'f', 6, 32))
		}
		for _, val := range ex.validMask {
			record = append(record, strconv.FormatFloat(float64(val), 'f', 0, 32))
		}
		record = append(record, strconv.Itoa(ex.action))
		if ex.isDeclarer {
			record = append(record, "1")
		} else {
			record = append(record, "0")
		}
		for _, val := range ex.policy {
			record = append(record, strconv.FormatFloat(float64(val), 'f', 6, 32))
		}
		record = append(record, strconv.FormatFloat(float64(ex.weight), 'f', 4, 32))
		if err := writer.Write(record); err != nil {
			return err
		}
	}
	return writer.Error()
}
