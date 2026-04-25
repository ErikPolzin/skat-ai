package server

import (
	"skat/agent"
	"skat/game"
	"skat/logger"
	"skat/rating"
	"time"
)

// Message represents a WebSocket message
type Message struct {
	Type string         `json:"type"`
	Data map[string]any `json:"data"`
}

// handleMessage processes incoming WebSocket messages
// Note: Most game actions are now handled via HTTP endpoints.
// This is kept for potential future use.
func (s *Server) handleMessage(client *Client, msg *Message) {
	logger.Debug("Received message", "type", msg.Type, "profile_id", client.profileID)

	// Add 2 second delay for local development to test loading states
	if !s.IsCloudRun() {
		time.Sleep(2 * time.Second)
	}

	switch msg.Type {
	default:
		logger.Warning("Unknown message type", "type", msg.Type)
	}
}

func (cm *ClientManager) BroadcastToPlayers(gs *game.GameState, msg *Message) {
	// Collect profile IDs of human players
	var profileIDs []string
	for _, player := range gs.Players {
		if player != nil && !player.IsAgent { // Only send to human players
			profileIDs = append(profileIDs, player.ID)
		}
	}
	cm.BroadcastToClients(profileIDs, msg)
}

func (cm *ClientManager) BroadcastStateChange(gs *game.GameState, msg string, fromPlayer game.GamePosition) {
	for _, player := range gs.Players {
		if player != nil && !player.IsAgent { // Only send to human players
			stateMsg := &Message{
				Type: "state_update",
				Data: map[string]any{
					"diff":        gs.SerializeForPlayer(player.ID),
					"description": msg,
					"from_player": fromPlayer,
				},
			}
			cm.SendToClient(player.ID, stateMsg)
		}
	}
}

// saveGameResults saves player results when a game completes
func (s *Server) maybeSaveGameResults(gs *game.GameState) {
	if gs.Phase == game.PhaseComplete {
		results := game.Results(gs)

		// Update player ratings and populate rating fields in results
		results, err := rating.UpdateRatings(gs, s.db, results)
		if err != nil {
			logger.Warning("Failed to update player ratings", "error", err)
		}

		// Save results with rating information
		if err := s.db.SavePlayerResults(results); err != nil {
			logger.Warning("Failed to save player results", "error", err)
		}
	}
}

func (s *Server) BroadcastAIActions(gs *game.GameState) {
	for {
		action := agent.NextAction(gs)
		if action == nil {
			break
		}
		currentPlayer := gs.CurrentPlayer

		// Wait longer if this is resolving a trick (3 cards on table)
		time.Sleep(1 * time.Second)

		response, err := action()
		if err != nil {
			logger.Error("Agent encountered an error", err)
			s.clients.BroadcastToPlayers(gs, &Message{
				Type: "error",
				Data: map[string]any{"message": err.Error()},
			})
			return
		}
		s.db.SaveGame(*gs)
		s.maybeSaveGameResults(gs)
		s.clients.BroadcastStateChange(gs, response, currentPlayer)
	}
}

