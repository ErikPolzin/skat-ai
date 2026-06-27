package main

import (
	"flag"
	"fmt"
	"math"
	"math/rand"
	"skat/agent"
	"skat/agent/strategies"
	"skat/game"
	"sort"
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
	setup := flag.String("setup", "heuristic", "game setup: heuristic or random-contract")
	binWidth := flag.Float64("bin-width", 0.1, "probability width for calibration buckets")
	flag.Parse()

	rand.Seed(*seed)
	metric := strategies.NewPerfectInfoMinimaxStrategyWithDepth(1)
	observations := collectObservations(*games, *setup, metric)
	printSummary(observations, *binWidth)
}

func collectObservations(targetGames int, setup string, metric *strategies.PerfectInfoMinimaxStrategy) []observation {
	var observations []observation
	completed := 0
	attempts := 0

	for completed < targetGames {
		attempts++
		g := newSampleGame(setup)
		if g == nil {
			continue
		}

		var gamePredictions []float64
		var gameFeatures [][]float64
		for g.Phase == game.PhasePlaying {
			gamePredictions = append(gamePredictions, metric.EvaluateState(g))
			features := metric.EvaluateFeatures(g)
			gameFeatures = append(gameFeatures, features.Values)
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
					playerAgent := agent.MustGetAgentForPlayer(player)
					if playerAgent != nil {
						playerAgent.OnTrickComplete(trick)
					}
				}
			}
		}

		declarerWon, _, _ := g.GetGameResult()
		for i, prediction := range gamePredictions {
			observations = append(observations, observation{
				predicted:   prediction,
				features:    gameFeatures[i],
				declarerWon: declarerWon,
			})
		}
		completed++
		if attempts > targetGames*20 {
			break
		}
	}
	return observations
}

func newSampleGame(setup string) *game.GameState {
	base := agent.NewHeuristicAgent("heuristic")
	config := agent.NewThreeWayConfig(base, base.Clone(), base.Clone())
	g := agent.WithAgentPlayers(game.NewGame(), config).WithCardsDealt()

	if setup == "heuristic" {
		g = agent.WithAgentBidding(g, config)
		if g.Declarer == nil || g.Phase != game.PhaseSkatExchange {
			return nil
		}
		g = agent.WithAgentSkatDecision(g)
		var overbid bool
		g, overbid = agent.WithAgentGameChoice(g)
		if overbid || g.Phase != game.PhasePlaying || g.Mode == game.ModeRamsch {
			return nil
		}
		return g
	}

	declarer := game.GamePosition(rand.Intn(3))
	g.Declarer = &declarer
	switch rand.Intn(6) {
	case 0:
		g.Mode = game.ModeGrand
		g.TrumpSuit = game.NoSuit
	case 1:
		g.Mode = game.ModeNull
		g.TrumpSuit = game.NoSuit
	default:
		g.Mode = game.ModeSuit
		g.TrumpSuit = []game.Suit{game.Clubs, game.Spades, game.Hearts, game.Diamonds}[rand.Intn(4)]
	}
	g.Phase = game.PhasePlaying
	g.CurrentPlayer = game.Listener
	g.TrickStarter = game.Listener
	return g
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
	directBrier := 0.0
	directLogLoss := 0.0
	fittedBrier := 0.0
	fittedLogLoss := 0.0
	for _, obs := range observations {
		key := int(math.Floor(obs.predicted / binWidth))
		bucket := buckets[key]
		if bucket == nil {
			bucket = &bucketStats{}
			buckets[key] = bucket
		}
		bucket.count++
		actual := 0.0
		if obs.declarerWon {
			wins++
			bucket.wins++
			actual = 1
		}
		bucket.sumPredicted += obs.predicted
		err := obs.predicted - actual
		directBrier += err * err
		directLogLoss += binaryLogLoss(obs.predicted, actual)
		fittedPredicted := fitted.predict(obs.features)
		fittedErr := fittedPredicted - actual
		fittedBrier += fittedErr * fittedErr
		fittedLogLoss += binaryLogLoss(fittedPredicted, actual)
	}

	fmt.Printf("observations: %d\n", len(observations))
	fmt.Printf("declarer win rate: %.3f\n", float64(wins)/float64(len(observations)))
	fmt.Printf("direct probability brier score: %.4f\n", directBrier/float64(len(observations)))
	fmt.Printf("direct probability log loss: %.4f\n", directLogLoss/float64(len(observations)))
	fmt.Printf("fitted feature brier score: %.4f\n", fittedBrier/float64(len(observations)))
	fmt.Printf("fitted feature log loss: %.4f\n\n", fittedLogLoss/float64(len(observations)))
	printFittedModel(fitted, observations)
	fmt.Println("bucket_low,bucket_high,count,avg_predicted_win_rate,observed_win_rate")

	keys := make([]int, 0, len(buckets))
	for key := range buckets {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	for _, key := range keys {
		bucket := buckets[key]
		low := float64(key) * binWidth
		high := low + binWidth
		fmt.Printf("%.2f,%.2f,%d,%.3f,%.3f\n",
			low,
			high,
			bucket.count,
			bucket.sumPredicted/float64(bucket.count),
			float64(bucket.wins)/float64(bucket.count),
		)
	}
}

func fitLogisticModel(observations []observation) logisticModel {
	if len(observations) == 0 {
		return logisticModel{}
	}
	featureCount := len(observations[0].features)
	model := logisticModel{coefficients: make([]float64, featureCount)}
	winRate := 0.0
	for _, obs := range observations {
		if obs.declarerWon {
			winRate++
		}
	}
	winRate = clampProbability(winRate / float64(len(observations)))
	model.intercept = math.Log(winRate / (1 - winRate))

	learningRate := 0.25
	l2 := 0.001
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
			grad := gradCoefficients[i]*scale + l2*model.coefficients[i]
			update := learningRate * grad
			model.coefficients[i] -= update
			if math.Abs(update) > maxStep {
				maxStep = math.Abs(update)
			}
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
	return sigmoid(logit)
}

func sigmoid(x float64) float64 {
	if x >= 0 {
		z := math.Exp(-x)
		return 1 / (1 + z)
	}
	z := math.Exp(x)
	return z / (1 + z)
}

func printFittedModel(model logisticModel, observations []observation) {
	if len(observations) == 0 {
		return
	}
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

func clampProbability(p float64) float64 {
	const epsilon = 1e-9
	if p < epsilon {
		return epsilon
	}
	if p > 1-epsilon {
		return 1 - epsilon
	}
	return p
}
