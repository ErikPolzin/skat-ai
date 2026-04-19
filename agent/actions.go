package agent

import (
	"fmt"
	"log"
	"skat/game"
)

type Action func() (string, error)

// getAgentForPlayer creates an agent instance based on the player's agent type
func getAgentForPlayer(player *game.PlayerState) Agent {
	if !player.IsAgent {
		return nil
	}

	// Currently only MCTS agent is supported for full gameplay
	// BiddingAgent only implements Bid(), not the full Agent interface
	return NewMCTSAgent(player.Name, 500)
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
		log.Printf("Game loop: phase=%v, currentPlayer=%s (position=%d), isAgent=%v",
			phase, currentPlayer.Name, gs.CurrentPlayer, currentPlayer.IsAgent)
	} else {
		log.Printf("Game loop: phase=%v, currentPlayer=nil", phase)
		return nil
	}
	if !currentPlayer.IsAgent {
		log.Printf("Skipping game loop: waiting for human player input")
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
		log.Printf("No valid moves for AI %s", player.Name)
		return nil
	}

	// Get the agent for this player
	agent := getAgentForPlayer(player)
	if agent == nil {
		log.Printf("No agent found for player %s", player.Name)
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
		log.Printf("No valid bids for AI %s", player.Name)
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
			log.Printf("No agent found for player %s", player.Name)
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

	log.Printf("AI %s choosing to %s", player.Name, action)

	return func() (string, error) {
		return gs.Bid(player.ID, action)
	}
}
