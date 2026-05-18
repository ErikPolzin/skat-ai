package main

import (
	"flag"
	"fmt"
	"os"
	"skat/agent"
	"skat/agent/strategies"
	"skat/game"
)

type stats struct {
	games       int
	wins        int
	points      int
	overbid     int
	zwang       int
	grandGames  int
	grandWins   int
	suitGames   int
	suitWins    int
	nullGames   int
	nullWins    int
	maxBid      int
	bidDecision int
}

func main() {
	strategy := flag.String("strategy", "heuristic", "Bidding/game choice strategy: heuristic or contract")
	games := flag.Int("games", 5000, "Games per threshold")
	start := flag.Float64("start", 0.30, "First threshold")
	end := flag.Float64("end", 0.80, "Last threshold")
	step := flag.Float64("step", 0.05, "Threshold increment")
	flag.Parse()

	if *games <= 0 || *step <= 0 || *end < *start {
		fmt.Fprintln(os.Stderr, "invalid sweep parameters")
		os.Exit(1)
	}

	fmt.Println("strategy,threshold,games,declarer_wins,declarer_win_rate,avg_points,overbid_rate,forced_zero_bid_rate,grand_games,grand_win_rate,suit_games,suit_win_rate,null_games,null_win_rate,max_bid,bid_decisions")
	for threshold := *start; threshold <= *end+(*step/2); threshold += *step {
		s, err := runThreshold(*strategy, threshold, *games)
		if err != nil {
			fmt.Fprintf(os.Stderr, "threshold %.3f: %v\n", threshold, err)
			os.Exit(1)
		}

		fmt.Printf("%s,%.3f,%d,%d,%.4f,%.2f,%.4f,%.4f,%d,%.4f,%d,%.4f,%d,%.4f,%d,%d\n",
			*strategy,
			threshold,
			s.games,
			s.wins,
			rate(s.wins, s.games),
			float64(s.points)/float64(s.games),
			rate(s.overbid, s.games),
			rate(s.zwang, s.games),
			s.grandGames,
			rate(s.grandWins, s.grandGames),
			s.suitGames,
			rate(s.suitWins, s.suitGames),
			s.nullGames,
			rate(s.nullWins, s.nullGames),
			s.maxBid,
			s.bidDecision,
		)
	}
}

func runThreshold(strategy string, threshold float64, games int) (stats, error) {
	a, err := newAgent(strategy, threshold)
	if err != nil {
		return stats{}, err
	}
	a.EnableMetrics()

	var s stats
	g := game.NewGame()
	config := agent.AgentConfig{
		TestAgent: a,
		Bidding:   agent.BiddingThreeTest,
		Playing:   agent.PlayingAsIs,
	}
	g = agent.WithAgentPlayers(g, config)

	for i := 0; i < games; i++ {
		g = g.WithCardsDealt()
		g = agent.WithAgentBidding(g, config)

		if g.IsZwangsspiel() {
			s.zwang++
		}
		if g.BidValue > s.maxBid {
			s.maxBid = g.BidValue
		}

		g = agent.WithAgentSkatDecision(g)
		g, _ = agent.WithAgentGameChoice(g)
		if !g.Overbid {
			g = agent.WithAgentCardPlay(g)
		}

		declarerWon, _, _ := g.GetGameResult()
		s.games++
		if declarerWon {
			s.wins++
		}
		s.points += g.Result().Value
		if g.Overbid {
			s.overbid++
		}

		switch g.Mode {
		case game.ModeGrand:
			s.grandGames++
			if declarerWon {
				s.grandWins++
			}
		case game.ModeSuit:
			s.suitGames++
			if declarerWon {
				s.suitWins++
			}
		case game.ModeNull:
			s.nullGames++
			if declarerWon {
				s.nullWins++
			}
		}

		g.NextGame()
	}
	s.bidDecision = countBidDecisions(a.GetMetrics())

	return s, nil
}

func newAgent(strategy string, threshold float64) (*agent.SkatAgent, error) {
	switch strategy {
	case "heuristic", "contract":
		config := strategies.DefaultContractEvaluatorConfig()
		config.MinWinProbability = threshold
		return agent.NewAgentWithStrategies(
			"Sweeper",
			strategies.NewHeuristicBiddingStrategyWithConfig(config),
			strategies.NewHeuristicGameChoiceStrategyWithConfig(config),
			strategies.NewHeuristicCardPlayStrategy(),
		), nil
	default:
		return nil, fmt.Errorf("unknown strategy %q", strategy)
	}
}

func rate(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return float64(n) / float64(d)
}

func countBidDecisions(m agent.AgentMetricsSnapshot) int {
	total := 0
	for _, n := range m.BiddingAccepts {
		total += n
	}
	for _, n := range m.BiddingRejects {
		total += n
	}
	return total
}
