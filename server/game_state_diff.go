package server

import (
	"skat/game"
	"time"
)

// GameStateDiff represents a change in the game state
type GameStateDiff struct {
	Type        string                 `json:"type"`                  // The type of change (e.g., "state_update")
	Timestamp   int64                  `json:"timestamp"`             // Unix timestamp of when the change occurred
	Changes     map[string]interface{} `json:"changes"`               // The actual changes to the state
	PlayerID    string                 `json:"player_id,omitempty"`   // The player who triggered the change (if applicable)
	PlayerName  string                 `json:"player_name,omitempty"` // The name of the player who triggered the change
	Description string                 `json:"description"`           // Human-readable description of what happened
}

// GameStateBroadcast encapsulates the full state and diff for efficient broadcasting
type GameStateBroadcast struct {
	FullState *GameInfo      `json:"full_state,omitempty"` // Full state for new/reconnecting clients
	Diff      *GameStateDiff `json:"diff"`                 // Incremental changes for existing clients
}

// createStateDiff creates a diff from the current game state after an action
func (r *GameSession) createStateDiff(actionType string, playerID string, playerName string, description string, changes map[string]interface{}) *GameStateDiff {
	return &GameStateDiff{
		Type:        actionType,
		Timestamp:   time.Now().Unix(),
		Changes:     changes,
		PlayerID:    playerID,
		PlayerName:  playerName,
		Description: description,
	}
}

// broadcastStateDiff sends a state diff to all connected players
func (r *GameSession) broadcastStateDiff(diff *GameStateDiff) {
	msg := &Message{
		Type: "state_update",
		Data: map[string]interface{}{
			"diff": diff,
		},
	}
	r.broadcast(msg)
}

// broadcastStateChange creates and broadcasts a state diff based on the action
func (r *GameSession) broadcastStateChange(actionType string, playerID string, additionalData map[string]interface{}) {
	changes := make(map[string]interface{})

	// Get player name if playerID is provided
	playerName := ""
	if playerID != "" {
		if player, exists := r.Players[playerID]; exists {
			playerName = player.Name
		}
	}

	// Generate human-readable description
	description := r.generateActionDescription(actionType, playerName, additionalData)

	// Add common state fields that might have changed (if game started)
	if r.GameState != nil {
		changes["current_player"] = r.GameState.CurrentPlayer
		changes["phase"] = phaseToString(r.GameState.Phase)
	}

	// Add action-specific changes
	switch actionType {
	case "player_joined", "player_left", "player_reconnected", "player_disconnected":
		// Always update players list for these events
		changes["players"] = r.GetPlayersInfo()

	case "game_started":
		changes["phase"] = phaseToString(r.GameState.Phase)
		changes["current_player"] = r.GameState.CurrentPlayer
		// Each player gets their own hand separately

	case "bid_made":
		changes["bid_value"] = r.GameState.BidValue
		changes["current_player"] = r.GameState.CurrentPlayer
		if val, ok := additionalData["action"]; ok {
			changes["last_bid_action"] = val
		}

	case "bidding_complete":
		changes["declarer"] = r.GameState.Declarer
		changes["phase"] = phaseToString(r.GameState.Phase)
		changes["bid_value"] = r.GameState.BidValue

	case "skat_available":
		// Only sent to declarer, includes skat cards for preview
		if val, ok := additionalData["skat"]; ok {
			changes["skat_cards"] = val
		}
		changes["phase"] = phaseToString(r.GameState.Phase)

	case "skat_picked_up":
		// Declarer picked up skat
		changes["has_picked_up_skat"] = true

	case "playing_hand":
		// Declarer chose to play hand without skat
		changes["phase"] = phaseToString(r.GameState.Phase)

	case "cards_discarded":
		// Cards discarded back to skat
		changes["phase"] = phaseToString(r.GameState.Phase)

	case "game_declared":
		changes["game_mode"] = getGameModeString(r.GameState.Mode)
		if r.GameState.Mode == game.ModeSuit {
			changes["trump_suit"] = r.GameState.TrumpSuit.String()
		}
		changes["phase"] = phaseToString(r.GameState.Phase)

	case "card_played":
		changes["players"] = r.GetPlayersInfo()
		changes["trick"] = cardsToJSON(r.GameState.Trick)
		changes["current_player"] = r.GameState.CurrentPlayer
		if val, ok := additionalData["card"]; ok {
			changes["last_card_played"] = val
		}

	case "trick_complete":
		changes["players"] = r.GetPlayersInfo()
		changes["trick_winner"] = r.GameState.TrickWinner
		changes["trick"] = []map[string]interface{}{} // Empty trick
		changes["current_player"] = r.GameState.CurrentPlayer
		changes["declarer_score"] = r.GameState.DeclarerScore

	case "game_complete":
		changes["phase"] = "complete"
		changes["declarer_score"] = r.GameState.DeclarerScore
		changes["game_over"] = true
		if val, ok := additionalData["declarer_won"]; ok {
			changes["declarer_won"] = val
		}

	case "cards_dealt":
		// Dealer has dealt the cards, update player info with card counts
		changes["players"] = r.GetPlayersInfo()
		changes["phase"] = phaseToString(r.GameState.Phase)
		changes["current_player"] = r.GameState.CurrentPlayer
	}

	// Merge additional data
	for k, v := range additionalData {
		if _, exists := changes[k]; !exists {
			changes[k] = v
		}
	}

	diff := r.createStateDiff(actionType, playerID, playerName, description, changes)
	r.broadcastStateDiff(diff)
}

// getGameModeString converts GameMode to string
func getGameModeString(mode game.GameMode) string {
	switch mode {
	case game.ModeGrand:
		return "grand"
	case game.ModeSuit:
		return "suit"
	case game.ModeNull:
		return "null"
	default:
		return "unknown"
	}
}

// sendPlayerSpecificUpdate sends a state update with player-specific data (like hand)
func (r *GameSession) sendPlayerSpecificUpdate(player *Player, actionType string, additionalData map[string]interface{}) {
	// Skip AI players
	if player.Agent != nil {
		return
	}

	changes := make(map[string]interface{})

	// Add player's hand if game is active
	if r.GameState != nil && player.Position >= 0 && player.Position < 3 {
		changes["hand"] = cardsToJSON(r.GameState.Players[player.Position].Hand)
	}

	// Merge additional data
	for k, v := range additionalData {
		changes[k] = v
	}

	description := r.generateActionDescription(actionType, player.Name, additionalData)
	diff := r.createStateDiff(actionType, player.ID, player.Name, description, changes)

	// Use ClientManager to send message
	if r.server != nil && r.server.clients != nil {
		r.server.clients.SendToClient(player.ID, &Message{
			Type: "state_update",
			Data: map[string]interface{}{
				"diff": diff,
			},
		})
	}
}

// generateActionDescription creates a human-readable description of the action
func (r *GameSession) generateActionDescription(actionType string, playerName string, data map[string]interface{}) string {
	switch actionType {
	case "player_joined":
		return playerName + " joined the game"
	case "player_left":
		return playerName + " left the game"
	case "player_reconnected":
		return playerName + " reconnected"
	case "player_disconnected":
		return playerName + " disconnected"
	case "game_started":
		return "Game started"
	case "bid_made":
		if action, ok := data["action"].(string); ok {
			if action == "pass" {
				return "Pass"
			} else if action == "hold" {
				return "Hold"
			} else {
				return "Bid " + action
			}
		}
		return "Bid"
	case "bidding_complete":
		return "Bidding complete - " + playerName + " is the declarer"
	case "skat_picked":
		return "Picked up skat"
	case "game_declared":
		mode := ""
		if m, ok := data["mode"].(string); ok {
			mode = m
		}
		trump := ""
		if t, ok := data["trump"].(string); ok && t != "" {
			trump = " (" + t + ")"
		}
		return mode + trump
	case "card_played":
		if card, ok := data["card"].(map[string]interface{}); ok {
			rank := card["rank"]
			suit := card["suit"]
			return rank.(string) + " of " + suit.(string)
		}
		return "Played"
	case "trick_complete":
		if winner, ok := data["winner"].(game.GamePosition); ok {
			if winnerPlayer := r.getPlayerByPosition(winner); winnerPlayer != nil {
				return winnerPlayer.Name + " won the trick"
			}
		}
		return "Trick complete"
	case "game_complete":
		if won, ok := data["declarer_won"].(bool); ok {
			if won {
				return "Game over - Declarer won!"
			} else {
				return "Game over - Defenders won!"
			}
		}
		return "Game complete"
	case "cards_dealt":
		return "Dealt"
	default:
		return ""
	}
}
