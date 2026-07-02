package main

import (
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"strings"
	"time"

	"skat/agent"
	"skat/agent/strategies"
	"skat/game"
)

type bucket struct {
	count     int
	wins      int
	predicted float64
}

type summary struct {
	games     int
	wins      int
	predicted float64
}

func main() {
	games := flag.Int("games", 5000, "number of random forced-contract games")
	weights := flag.String("weights", ".data/models/contract.weights", "contract network weights")
	seed := flag.Int64("seed", 0, "random seed (0 chooses a fresh seed)")
	bins := flag.Int("bins", 10, "number of probability histogram bins")
	diffRange := flag.Float64("diff-range", 25, "signed histogram range in percentage points")
	contractTypesFlag := flag.String("contract-types", "all", "comma-separated: grand,suit,null,pickup,hand,null-hand,schneider,schwarz")
	flag.Parse()
	if *games <= 0 || *bins <= 0 || *diffRange <= 0 {
		fmt.Fprintln(os.Stderr, "games, bins, and diff-range must be positive")
		os.Exit(1)
	}
	contractTypes, err := parseContractTypes(*contractTypesFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	estimator, err := strategies.NewNeuralContractWinProbabilityEstimatorFromWeights(*weights)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load weights: %v\n", err)
		os.Exit(1)
	}
	resolvedSeed := *seed
	if resolvedSeed == 0 {
		resolvedSeed = time.Now().UnixNano()
	}
	rng := rand.New(rand.NewSource(resolvedSeed))
	discarder := strategies.NewHeuristicGameChoiceStrategy()
	baseAgent := agent.NewHeuristicAgent("contract-eval")
	config := agent.NewThreeWayConfig(baseAgent, baseAgent.Clone(), baseAgent.Clone())
	histogram := make([]bucket, *bins)
	byDeclaration := map[string]*summary{}
	byMode := map[string]*summary{}

	for gameIndex := 0; gameIndex < *games; {
		g := agent.WithAgentPlayers(game.NewGame(), config)
		for _, player := range g.Players {
			agent.MustGetAgentForPlayer(player).OnGameStart()
		}
		g.GameNumber = gameIndex
		if _, err := g.DealWithRand(rng); err != nil {
			panic(err)
		}
		declarer := game.GamePosition(rng.Intn(3))
		g = g.WithDeclarer(declarer, 0)
		originalHand := append(game.Cards(nil), g.Players[declarer].Hand...)
		mode, suit := randomContract(rng)
		playedHand, schneider, schwarz := randomDeclaration(rng, mode)
		if !matchesContractType(contractTypes, mode, playedHand, schneider, schwarz) {
			continue
		}
		gameIndex++

		if _, err := g.SkatDecision(!playedHand); err != nil {
			panic(err)
		}
		if !playedHand {
			choice := discarder.ChooseGameAndSkatDiscardForContract(g.Players[declarer].Hand, mode, suit)
			if _, err := g.Discard(choice.Discard[0], choice.Discard[1]); err != nil {
				panic(err)
			}
		}
		if _, err := g.DeclareGame(mode, suit, schneider, schwarz); err != nil {
			panic(err)
		}
		predicted := estimator.EstimateWinProbability(originalHand, mode, suit, playedHand, schneider, schwarz)
		agent.WithAgentCardPlay(g)
		won := g.Result().DeclarerWon

		index := min(*bins-1, int(math.Floor(predicted*float64(*bins))))
		histogram[index].count++
		histogram[index].predicted += predicted
		if won {
			histogram[index].wins++
		}
		addSummary(byMode, string(mode), predicted, won)
		addSummary(byDeclaration, declarationLabel(mode, playedHand, schneider, schwarz), predicted, won)
	}

	fmt.Printf("Neural contract calibration: %d random forced games, seed %d\n", *games, resolvedSeed)
	fmt.Printf("Contract types: %s\n", *contractTypesFlag)
	fmt.Printf("Weights: %s\n\n", *weights)
	fmt.Printf("%-11s %7s %10s %10s %9s  - over (%.0fpp) %s under +\n", "Probability", "Games", "Predicted", "Actual", "Diff", *diffRange, strings.Repeat(" ", 23))
	for i, b := range histogram {
		if b.count == 0 {
			continue
		}
		predicted := b.predicted / float64(b.count)
		actual := float64(b.wins) / float64(b.count)
		fmt.Printf("%3d-%3d%% %8d %9.1f%% %9.1f%% %+8.1f  %s\n",
			i*100/(*bins), (i+1)*100/(*bins), b.count, predicted*100, actual*100,
			(actual-predicted)*100, diffBar(actual-predicted, *diffRange/100))
	}
	printSummaries("Mode", byMode, []string{string(game.ModeGrand), string(game.ModeSuit), string(game.ModeNull)})
	printSummaries("Forced declaration", byDeclaration, []string{"Pickup", "Hand", "Schneider announced", "Schwarz announced", "Null Hand"})
}

var validContractTypes = map[string]bool{
	"all": true, "grand": true, "suit": true, "null": true,
	"pickup": true, "hand": true, "null-hand": true,
	"schneider": true, "schwarz": true,
}

func parseContractTypes(value string) (map[string]bool, error) {
	result := make(map[string]bool)
	for _, raw := range strings.Split(strings.ToLower(value), ",") {
		contractType := strings.TrimSpace(raw)
		if !validContractTypes[contractType] {
			return nil, fmt.Errorf("unknown contract type %q", contractType)
		}
		result[contractType] = true
	}
	return result, nil
}

func matchesContractType(filter map[string]bool, mode game.GameMode, hand, schneider, schwarz bool) bool {
	if filter["all"] || filter[string(mode)] {
		return true
	}
	switch {
	case schwarz:
		return filter["schwarz"]
	case schneider:
		return filter["schneider"]
	case mode == game.ModeNull && hand:
		return filter["null-hand"]
	case hand:
		return filter["hand"]
	default:
		return filter["pickup"]
	}
}

func randomContract(rng *rand.Rand) (game.GameMode, game.Suit) {
	switch rng.Intn(3) {
	case 0:
		return game.ModeGrand, game.NoSuit
	case 1:
		return game.ModeSuit, game.Suit(int(game.Clubs) + rng.Intn(4))
	default:
		return game.ModeNull, game.NoSuit
	}
}

func randomDeclaration(rng *rand.Rand, mode game.GameMode) (bool, bool, bool) {
	variants := 4
	if mode == game.ModeNull {
		variants = 2
	}
	switch rng.Intn(variants) {
	case 0:
		return false, false, false
	case 1:
		return true, false, false
	case 2:
		return true, true, false
	default:
		return true, true, true
	}
}

func declarationLabel(mode game.GameMode, hand, schneider, schwarz bool) string {
	if schwarz {
		return "Schwarz announced"
	}
	if schneider {
		return "Schneider announced"
	}
	if mode == game.ModeNull && hand {
		return "Null Hand"
	}
	if hand {
		return "Hand"
	}
	return "Pickup"
}

func addSummary(summaries map[string]*summary, label string, predicted float64, won bool) {
	s := summaries[label]
	if s == nil {
		s = &summary{}
		summaries[label] = s
	}
	s.games++
	s.predicted += predicted
	if won {
		s.wins++
	}
}

func printSummaries(title string, summaries map[string]*summary, order []string) {
	fmt.Printf("\n%s win rates:\n", title)
	for _, label := range order {
		s := summaries[label]
		if s == nil {
			continue
		}
		predicted := s.predicted / float64(s.games) * 100
		actual := float64(s.wins) / float64(s.games) * 100
		fmt.Printf("  %-21s %7d games  predicted %6.2f%%  actual %6.2f%%  diff %+6.2fpp\n",
			label+":", s.games, predicted, actual, actual-predicted)
	}
}

func diffBar(diff, maxDiff float64) string {
	const halfWidth = 20
	extent := min(halfWidth, int(math.Round(math.Abs(diff)/maxDiff*halfWidth)))
	if extent == 0 && math.Abs(diff) > 0.001 {
		extent = 1
	}
	negative, positive := 0, 0
	if diff < 0 {
		negative = extent
	} else {
		positive = extent
	}
	return strings.Repeat(" ", halfWidth-negative) + strings.Repeat("▒", negative) +
		"│" + strings.Repeat("▒", positive) + strings.Repeat(" ", halfWidth-positive)
}
