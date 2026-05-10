package agent

import (
	"skat/game"
	"skat/logger"
)

type Action func() (string, error)

// gameLoop manages the game flow
func NextAction(gs *game.GameState) Action {
	if gs.Phase == game.PhaseComplete {
		return nil
	}

	phase := gs.Phase
	currentPlayer := gs.GetCurrentPlayer()

	if currentPlayer == nil {
		return nil
	}

	if len(gs.Trick) == 3 && phase == game.PhasePlaying {
		return generateResolveTrickAction(gs)
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
		logger.Error("Unknown agent game phase %s", phase)
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
					agent := GetAgentForPlayer(gs.Players[i])
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
					agent := GetAgentForPlayer(gs.Players[i])
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
		agentInstance := GetAgentForPlayer(player)

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
	agentInstance := GetAgentForPlayer(player)

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
		logger.Warning("No valid moves for AI %s", player.Name)
		return nil
	}

	// Get the agent for this player
	agent := GetAgentForPlayer(player)
	if agent == nil {
		logger.Warning("No agent found for player %s", player.Name)
		return nil
	}
	move := agent.SelectMove(gs, validMoves)

	return func() (string, error) {
		return gs.PlayCard(move)
	}
}

func generateAgentBidAction(gs *game.GameState, player *game.PlayerState) Action {
	// Get the agent for this player
	agent := GetAgentForPlayer(player)
	if agent == nil {
		logger.Warning("No agent found for player %s", player.Name)
		return nil
	}
	// Get a copy of the game state for the agent
	stateCopy := gs // Make a copy
	// Call the agent's Bid method
	accept := agent.Bid(stateCopy)

	return func() (string, error) {
		return gs.Bid(accept)
	}
}
