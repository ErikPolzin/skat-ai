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
	if gs.Phase != game.PhaseComplete {
		return nil
	}
	results := gs.PlayerResults()
	if results == nil && gs.ForfeitedPlayer == nil {
		logger.Warning("Failed to save player results, no results")
		return nil
	}

	maxGames := gs.MaxGames
	if maxGames <= 0 {
		maxGames = game.DefaultMaxGames
	}
	isFinalGame := gs.GameNumber+1 >= maxGames || gs.ForfeitedPlayer != nil

	session, err := s.db.GetGameSession(gs.SessionID)
	if err == nil && session.EndedAt != nil {
		return nil
	}

	if results != nil {
		// Save Skat/game points after each completed game so the session table can update.
		if err := s.db.SavePlayerResults(results[:]); err != nil {
			logger.Warning("Failed to save player results: %e", err)
		}
	}

	if !isFinalGame {
		return nil
	}

	gameResults, err := s.db.GetPlayerResultsForSession(gs.SessionID)
	if err != nil {
		return fmt.Errorf("failed to get session game results: %w", err)
	}
	sessionResults := aggregateSessionResults(gs, gameResults)

	playerRatings := make(map[string]*rating.PlayerRating)
	aiCount := 0
	for _, player := range gs.Players {
		if player != nil {
			rating, err := s.db.GetPlayerRating(player.ID)
			if err != nil {
				return fmt.Errorf("failed to get player rating: %w", err)
			}
			playerRatings[player.ID] = rating.ToGamePlayerRating()
			if player.IsAgent {
				aiCount++
			}
		}
	}

	if err := rating.UpdateRatings(sessionResults, playerRatings, aiCount); err != nil {
		logger.Warning("Failed to update player ratings: %e", err)
	}

	for _, rat := range playerRatings {
		if err := s.db.SavePlayerRating(db.NewPlayerRating(rat)); err != nil {
			return fmt.Errorf("failed to save player rating: %w", err)
		}
	}

	if err := s.db.SavePlayerSessionResults(sessionResults); err != nil {
		logger.Warning("Failed to save player session results: %e", err)
	}

	endedAt := time.Now().UTC().Format(time.RFC3339)
	if err := s.db.SaveGameSession(game.GameSessionState{
		ID:           gs.SessionID,
		Code:         string(gs.Code),
		GameID:       gs.ID,
		PlayerCount:  gs.PlayerCount(),
		MaxGames:     maxGames,
		PassPolicy:   string(gs.PassPolicy),
		TimerEnabled: gs.TimerEnabled,
		EndedAt:      &endedAt,
	}); err != nil {
		logger.Warning("Failed to mark session ended: %e", err)
	}
	return nil
}

func aggregateSessionResults(gs *game.GameState, gameResults []game.PlayerResultState) []game.PlayerSessionResultState {
	type gamePlayerKey struct {
		gameID   string
		playerID string
	}

	deduped := make(map[gamePlayerKey]game.PlayerResultState)
	for _, result := range gameResults {
		deduped[gamePlayerKey{gameID: result.GameID, playerID: result.PlayerID}] = result
	}

	totals := make(map[string]int)
	for _, result := range deduped {
		totals[result.PlayerID] += result.PlayerPoints
	}

	topScore := 0
	hasScore := false
	for playerID, points := range totals {
		if forfeitedPlayerID(gs) == playerID {
			continue
		}
		if !hasScore || points > topScore {
			topScore = points
			hasScore = true
		}
	}

	sessionResults := make([]game.PlayerSessionResultState, 0, len(totals))
	for _, player := range gs.Players {
		if player == nil {
			continue
		}
		points := totals[player.ID]
		isForfeit := gs.ForfeitedPlayer != nil && gs.GetPositionForPlayer(player.ID) == *gs.ForfeitedPlayer
		sessionResults = append(sessionResults, game.PlayerSessionResultState{
			SessionID:    gs.SessionID,
			PlayerID:     player.ID,
			PlayerPoints: points,
			IsWinner:     !isForfeit && hasScore && points == topScore,
			IsForfeit:    isForfeit,
		})
	}
	return sessionResults
}

func forfeitedPlayerID(gs *game.GameState) string {
	if gs.ForfeitedPlayer == nil {
		return ""
	}
	player := gs.GetPlayerByPosition(*gs.ForfeitedPlayer)
	if player == nil {
		return ""
	}
	return player.ID
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
