package server

import (
	"fmt"
	"skat/agent"
	"skat/game"
	"skat/game/rating"
	"skat/logger"
	"skat/server/db"
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
	switch msg.Type {
	default:
		logger.Warning("Unknown message type %s", msg.Type)
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
	// Fetch formatted session results if game just completed
	var sessionResults []game.SessionGameResult
	var gamesPlayed int
	if gs.Phase == game.PhaseComplete && gs.SessionID != "" {
		results, err := cm.db.GetFormattedSessionResults(gs.SessionID)
		if err != nil {
			logger.Warning("Failed to fetch session results for broadcast: %e", err)
		} else {
			sessionResults = results
			gamesPlayed = len(results)
		}
	}

	for _, player := range gs.Players {
		if player != nil && !player.IsAgent { // Only send to human players
			msgData := map[string]any{
				"diff":        gs.SerializeForPlayer(player.ID),
				"description": msg,
				"from_player": fromPlayer,
			}

			// Include session results if available
			if sessionResults != nil {
				msgData["session_results"] = sessionResults
				msgData["games_played"] = gamesPlayed
			}

			stateMsg := &Message{
				Type: "state_update",
				Data: msgData,
			}
			cm.SendToClient(player.ID, stateMsg)
		}
	}
}

// saveGameResults saves player results when a game completes
func (s *Server) maybeSaveGameResults(gs *game.GameState) error {
	if gs.Phase == game.PhaseComplete {
		results := gs.PlayerResults()
		if results == nil {
			logger.Warning("Failed to update player ratings, no results")
			return nil
		}

		playerRatings := make(map[string]*rating.PlayerRating)

		for _, player := range gs.Players {
			if player != nil {
				rating, err := s.db.GetPlayerRating(player.ID)
				if err != nil {
					return fmt.Errorf("failed to get player rating: %w", err)
				}
				playerRatings[player.ID] = rating.ToGamePlayerRating()
			}
		}

		// Update player ratings and populate rating fields in results
		err := rating.UpdateRatings(gs, results, playerRatings)
		if err != nil {
			logger.Warning("Failed to update player ratings: %e", err)
		}

		for _, rat := range playerRatings {
			if err := s.db.SavePlayerRating(db.NewPlayerRating(rat)); err != nil {
				return fmt.Errorf("failed to save player rating: %w", err)
			}
		}

		// Save results with rating information
		if err := s.db.SavePlayerResults(results[:]); err != nil {
			logger.Warning("Failed to save player results: %e", err)
		}
	}
	return nil
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
			logger.Error("Agent encountered an error: %e", err)
			s.clients.BroadcastToPlayers(gs, &Message{
				Type: "error",
				Data: map[string]any{"message": err.Error()},
			})
			return
		}
		s.cache.SaveGame(*gs)
		s.maybeSaveGameResults(gs)
		s.clients.BroadcastStateChange(gs, response, currentPlayer)
	}
}
