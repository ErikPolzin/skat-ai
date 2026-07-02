package agent

import (
	"skat/game"
	"skat/logger"
)

type Action func() (string, error)

// ErrorAction returns an action whose response contains the error message.
func ErrorAction(err error) Action {
	return func() (string, error) {
		return err.Error(), err
	}
}

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
					agent, loadErr := GetAgentForPlayer(gs.Players[i])
					if loadErr != nil {
						return loadErr.Error(), loadErr
					}
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
					agent, loadErr := GetAgentForPlayer(gs.Players[i])
					if loadErr != nil {
						return loadErr.Error(), loadErr
					}
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
	agentInstance, err := GetAgentForPlayer(player)
	if err != nil {
		return ErrorAction(err)
	}
	// Check if agent has already picked up skat
	if len(player.Hand) == 12 {
		// Agent has picked up skat. Choose the game and its discard together so
		// declaration cannot switch to a contract the discard was not built for.

		// Get shared agent instance for game mode decision
		choice := agentInstance.ChooseGameAndSkatDiscard(gs)

		return func() (string, error) {
			if _, err := gs.Discard(choice.Discard[0], choice.Discard[1]); err != nil {
				return "", err
			}
			return gs.DeclareGame(choice.Mode, choice.TrumpSuit, false, false)
		}
	} else {
		choice := agentInstance.ChooseGame(gs)
		if choice.PlayedHand {
			return func() (string, error) {
				if _, err := gs.SkatDecision(false); err != nil {
					return "", err
				}
				return gs.DeclareGame(choice.Mode, choice.TrumpSuit, choice.AnnouncedSchneider, choice.AnnouncedSchwarz)
			}
		}
		return func() (string, error) {
			return gs.SkatDecision(true)
		}
	}
}

// processAgentDeclaration handles an AI agent's declaration
func generateAgentDeclarationAction(gs *game.GameState, player *game.PlayerState) Action {
	// Get shared agent instance for Q-learning game choice
	agentInstance, err := GetAgentForPlayer(player)
	if err != nil {
		return ErrorAction(err)
	}

	// Use Q-learning to choose the best game mode and trump suit
	choice := agentInstance.ChooseGame(gs)

	return func() (string, error) {
		return gs.DeclareGame(choice.Mode, choice.TrumpSuit, choice.AnnouncedSchneider, choice.AnnouncedSchwarz)
	}
}

func generateAgentPlayAction(gs *game.GameState, player *game.PlayerState) Action {
	validMoves := gs.GetValidMoves()

	if len(validMoves) == 0 {
		logger.Warning("No valid moves for AI %s", player.Name)
		return nil
	}

	// Get the agent for this player
	agent, err := GetAgentForPlayer(player)
	if err != nil {
		return ErrorAction(err)
	}
	move := agent.SelectMove(gs, validMoves)

	return func() (string, error) {
		return gs.PlayCard(move)
	}
}

func generateAgentBidAction(gs *game.GameState, player *game.PlayerState) Action {
	// Get the agent for this player
	agent, err := GetAgentForPlayer(player)
	if err != nil {
		return ErrorAction(err)
	}
	// Get a copy of the game state for the agent
	stateCopy := gs // Make a copy
	// Call the agent's Bid method
	accept := agent.Bid(stateCopy)

	return func() (string, error) {
		return gs.Bid(accept)
	}
}
