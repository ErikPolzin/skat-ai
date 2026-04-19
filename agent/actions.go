package agent

import (
	"context"
	"fmt"
	"os"
	"skat/game"
	"skat/logger"
	"sync"
)

type Action func() (string, error)

var (
	// Shared agent instance with loaded Q-table
	sharedAgent     *SkatAgent
	sharedAgentOnce sync.Once
)

// getAgentForPlayer creates an agent instance based on the player's agent type
func getAgentForPlayer(player *game.PlayerState) Agent {
	if !player.IsAgent {
		return nil
	}

	// Initialize shared agent once
	sharedAgentOnce.Do(func() {
		sharedAgent = NewSkatAgent("SkatAgent", 500)

		// Log current working directory for debugging
		if cwd, err := os.Getwd(); err == nil {
			logger.Debug("Agent loading Q-table from working directory", "cwd", cwd)
		}

		// Try to load from GCS first (production), then local file (development)
		gcsBucket := os.Getenv("GCS_BUCKET")
		gcsPath := os.Getenv("GCS_QTABLE_PATH")

		if gcsBucket != "" && gcsPath != "" {
			// Load from GCS (production)
			logger.Info("Loading Q-table from GCS", "bucket", gcsBucket, "path", gcsPath)
			ctx := context.Background()
			if err := sharedAgent.LoadQTableFromGCS(ctx, gcsBucket, gcsPath, true); err != nil {
				logger.Error("Could not load Q-table from GCS", "error", err)
				logger.Warning("Agent will use untrained behavior (will pass on most hands)")
			} else {
				logger.Info("Loaded Q-table from GCS", "states", sharedAgent.GetQTableSize())
				sharedAgent.Epsilon = 0.0
			}
		} else {
			// Load from local file (development)
			qtablePath := "bidding_qtable.gob"
			if path := os.Getenv("QTABLE_BINARY_PATH"); path != "" {
				qtablePath = path
			}
			logger.Info("Loading Q-table from local file", "path", qtablePath)
			if err := sharedAgent.LoadQTableBinary(qtablePath); err != nil {
				logger.Error("Could not load Q-table from local file", "path", qtablePath, "error", err)
				logger.Warning("Agent will use untrained behavior (will pass on most hands)")
			} else {
				logger.Info("Loaded Q-table from local file", "path", qtablePath, "states", sharedAgent.GetQTableSize())
				sharedAgent.Epsilon = 0.0
			}
		}
	})

	return sharedAgent
}

// gameLoop manages the game flow
func NextAction(gs *game.GameState) Action {
	if gs.Phase == game.PhaseComplete {
		return nil
	}

	phase := gs.Phase
	currentPlayer := gs.GetCurrentPlayer()

	if len(gs.Trick) == 3 && phase == game.PhasePlaying {
		return generateResolveTrickAction(gs)
	}

	if currentPlayer != nil {
		logger.Debug("Game loop", "phase", phase, "currentPlayer", currentPlayer.Name, "position", gs.CurrentPlayer, "isAgent", currentPlayer.IsAgent)
	} else {
		logger.Debug("Game loop", "phase", phase, "currentPlayer", "nil")
		return nil
	}
	if !currentPlayer.IsAgent {
		logger.Debug("Skipping game loop: waiting for human player input")
		return nil
	}

	switch phase {
	case game.PhaseDealing:
		return generateAgentDealAction(gs, currentPlayer)
	case game.PhaseBidding:
		return generateAgentBidAction(gs, currentPlayer)
	case game.PhaseDeclarerChoice:
		return generateAgentDeclarationAction(gs, currentPlayer)
	case game.PhasePlaying:
		return generateAgentPlayAction(gs, currentPlayer)
	default:
		return nil
	}
}

func generateAgentDealAction(gs *game.GameState, player *game.PlayerState) Action {
	return func() (string, error) {
		return gs.Deal(player.ID)
	}
}

func generateResolveTrickAction(gs *game.GameState) Action {
	return func() (string, error) {
		return gs.ResolveTrick()
	}
}

// processAgentDeclaration handles an AI agent's declaration
func generateAgentDeclarationAction(gs *game.GameState, player *game.PlayerState) Action {
	// AI always picks suit game with trump based on strongest suit
	// Count cards per suit in agent's hand
	suitCounts := make(map[game.Suit]int)
	for _, card := range player.Hand {
		if card.Rank != game.Jack { // Jacks are trump in all games
			suitCounts[card.Suit]++
		}
	}
	// Find suit with most cards
	maxCount := 0
	var trump game.Suit
	for suit, count := range suitCounts {
		if count > maxCount {
			maxCount = count
			trump = suit
		}
	}
	// Default to clubs if no clear winner
	if maxCount == 0 {
		trump = game.Clubs
	}
	return func() (string, error) {
		return gs.DeclareGame(player.ID, game.ModeSuit, trump)
	}
}

func generateAgentPlayAction(gs *game.GameState, player *game.PlayerState) Action {
	validMoves := gs.GetValidMoves()

	if len(validMoves) == 0 {
		logger.Warning("No valid moves for AI", "player", player.Name)
		return nil
	}

	// Get the agent for this player
	agent := getAgentForPlayer(player)
	if agent == nil {
		logger.Warning("No agent found for player", "player", player.Name)
		return nil
	}
	move := agent.SelectMove(gs, validMoves)

	return func() (string, error) {
		return gs.PlayCard(player.ID, move)
	}
}

func generateAgentBidAction(gs *game.GameState, player *game.PlayerState) Action {
	validBids := gs.GetValidBids()

	if len(validBids) == 0 {
		logger.Warning("No valid bids for AI", "player", player.Name)
		return nil
	}

	// Use the agent's Bid method for intelligent bidding
	var action string

	if player.IsAgent {
		// Get a copy of the game state for the agent
		currentBid := gs.BidValue
		stateCopy := gs // Make a copy

		// Get the agent for this player
		agent := getAgentForPlayer(player)
		if agent == nil {
			logger.Warning("No agent found for player", "player", player.Name)
			return nil
		}

		// Call the agent's Bid method
		agentBid := agent.Bid(stateCopy, currentBid)

		if agentBid == 0 {
			// Agent wants to pass
			action = "pass"
		} else {
			// Agent wants to bid or hold
			// Check if the agent's bid matches any valid action
			bidStr := fmt.Sprintf("%d", agentBid)

			// Check if this bid value is in valid bids
			for _, validBid := range validBids {
				if validBid == bidStr {
					action = bidStr
					break
				} else if validBid == "hold" && agentBid == currentBid {
					// Agent wants to hold at current bid
					action = "hold"
					break
				}
			}

			// If agent's bid isn't valid, default to pass
			if action == "" {
				action = "pass"
			}
		}
	} else {
		// Fallback if no agent (shouldn't happen)
		action = "pass"
	}

	logger.Debug("AI choosing bid", "player", player.Name, "action", action)

	return func() (string, error) {
		return gs.Bid(player.ID, action)
	}
}
