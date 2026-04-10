package main

import (
	"fmt"
	"os"
	"skat/agent"
	"skat/config"
	"skat/game"
	"skat/training"
)

func main() {
	fmt.Println("Full Skat AI - Bidding + MCTS Play")
	fmt.Println("===================================")
	fmt.Println()

	// Setup: 1 trained bidding agent + 2 heuristic bidders
	// All use MCTS for card play
	var biddingAgents [3]*agent.BiddingAgent

	// Load storage configuration
	cfg := config.LoadFromEnv()
	fmt.Printf("Storage backend: %s\n\n", cfg)

	// Try to load trained agent
	trainedAgent := agent.NewBiddingAgent()
	if err := cfg.LoadQTable(trainedAgent); err != nil {
		fmt.Println("⚠ No saved Q-table found. Training a new agent...")
		trainer := training.NewBiddingTrainer()
		trainer.TrainBidding(500)
		trainedAgent = trainer.GetBiddingAgent(0)
		cfg.SaveQTable(trainedAgent)
		fmt.Println("✓ Training complete and saved\n")
	} else {
		fmt.Println("✓ Loaded trained bidding agent\n")
	}

	trainedAgent.Epsilon = 0.0 // No exploration during play

	// Player 0: Trained RL bidding agent
	biddingAgents[0] = trainedAgent

	// Players 1-2: Heuristic bidding
	biddingAgents[1] = agent.NewBiddingAgent()
	biddingAgents[2] = agent.NewBiddingAgent()

	// All players use MCTS for card play
	mctsAgents := [3]*agent.MCTSAgent{
		agent.NewMCTSAgent("MCTS-0", 300),
		agent.NewMCTSAgent("MCTS-1", 300),
		agent.NewMCTSAgent("MCTS-2", 300),
	}

	// Play games
	numGames := 200
	stats := make([]PlayerStats, 3)

	fmt.Printf("Playing %d games...\n", numGames)
	fmt.Println()

	for gameNum := 0; gameNum < numGames; gameNum++ {
		g := game.NewGame()

		// BIDDING PHASE
		declarer, finalBid := conductBidding(g, biddingAgents)

		if declarer == -1 {
			// Everyone passed
			continue
		}

		stats[declarer].GamesAsDeclarer++

		// Declarer picks up skat
		g.Declarer = declarer
		g.Players[declarer].Hand = append(g.Players[declarer].Hand, g.Skat[:]...)
		discardCards(g, declarer)

		g.Mode = game.ModeGrand
		g.TrumpSuit = game.Clubs
		g.Phase = game.PhasePlaying
		g.CurrentPlayer = declarer

		// CARD PLAY PHASE (using MCTS)
		for g.Phase == game.PhasePlaying {
			validMoves := g.GetValidMoves()
			if len(validMoves) == 0 {
				break
			}
			move := mctsAgents[g.CurrentPlayer].SelectMove(g, validMoves)
			g.PlayCard(move)
		}

		// Record results
		won := g.DeclarerScore >= 61
		stats[declarer].TotalPointsScored += g.DeclarerScore

		if won {
			stats[declarer].Wins++
			stats[declarer].TotalGameValue += finalBid
		} else {
			stats[declarer].Losses++
			stats[declarer].TotalGameValue -= finalBid
		}

		// Progress update
		if (gameNum+1)%50 == 0 {
			fmt.Printf("Completed %d/%d games...\n", gameNum+1, numGames)
		}
	}

	// Display results
	fmt.Println()
	fmt.Println("=" + repeat("=", 70))
	fmt.Println("FINAL RESULTS")
	fmt.Println("=" + repeat("=", 70))
	fmt.Println()

	playerNames := []string{
		"Player 0 (Trained RL Bidding + MCTS Play)",
		"Player 1 (Heuristic Bidding + MCTS Play)",
		"Player 2 (Heuristic Bidding + MCTS Play)",
	}

	for i := 0; i < 3; i++ {
		fmt.Printf("%s\n", playerNames[i])
		fmt.Println(repeat("-", 70))

		if stats[i].GamesAsDeclarer == 0 {
			fmt.Println("  No games as declarer")
			fmt.Println()
			continue
		}

		winRate := float64(stats[i].Wins) / float64(stats[i].GamesAsDeclarer) * 100
		avgPoints := float64(stats[i].TotalPointsScored) / float64(stats[i].GamesAsDeclarer)

		fmt.Printf("  Games as declarer: %d\n", stats[i].GamesAsDeclarer)
		fmt.Printf("  Win rate:          %.1f%% (%d wins, %d losses)\n",
			winRate, stats[i].Wins, stats[i].Losses)
		fmt.Printf("  Avg points/game:   %.1f\n", avgPoints)
		fmt.Printf("  Total game value:  %+d\n", stats[i].TotalGameValue)
		fmt.Println()
	}

	// Compare Player 0 (trained) vs others
	if stats[0].GamesAsDeclarer > 0 {
		trainedWR := float64(stats[0].Wins) / float64(stats[0].GamesAsDeclarer) * 100

		othersWins := stats[1].Wins + stats[2].Wins
		othersGames := stats[1].GamesAsDeclarer + stats[2].GamesAsDeclarer
		othersWR := 0.0
		if othersGames > 0 {
			othersWR = float64(othersWins) / float64(othersGames) * 100
		}

		fmt.Println(repeat("=", 70))
		fmt.Printf("Trained Agent:    %.1f%% win rate\n", trainedWR)
		fmt.Printf("Heuristic Agents: %.1f%% win rate\n", othersWR)
		fmt.Printf("Improvement:      %+.1f percentage points\n", trainedWR-othersWR)
		fmt.Println(repeat("=", 70))
	}
}

type PlayerStats struct {
	GamesAsDeclarer   int
	Wins              int
	Losses            int
	TotalPointsScored int
	TotalGameValue    int
}

func conductBidding(g *game.GameState, agents [3]*agent.BiddingAgent) (game.GamePosition, int) {
	// Simplified 3-player bidding
	currentBid := 17
	lastBidder := -1

	// Each player bids once in sequence
	for round := 0; round < 3; round++ {
		for p := 0; p < 3; p++ {
			bid := agents[p].Bid(g, currentBid)
			if bid > currentBid {
				currentBid = bid
				lastBidder = p
			}
		}
	}

	if lastBidder == -1 {
		return -1, 0 // Everyone passed
	}

	return game.GamePosition(lastBidder), currentBid
}

func discardCards(g *game.GameState, declarer game.GamePosition) {
	player := g.Players[declarer]

	type cv struct {
		card  game.Card
		value int
	}

	cards := make([]cv, len(player.Hand))
	for i, card := range player.Hand {
		val := card.Value()
		if card.Rank == game.Jack {
			val = 100
		}
		cards[i] = cv{card, val}
	}

	// Sort by value
	for i := 0; i < len(cards)-1; i++ {
		for j := i + 1; j < len(cards); j++ {
			if cards[i].value > cards[j].value {
				cards[i], cards[j] = cards[j], cards[i]
			}
		}
	}

	discard1 := cards[0].card
	discard2 := cards[1].card

	newHand := make([]game.Card, 0, 10)
	discarded := 0
	for _, card := range player.Hand {
		if discarded < 2 && (card == discard1 || card == discard2) {
			discarded++
			continue
		}
		newHand = append(newHand, card)
	}
	player.Hand = newHand
}

func repeat(s string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}

func init() {
	// Check if we have a saved Q-table, if not offer to train
	if _, err := os.Stat("bidding_qtable.json"); os.IsNotExist(err) {
		fmt.Println("Note: No saved bidding agent found.")
		fmt.Println("This will train a new agent on first run (takes ~1 minute)")
		fmt.Println()
	}
}
