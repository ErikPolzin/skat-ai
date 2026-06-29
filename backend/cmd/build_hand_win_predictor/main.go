package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"skat/agent"
	"skat/agent/strategies"
	"skat/game"
	"sync"
)

type completedRollout struct {
	states      []*game.GameState
	declarerWon bool
}

func main() {
	games := flag.Int("games", 100000, "completed suit/grand heuristic games")
	maxCards := flag.Int("max-cards", 6, "largest remaining hand stored")
	maxBuckets := flag.Int("max-buckets", 10000000, "maximum predictor buckets (0 means unlimited)")
	workers := flag.Int("workers", runtime.NumCPU(), "parallel rollout workers")
	seed := flag.Int64("seed", 1, "random seed")
	output := flag.String("output", ".data/hand_win_predictor.gob", "output predictor")
	input := flag.String("input", "", "existing predictor to refine")
	holdoutGames := flag.Int("holdout-games", 1000, "fresh heuristic games used to measure lookup coverage")
	minSamples := flag.Uint64("min-samples", 8, "minimum samples for holdout lookup coverage")
	flag.Parse()
	if *games <= 0 || *maxCards < 1 || *maxCards > 10 || *maxBuckets < 0 {
		fmt.Fprintln(os.Stderr, "games must be positive, max-cards must be between 1 and 10, and max-buckets cannot be negative")
		os.Exit(1)
	}
	rand.Seed(*seed)
	workerCount := max(1, min(*workers, *games))
	var predictor *strategies.HandWinPredictor
	if *input != "" {
		var err error
		predictor, err = strategies.LoadHandWinPredictor(*input)
		if err != nil {
			fmt.Fprintf(os.Stderr, "load hand win predictor: %v\n", err)
			os.Exit(1)
		}
		*maxCards = predictor.MaxCards
	}
	if predictor != nil {
		predictor.MaxBuckets = *maxBuckets
	}
	if predictor == nil {
		predictor = strategies.NewHandWinPredictor(*maxCards)
		predictor.MaxBuckets = *maxBuckets
	}
	buildPredictor(predictor, *games, *maxCards, workerCount)
	if err := predictor.Save(*output); err != nil {
		fmt.Fprintf(os.Stderr, "save hand win predictor: %v\n", err)
		os.Exit(1)
	}
	printPredictorSummary(predictor, *games)
	fmt.Printf("saved: %s\n", *output)
	if *holdoutGames > 0 {
		evaluateHoldout(predictor, *holdoutGames, *maxCards, *minSamples)
	}
}

func buildPredictor(predictor *strategies.HandWinPredictor, games, maxCards, workerCount int) {
	jobs := make(chan struct{})
	results := make(chan completedRollout, workerCount)
	var wg sync.WaitGroup
	for range workerCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range jobs {
				for {
					rollout, ok := playHeuristicRollout(maxCards)
					if !ok {
						continue
					}
					results <- rollout
					break
				}
			}
		}()
	}
	go func() {
		for range games {
			jobs <- struct{}{}
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	for rollout := range results {
		for _, state := range rollout.states {
			predictor.Observe(state, rollout.declarerWon)
		}
	}
}

func printPredictorSummary(predictor *strategies.HandWinPredictor, games int) {
	covered := map[uint64]int{1: 0, 2: 0, 4: 0, 8: 0, 16: 0}
	observations := uint64(0)
	for _, stats := range predictor.Buckets {
		observations += stats.Count
		for threshold := range covered {
			if stats.Count >= threshold {
				covered[threshold]++
			}
		}
	}
	fmt.Printf("games this run: %d\nobservations: %d\nbuckets: %d\n", games, observations, len(predictor.Buckets))
	if predictor.MaxBuckets > 0 {
		fmt.Printf("bucket limit: %d (%.1f%% full)\n", predictor.MaxBuckets, 100*float64(len(predictor.Buckets))/float64(predictor.MaxBuckets))
	} else {
		fmt.Println("bucket limit: unlimited")
	}
	for _, threshold := range []uint64{1, 2, 4, 8, 16} {
		fmt.Printf("buckets with >= %d samples: %d\n", threshold, covered[threshold])
	}
}

type holdoutStats struct {
	states int
	hits   int
	brier  float64
	levels [4]int
}

func evaluateHoldout(predictor *strategies.HandWinPredictor, games, maxCards int, minSamples uint64) {
	byCards := make(map[int]*holdoutStats)
	completed := 0
	for completed < games {
		rollout, ok := playHeuristicRollout(maxCards)
		if !ok {
			continue
		}
		actual := 0.0
		if rollout.declarerWon {
			actual = 1
		}
		for _, state := range rollout.states {
			cards := len(state.Players[0].Hand)
			stats := byCards[cards]
			if stats == nil {
				stats = &holdoutStats{}
				byCards[cards] = stats
			}
			stats.states++
			if estimate, found := predictor.Lookup(state, minSamples); found {
				stats.hits++
				if estimate.Level >= 0 && estimate.Level < len(stats.levels) {
					stats.levels[estimate.Level]++
				}
				err := estimate.WinProbability - actual
				stats.brier += err * err
			}
		}
		completed++
	}
	fmt.Printf("holdout games: %d (minimum %d samples)\n", games, minSamples)
	for cards := 1; cards <= maxCards; cards++ {
		stats := byCards[cards]
		if stats == nil {
			continue
		}
		brier := 0.0
		if stats.hits > 0 {
			brier = stats.brier / float64(stats.hits)
		}
		fmt.Printf("  %d cards: %d/%d lookups (%.1f%%), levels %d/%d/%d/%d, lookup Brier %.4f\n",
			cards, stats.hits, stats.states, 100*float64(stats.hits)/float64(stats.states),
			stats.levels[0], stats.levels[1], stats.levels[2], stats.levels[3], brier)
	}
}

func playHeuristicRollout(maxCards int) (completedRollout, bool) {
	base := agent.NewHeuristicAgent("hand-win-predictor-heuristic")
	config := agent.NewThreeWayConfig(base, base.Clone(), base.Clone())
	g := agent.WithAgentPlayers(game.NewGame(), config).WithCardsDealt()
	defer func() {
		for _, player := range g.Players {
			agent.RemoveAgentForPlayer(player)
		}
	}()
	g = agent.WithAgentBidding(g, config)
	if g.Declarer == nil || g.Phase != game.PhaseSkatExchange {
		return completedRollout{}, false
	}
	g = agent.WithAgentSkatDecision(g)
	var overbid bool
	g, overbid = agent.WithAgentGameChoice(g)
	if overbid || g.Phase != game.PhasePlaying || (g.Mode != game.ModeSuit && g.Mode != game.ModeGrand) {
		return completedRollout{}, false
	}

	rollout := completedRollout{}
	for g.Phase == game.PhasePlaying {
		if len(g.Trick) == 0 && len(g.Players[0].Hand) <= maxCards {
			rollout.states = append(rollout.states, g.Clone())
		}
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
	rollout.declarerWon, _, _ = g.GetGameResult()
	if g.Overbid {
		rollout.declarerWon = false
	}
	return rollout, true
}
