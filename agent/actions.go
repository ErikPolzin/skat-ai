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
func getAgentForPlayer(player *game.PlayerState) *SkatAgent {
	if !player.IsAgent {
		return nil
	}

	// Initialize shared agent once
	sharedAgentOnce.Do(func() {
		sharedAgent = NewHeuristicAgent("SkatAgent")

		// Log current working directory for debugging
		if cwd, err := os.Getwd(); err == nil {
			logger.Debug("Agent loading Q-table from working directory", "cwd", cwd)
		}

		// Try to load from GCS first (production), then local file (development)
		gcsBucket := os.Getenv("GCS_BUCKET")
		gcsPath := os.Getenv("GCS_QTABLE_PATH")
		if gcsPath == "" {
			gcsPath = "qtables/bidding_qtable.gob"
		}

		if gcsBucket != "" {
			// Load from GCS (production)
			logger.Info("Loading bidding Q-table from GCS", "bucket", gcsBucket, "path", gcsPath)
			ctx := context.Background()
			if qStrat, ok := sharedAgent.GetBiddingStrategy().(*QLearningBiddingStrategy); ok {
				data, err := LoadQTableDataFromGCS(ctx, gcsBucket, gcsPath, true)
				if err != nil {
					logger.Error("Could not load bidding Q-table from GCS", "error", err)
					logger.Warning("Agent will use untrained bidding behavior with heuristics")
				} else {
					qStrat.SetQTable(data.QTable)
					qStrat.SetEpsilon(0.0) // Disable exploration in production
					logger.Info("Loaded bidding Q-table from GCS", "states", qStrat.GetQTableSize())
				}
			}

			// Load game choice Q-table from GCS
			gameChoicePath := os.Getenv("GCS_GAME_CHOICE_PATH")
			if gameChoicePath == "" {
				gameChoicePath = "qtables/game_choice_qtable.gob"
			}
			logger.Info("Loading game choice Q-table from GCS", "bucket", gcsBucket, "path", gameChoicePath)
			if qStrat, ok := sharedAgent.GetGameChoiceStrategy().(*QLearningGameChoiceStrategy); ok {
				data, err := LoadQTableDataFromGCS(ctx, gcsBucket, gameChoicePath, true)
				if err != nil {
					logger.Error("Could not load game choice Q-table from GCS", "error", err)
					logger.Warning("Agent will use heuristic game choice")
				} else {
					qStrat.SetQTable(data.QTable)
					qStrat.SetEpsilon(0.0) // Disable exploration in production
					logger.Info("Loaded game choice Q-table from GCS")
				}
			}
		} else {
			// Load from local file (development)
			qtablePath := "bidding_qtable.gob"
			if path := os.Getenv("QTABLE_BINARY_PATH"); path != "" {
				qtablePath = path
			}
			logger.Info("Loading bidding Q-table from local file", "path", qtablePath)
			if qStrat, ok := sharedAgent.GetBiddingStrategy().(*QLearningBiddingStrategy); ok {
				data, err := LoadQTableData(qtablePath, true)
				if err != nil {
					logger.Error("Could not load bidding Q-table from local file", "path", qtablePath, "error", err)
					logger.Warning("Agent will use untrained bidding behavior with heuristics")
				} else {
					qStrat.SetQTable(data.QTable)
					qStrat.SetEpsilon(0.0) // Disable exploration in production
					logger.Info("Loaded bidding Q-table from local file", "path", qtablePath, "states", qStrat.GetQTableSize())
				}
			}

			// Load game choice Q-table from local file
			gameChoicePath := "game_choice_qtable.gob"
			if path := os.Getenv("GAME_CHOICE_QTABLE_PATH"); path != "" {
				gameChoicePath = path
			}
			logger.Info("Loading game choice Q-table from local file", "path", gameChoicePath)
			if qStrat, ok := sharedAgent.GetGameChoiceStrategy().(*QLearningGameChoiceStrategy); ok {
				data, err := LoadQTableData(gameChoicePath, true)
				if err != nil {
					logger.Error("Could not load game choice Q-table from local file", "path", gameChoicePath, "error", err)
					logger.Warning("Agent will use heuristic game choice")
				} else {
					qStrat.SetQTable(data.QTable)
					qStrat.SetEpsilon(0.0) // Disable exploration in production
					logger.Info("Loaded game choice Q-table from local file")
				}
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
		result, err := gs.Deal()

		// Reset card tracking for all agents at start of new game
		if err == nil {
			for i := range gs.Players {
				if gs.Players[i].IsAgent {
					agent := getAgentForPlayer(gs.Players[i])
					if agent != nil {
						agent.OnGameStart()
					}
				}
			}
		}

		return result, err
	}
}

func generateResolveTrickAction(gs *game.GameState) Action {
	return func() (string, error) {
		// Track the trick before it gets cleared
		trick := make([]game.Card, len(gs.Trick))
		copy(trick, gs.Trick)

		result, err := gs.ResolveTrick()

		// Notify all agents that a trick was completed (for card tracking)
		if err == nil && len(trick) == 3 {
			for i := range gs.Players {
				if gs.Players[i].IsAgent {
					agent := getAgentForPlayer(gs.Players[i])
					if agent != nil {
						agent.OnTrickComplete(trick)
					}
				}
			}
		}

		return result, err
	}
}

func generateAgentSkatExchangeAction(gs *game.GameState, player *game.PlayerState) Action {
	// Check if agent has already picked up skat
	if len(player.Hand) == 12 {
		// Agent has picked up skat, needs to discard 2 cards
		// Use game-aware discard strategy: pre-decide game mode, then discard optimally

		// Get shared agent instance for game mode decision
		agentInstance := getAgentForPlayer(player)

		// Agent will use Q-learning to choose game mode
		mode, trumpSuit := agentInstance.ChooseGame(gs)

		// Now choose optimal discard for that game mode
		card1, card2 := agentInstance.ChooseSkatDiscard(player.Hand, mode, trumpSuit)

		return func() (string, error) {
			return gs.Discard(card1, card2)
		}
	} else {
		// Agent hasn't picked up skat yet - always pick it up
		return func() (string, error) {
			return gs.SkatDecision(true)
		}
	}
}

// processAgentDeclaration handles an AI agent's declaration
func generateAgentDeclarationAction(gs *game.GameState, player *game.PlayerState) Action {
	// Get shared agent instance for Q-learning game choice
	agentInstance := getAgentForPlayer(player)

	// Use Q-learning to choose the best game mode and trump suit
	mode, trumpSuit := agentInstance.ChooseGame(gs)

	// AI agents don't announce schneider/schwarz for now (conservative strategy)
	// TODO: In the future, could add a strategy for when to make announcements
	announceSchneider := false
	announceSchwarz := false

	return func() (string, error) {
		return gs.DeclareGame(mode, trumpSuit, announceSchneider, announceSchwarz)
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
		return gs.PlayCard(move)
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
	logger.Debug("AI choosing bid", "player", player.Name, "accept", accept, "currentBid", gs.BidValue)

	return func() (string, error) {
		return gs.Bid(accept)
	}
}
