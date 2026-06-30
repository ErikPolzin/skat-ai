package main

import (
	"flag"
	"fmt"
	"math"
	"math/rand"
	"runtime"
	"skat/agent"
	"skat/agent/strategies"
	"skat/game"
	"sort"
	"sync"
)

type observation struct {
	predicted   float64
	features    []float64
	declarerWon bool
}

type bucketStats struct {
	count        int
	wins         int
	sumPredicted float64
}

type logisticModel struct {
	intercept    float64
	coefficients []float64
}

func main() {
	games := flag.Int("games", 500, "number of completed non-Ramsch games to sample")
	seed := flag.Int64("seed", 1, "random seed")
	setup := flag.String("setup", "heuristic", "card-play sampling policy: heuristic, minimax, or random-contract")
	minimaxDepth := flag.Int("minimax-depth", strategies.DefaultMinimaxBaseDepth, "base search depth used by minimax sampling")
	workers := flag.Int("workers", runtime.NumCPU(), "parallel game-sampling workers")
	binWidth := flag.Float64("bin-width", 0.1, "probability width for calibration buckets")
	flag.Parse()

	rand.Seed(*seed)
	metric := strategies.NewPerfectInfoMinimaxStrategyWithDepth(1)
	observations := collectObservations(*games, *setup, *minimaxDepth, *workers, metric)
	printSummary(observations, *binWidth)
}

func collectObservations(targetGames int, setup string, minimaxDepth, workers int, metric *strategies.PerfectInfoMinimaxStrategy) []observation {
	if targetGames <= 0 {
		return nil
	}
	workers = max(1, min(workers, targetGames))
	jobs := make(chan struct{})
	results := make(chan []observation, workers)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range jobs {
				for {
					g := newSampleGame(setup, minimaxDepth)
					if g != nil {
						results <- collectGameObservations(g, metric)
						break
					}
				}
			}
		}()
	}
	go func() {
		for range targetGames {
			jobs <- struct{}{}
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	observations := make([]observation, 0, targetGames*30)
	for gameObservations := range results {
		observations = append(observations, gameObservations...)
	}
	return observations
}

func collectGameObservations(g *game.GameState, metric *strategies.PerfectInfoMinimaxStrategy) []observation {
	var predictions []float64
	var featureValues [][]float64
	for g.Phase == game.PhasePlaying {
		predictions = append(predictions, metric.EvaluateState(g))
		featureValues = append(featureValues, metric.EvaluateFeatures(g).Values)
		moves := g.GetValidMoves()
		currentAgent := agent.MustGetAgentForPlayer(g.GetCurrentPlayer())
		move := currentAgent.SelectMove(g, moves)
		if _, err := g.PlayCard(move); err != nil {
			panic(err)
		}
		if len(g.Trick) == 3 {
			trick := append([]game.Card{}, g.Trick...)
			if _, err := g.ResolveTrick(); err != nil {
				panic(err)
			}
			for _, player := range g.Players {
				if playerAgent := agent.MustGetAgentForPlayer(player); playerAgent != nil {
					playerAgent.OnTrickComplete(trick)
				}
			}
		}
	}

	declarerWon, _, _ := g.GetGameResult()
	if g.Overbid {
		declarerWon = false
	}
	observations := make([]observation, len(predictions))
	for i, prediction := range predictions {
		observations[i] = observation{predicted: prediction, features: featureValues[i], declarerWon: declarerWon}
	}
	return observations
}

func newSampleGame(setup string, minimaxDepth int) *game.GameState {
	base := agent.NewHeuristicAgent("heuristic")
	if setup == "minimax" {
		var err error
		base, err = agent.NewHybridAgent("minimax-self-play", agent.HybridAgentConfig{
			BiddingType: "heuristic", GameChoiceType: "heuristic", CardPlayType: "minimax",
			MinimaxDepth: minimaxDepth, MinimaxSearch: minimaxSelfPlaySearchConfig(minimaxDepth),
		})
		if err != nil {
			panic(err)
		}
	}
	config := agent.NewThreeWayConfig(base, base.Clone(), base.Clone())
	g := agent.WithAgentPlayers(game.NewGame(), config).WithCardsDealt()

	if setup == "heuristic" || setup == "minimax" {
		g = agent.WithAgentBidding(g, config)
		if g.Declarer == nil || g.Phase != game.PhaseSkatExchange {
			return nil
		}
		var overbid bool
		g, overbid = agent.WithAgentSkatExchange(g)
		if overbid || g.Phase != game.PhasePlaying || g.Mode == game.ModeRamsch {
			return nil
		}
		return g
	}
	if setup != "random-contract" {
		panic(fmt.Sprintf("unknown setup %q", setup))
	}

	declarer := game.GamePosition(rand.Intn(3))
	g.Declarer = &declarer
	switch rand.Intn(6) {
	case 0:
		g.Mode, g.TrumpSuit = game.ModeGrand, game.NoSuit
	case 1:
		g.Mode, g.TrumpSuit = game.ModeNull, game.NoSuit
	default:
		g.Mode = game.ModeSuit
		g.TrumpSuit = []game.Suit{game.Clubs, game.Spades, game.Hearts, game.Diamonds}[rand.Intn(4)]
	}
	g.Phase = game.PhasePlaying
	g.CurrentPlayer, g.TrickStarter = game.Listener, game.Listener
	return g
}

func minimaxSelfPlaySearchConfig(depth int) *strategies.MinimaxSearchConfig {
	config := strategies.DefaultMinimaxSearchConfig(depth)
	return &config
}

func printSummary(observations []observation, binWidth float64) {
	if len(observations) == 0 {
		fmt.Println("no observations collected")
		return
	}
	if binWidth <= 0 {
		binWidth = 0.1
	}
	buckets := make(map[int]*bucketStats)
	fitted := fitLogisticModel(observations)
	wins := 0
	directBrier, directLogLoss := 0.0, 0.0
	fittedBrier, fittedLogLoss := 0.0, 0.0
	for _, obs := range observations {
		key := int(math.Floor(obs.predicted / binWidth))
		bucket := buckets[key]
		if bucket == nil {
			bucket = &bucketStats{}
			buckets[key] = bucket
		}
		actual := 0.0
		if obs.declarerWon {
			actual = 1
			wins++
			bucket.wins++
		}
		bucket.count++
		bucket.sumPredicted += obs.predicted
		err := obs.predicted - actual
		directBrier += err * err
		directLogLoss += binaryLogLoss(obs.predicted, actual)
		fittedPrediction := fitted.predict(obs.features)
		fittedErr := fittedPrediction - actual
		fittedBrier += fittedErr * fittedErr
		fittedLogLoss += binaryLogLoss(fittedPrediction, actual)
	}

	n := float64(len(observations))
	fmt.Printf("observations: %d\n", len(observations))
	fmt.Printf("declarer win rate: %.3f\n", float64(wins)/n)
	fmt.Printf("direct probability brier score: %.4f\n", directBrier/n)
	fmt.Printf("direct probability log loss: %.4f\n", directLogLoss/n)
	fmt.Printf("fitted feature brier score: %.4f\n", fittedBrier/n)
	fmt.Printf("fitted feature log loss: %.4f\n\n", fittedLogLoss/n)
	printFittedModel(fitted, observations)
	fmt.Println("bucket_low,bucket_high,count,avg_predicted_win_rate,observed_win_rate")

	keys := make([]int, 0, len(buckets))
	for key := range buckets {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	for _, key := range keys {
		bucket := buckets[key]
		fmt.Printf("%.2f,%.2f,%d,%.3f,%.3f\n", float64(key)*binWidth, float64(key+1)*binWidth,
			bucket.count, bucket.sumPredicted/float64(bucket.count), float64(bucket.wins)/float64(bucket.count))
	}
}

func fitLogisticModel(observations []observation) logisticModel {
	featureCount := len(observations[0].features)
	model := logisticModel{coefficients: make([]float64, featureCount)}
	wins := 0.0
	for _, obs := range observations {
		if obs.declarerWon {
			wins++
		}
	}
	winRate := clampProbability(wins / float64(len(observations)))
	model.intercept = math.Log(winRate / (1 - winRate))
	const learningRate, l2 = 0.25, 0.001
	for step := 0; step < 2500; step++ {
		gradIntercept := 0.0
		gradCoefficients := make([]float64, featureCount)
		for _, obs := range observations {
			y := 0.0
			if obs.declarerWon {
				y = 1
			}
			err := model.predict(obs.features) - y
			gradIntercept += err
			for i, value := range obs.features {
				gradCoefficients[i] += err * value
			}
		}
		scale := 1 / float64(len(observations))
		model.intercept -= learningRate * gradIntercept * scale
		maxStep := math.Abs(learningRate * gradIntercept * scale)
		for i := range model.coefficients {
			update := learningRate * (gradCoefficients[i]*scale + l2*model.coefficients[i])
			model.coefficients[i] -= update
			maxStep = math.Max(maxStep, math.Abs(update))
		}
		if maxStep < 1e-7 {
			break
		}
	}
	return model
}

func (m logisticModel) predict(features []float64) float64 {
	logit := m.intercept
	for i, value := range features {
		if i >= len(m.coefficients) {
			break
		}
		logit += m.coefficients[i] * value
	}
	if logit >= 0 {
		z := math.Exp(-logit)
		return 1 / (1 + z)
	}
	z := math.Exp(logit)
	return z / (1 + z)
}

func printFittedModel(model logisticModel, observations []observation) {
	metric := strategies.NewPerfectInfoMinimaxStrategyWithDepth(1)
	names := metric.EvaluateFeatures(&game.GameState{}).Names
	fmt.Printf("fitted intercept: %.6f\n", model.intercept)
	fmt.Println("fitted coefficients:")
	for i, coefficient := range model.coefficients {
		name := fmt.Sprintf("feature_%d", i)
		if i < len(names) {
			name = names[i]
		}
		fmt.Printf("  %-22s %+.6f\n", name, coefficient)
	}
	fmt.Println("\nGo constants:")
	fmt.Printf("const minimaxWinProbIntercept = %.6f\n", model.intercept)
	fmt.Println("var minimaxEvaluationFeatures = []minimaxEvaluationFeature{")
	for i, coefficient := range model.coefficients {
		name := fmt.Sprintf("feature_%d", i)
		if i < len(names) {
			name = names[i]
		}
		fmt.Printf("\t{name: %q, coefficient: %.6f},\n", name, coefficient)
	}
	fmt.Println("}")
	fmt.Println()
}

func binaryLogLoss(predicted, actual float64) float64 {
	p := clampProbability(predicted)
	if actual == 1 {
		return -math.Log(p)
	}
	return -math.Log(1 - p)
}

func clampProbability(probability float64) float64 {
	const epsilon = 1e-9
	return math.Max(epsilon, math.Min(1-epsilon, probability))
}
