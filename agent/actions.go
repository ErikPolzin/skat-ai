package agent

import (
	"context"
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
				logger.Warning("Agent will use untrained behavior with heuristics")
			} else {
				logger.Info("Loaded Q-table from GCS", "states", sharedAgent.GetQTableSize())
			}
			// Always disable exploration in production
			sharedAgent.Epsilon = 0.0
		} else {
			// Load from local file (development)
			qtablePath := "bidding_qtable.gob"
			if path := os.Getenv("QTABLE_BINARY_PATH"); path != "" {
				qtablePath = path
			}
			logger.Info("Loading Q-table from local file", "path", qtablePath)
			if err := sharedAgent.LoadQTableBinary(qtablePath); err != nil {
				logger.Error("Could not load Q-table from local file", "path", qtablePath, "error", err)
				logger.Warning("Agent will use untrained behavior with heuristics")
			} else {
				logger.Info("Loaded Q-table from local file", "path", qtablePath, "states", sharedAgent.GetQTableSize())
			}
			// Always disable exploration in production
			sharedAgent.Epsilon = 0.0
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
	case game.PhaseSkatExchange:
		return generateAgentSkatExchangeAction(gs, currentPlayer)
	case game.PhaseDeclarerChoice:
		return generateAgentDeclarationAction(gs, currentPlayer)
	case game.PhasePlaying:
		return generateAgentPlayAction(gs, currentPlayer)
	default:
		logger.Error("Unknown agent game phase", phase)
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

func generateAgentSkatExchangeAction(gs *game.GameState, player *game.PlayerState) Action {
	// Check if agent has already picked up skat
	if len(player.Hand) == 12 {
		// Agent has picked up skat, needs to discard 2 cards
		// Simple strategy: discard 2 lowest value non-jack cards
		type cardValue struct {
			card  game.Card
			value int
		}

		cards := make([]cardValue, len(player.Hand))
		for i, card := range player.Hand {
			val := card.Value()
			if card.Rank == game.Jack {
				val = 100 // Never discard jacks
			}
			cards[i] = cardValue{card, val}
		}

		// Sort by value
		for i := 0; i < len(cards)-1; i++ {
			for j := i + 1; j < len(cards); j++ {
				if cards[i].value > cards[j].value {
					cards[i], cards[j] = cards[j], cards[i]
				}
			}
		}

		return func() (string, error) {
			return gs.Discard(player.ID, cards[0].card, cards[1].card)
		}
	} else {
		// Agent hasn't picked up skat yet - always pick it up
		return func() (string, error) {
			return gs.SkatDecision(player.ID, true)
		}
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
	// Get the agent for this player
	agent := getAgentForPlayer(player)
	if agent == nil {
		logger.Warning("No agent found for player", "player", player.Name)
		return nil
	}
	// Get a copy of the game state for the agent
	stateCopy := gs // Make a copy
	// Call the agent's Bid method
	accept := agent.Bid(stateCopy)
	// Log debug info if we have a SkatAgent
	if skatAgent, ok := agent.(*SkatAgent); ok {
		logger.Debug("Agent bid decision", "player", player.Name, "handScore", skatAgent.CurrentHandScore, "bidDecision", accept, "currentBid", gs.BidValue)
	}
	logger.Debug("AI choosing bid", "player", player.Name, "accept", accept)

	return func() (string, error) {
		return gs.Bid(player.ID, accept)
	}
}
