package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"runtime"
	"skat/agent"
	"skat/agent/training"
	"sort"
	"strings"
)

// AgentDefinition describes an agent configuration for the tournament
type AgentDefinition struct {
	Name   string
	Config agent.HybridAgentConfig
}

// TournamentResult holds results for a matchup between two agents
type TournamentResult struct {
	Agent1Name string
	Agent2Name string
	Agent1Wins int
	Agent2Wins int
	TotalGames int
}

// AgentRanking holds ELO rating and statistics for an agent
type AgentRanking struct {
	Name          string
	ELO           float64
	Wins          int
	Losses        int
	TotalGames    int
	AvgPoints     float64
	DeclarerWinPct float64
	DefenderWinPct float64
}

func main() {
	games := flag.Int("games", 200, "Number of games per matchup")
	qtablePath := flag.String("qtable-path", ".", "Path to directory containing Q-table files")
	dqnPath := flag.String("dqn-path", ".data/models/dqn_cardplay", "Path to DQN weights (without .declarer/.defender suffix)")
	flag.Parse()

	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║          SKAT AGENT TOURNAMENT - ROUND ROBIN              ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Printf("\nGames per matchup: %d\n", *games)
	fmt.Printf("CPU cores: %d\n\n", runtime.GOMAXPROCS(0))

	// Define all agents to compete
	agents := defineAgents(*qtablePath, *dqnPath)

	fmt.Println("Competing agents:")
	for i, a := range agents {
		fmt.Printf("  %d. %s\n", i+1, a.Name)
	}
	fmt.Println()

	// Run round-robin tournament
	results := runRoundRobin(agents, *games)

	// Calculate ELO ratings
	rankings := calculateELORankings(agents, results)

	// Display results
	displayTournamentResults(results)
	displayRankings(rankings)
}

// defineAgents creates all agent configurations for the tournament
func defineAgents(qtablePath, dqnPath string) []AgentDefinition {
	agents := []AgentDefinition{
		{
			Name: "Heuristic",
			Config: agent.HybridAgentConfig{
				BiddingType:    "heuristic",
				GameChoiceType: "heuristic",
				CardPlayType:   "heuristic",
			},
		},
		{
			Name: "Weighted-0.60",
			Config: agent.HybridAgentConfig{
				BiddingType:      "weighted",
				BiddingThreshold: 0.60,
				GameChoiceType:   "heuristic",
				CardPlayType:     "heuristic",
			},
		},
		{
			Name: "Weighted-0.65",
			Config: agent.HybridAgentConfig{
				BiddingType:      "weighted",
				BiddingThreshold: 0.65,
				GameChoiceType:   "heuristic",
				CardPlayType:     "heuristic",
			},
		},
		{
			Name: "Weighted-0.70",
			Config: agent.HybridAgentConfig{
				BiddingType:      "weighted",
				BiddingThreshold: 0.70,
				GameChoiceType:   "heuristic",
				CardPlayType:     "heuristic",
			},
		},
		{
			Name: "MCTS-500",
			Config: agent.HybridAgentConfig{
				BiddingType:     "weighted",
				BiddingThreshold: 0.65,
				GameChoiceType:  "heuristic",
				CardPlayType:    "mcts",
				MCTSSimulations: 500,
			},
		},
	}

	// Add Q-learning agents if Q-tables exist
	biddingQTablePath := qtablePath + "/bidding_qtable.gob"
	gameChoiceQTablePath := qtablePath + "/game_choice_qtable.gob"

	if _, err := os.Stat(biddingQTablePath); err == nil {
		data, err := agent.LoadQTableData(biddingQTablePath, false)
		if err == nil {
			agents = append(agents, AgentDefinition{
				Name: "QL-Bidding",
				Config: agent.HybridAgentConfig{
					BiddingType:    "qlearning",
					BiddingQTable:  data.QTable,
					GameChoiceType: "heuristic",
					CardPlayType:   "heuristic",
				},
			})
		}
	}

	if _, err := os.Stat(gameChoiceQTablePath); err == nil {
		data, err := agent.LoadQTableData(gameChoiceQTablePath, false)
		if err == nil {
			agents = append(agents, AgentDefinition{
				Name: "QL-GameChoice",
				Config: agent.HybridAgentConfig{
					BiddingType:      "weighted",
					BiddingThreshold: 0.65,
					GameChoiceType:   "qlearning",
					GameChoiceQTable: data.QTable,
					CardPlayType:     "heuristic",
				},
			})
		}
	}

	// Add combined Q-learning if both exist
	if _, err := os.Stat(biddingQTablePath); err == nil {
		if _, err := os.Stat(gameChoiceQTablePath); err == nil {
			bData, err1 := agent.LoadQTableData(biddingQTablePath, false)
			gData, err2 := agent.LoadQTableData(gameChoiceQTablePath, false)
			if err1 == nil && err2 == nil {
				agents = append(agents, AgentDefinition{
					Name: "QL-Combined",
					Config: agent.HybridAgentConfig{
						BiddingType:      "qlearning",
						BiddingQTable:    bData.QTable,
						GameChoiceType:   "qlearning",
						GameChoiceQTable: gData.QTable,
						CardPlayType:     "heuristic",
					},
				})
			}
		}
	}

	// Add DQN agent if weights exist
	declarerPath := dqnPath + ".declarer"
	defenderPath := dqnPath + ".defender"
	if _, err := os.Stat(declarerPath); err == nil {
		if _, err := os.Stat(defenderPath); err == nil {
			agents = append(agents, AgentDefinition{
				Name: "DQN-CardPlay",
				Config: agent.HybridAgentConfig{
					BiddingType:      "weighted",
					BiddingThreshold: 0.65,
					GameChoiceType:   "heuristic",
					CardPlayType:     "dqn",
					DQNDeclarerPath:  declarerPath,
					DQNDefenderPath:  defenderPath,
				},
			})
		}
	}

	return agents
}

// runRoundRobin runs all agents against each other
func runRoundRobin(agentDefs []AgentDefinition, gamesPerMatchup int) []TournamentResult {
	var results []TournamentResult

	totalMatchups := len(agentDefs) * (len(agentDefs) - 1) / 2
	currentMatchup := 0

	for i := 0; i < len(agentDefs); i++ {
		for j := i + 1; j < len(agentDefs); j++ {
			currentMatchup++
			fmt.Printf("\n[%d/%d] %s vs %s\n", currentMatchup, totalMatchups,
				agentDefs[i].Name, agentDefs[j].Name)
			fmt.Println(strings.Repeat("─", 60))

			result := runMatchup(agentDefs[i], agentDefs[j], gamesPerMatchup)
			results = append(results, result)

			fmt.Printf("Results: %s %d - %d %s\n",
				agentDefs[i].Name, result.Agent1Wins,
				result.Agent2Wins, agentDefs[j].Name)
		}
	}

	return results
}

// runMatchup runs a series of games between two agents
func runMatchup(def1, def2 AgentDefinition, numGames int) TournamentResult {
	agent1, err := agent.NewHybridAgent(def1.Name, def1.Config)
	if err != nil {
		fmt.Printf("Error creating %s: %v\n", def1.Name, err)
		os.Exit(1)
	}
	agent1.EnableMetrics()

	agent2, err := agent.NewHybridAgent(def2.Name, def2.Config)
	if err != nil {
		fmt.Printf("Error creating %s: %v\n", def2.Name, err)
		os.Exit(1)
	}
	agent2.EnableMetrics()

	// Run evaluation
	stats := training.EvaluateAgents(agent1, agent2, numGames)

	return TournamentResult{
		Agent1Name: def1.Name,
		Agent2Name: def2.Name,
		Agent1Wins: int(stats.TestWins),
		Agent2Wins: int(stats.BaselineWins),
		TotalGames: int(stats.TestGames + stats.BaselineGames),
	}
}

// calculateELORankings computes ELO ratings from tournament results
func calculateELORankings(agentDefs []AgentDefinition, results []TournamentResult) []AgentRanking {
	// Initialize ratings at 1500
	ratings := make(map[string]*AgentRanking)
	for _, def := range agentDefs {
		ratings[def.Name] = &AgentRanking{
			Name: def.Name,
			ELO:  1500.0,
		}
	}

	// K-factor for ELO calculation
	const K = 32.0

	// Process each result
	for _, result := range results {
		r1 := ratings[result.Agent1Name]
		r2 := ratings[result.Agent2Name]

		// Expected scores
		expected1 := 1.0 / (1.0 + math.Pow(10, (r2.ELO-r1.ELO)/400.0))
		expected2 := 1.0 / (1.0 + math.Pow(10, (r1.ELO-r2.ELO)/400.0))

		// Actual scores (win rate)
		total := float64(result.Agent1Wins + result.Agent2Wins)
		actual1 := 0.0
		actual2 := 0.0
		if total > 0 {
			actual1 = float64(result.Agent1Wins) / total
			actual2 = float64(result.Agent2Wins) / total
		}

		// Update ELO
		r1.ELO += K * (actual1 - expected1)
		r2.ELO += K * (actual2 - expected2)

		// Update win/loss records
		r1.Wins += result.Agent1Wins
		r1.Losses += result.Agent2Wins
		r1.TotalGames += result.TotalGames

		r2.Wins += result.Agent2Wins
		r2.Losses += result.Agent1Wins
		r2.TotalGames += result.TotalGames
	}

	// Convert to slice and sort by ELO
	var rankings []AgentRanking
	for _, r := range ratings {
		rankings = append(rankings, *r)
	}

	sort.Slice(rankings, func(i, j int) bool {
		return rankings[i].ELO > rankings[j].ELO
	})

	return rankings
}

// displayTournamentResults shows all matchup results
func displayTournamentResults(results []TournamentResult) {
	fmt.Println("\n╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║                    MATCHUP RESULTS                         ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝\n")

	for _, r := range results {
		winPct1 := 0.0
		winPct2 := 0.0
		total := float64(r.Agent1Wins + r.Agent2Wins)
		if total > 0 {
			winPct1 = float64(r.Agent1Wins) / total * 100
			winPct2 = float64(r.Agent2Wins) / total * 100
		}

		fmt.Printf("%-20s %3d (%.1f%%)  vs  %-20s %3d (%.1f%%)\n",
			r.Agent1Name, r.Agent1Wins, winPct1,
			r.Agent2Name, r.Agent2Wins, winPct2)
	}
}

// displayRankings shows the final ELO rankings
func displayRankings(rankings []AgentRanking) {
	fmt.Println("\n╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║                     FINAL RANKINGS                         ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝\n")

	fmt.Printf("%-4s %-20s %8s %6s %8s %8s\n",
		"Rank", "Agent", "ELO", "W-L", "Win%", "Games")
	fmt.Println(strings.Repeat("─", 60))

	for i, r := range rankings {
		winPct := 0.0
		if r.TotalGames > 0 {
			winPct = float64(r.Wins) / float64(r.Wins+r.Losses) * 100
		}

		fmt.Printf("%-4d %-20s %8.0f %3d-%-3d %7.1f%% %8d\n",
			i+1, r.Name, r.ELO, r.Wins, r.Losses, winPct, r.TotalGames)
	}

	fmt.Println()
}
