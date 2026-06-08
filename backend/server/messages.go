package server

import (
	"errors"
	"fmt"
	"skat/agent"
	"skat/game"
	"skat/game/rating"
	"skat/logger"
	cachepkg "skat/server/cache"
	"skat/server/db"
	"sort"
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

func (s *Server) BroadcastStateChange(gs *game.GameState, msg string, fromPlayer game.GamePosition) {
	// Fetch formatted session results if game just completed
	var sessionResults []game.SessionGameResult
	var sessionPlayerResults []game.PlayerSessionResultState
	var gamesPlayed int
	if gs.Phase == game.PhaseComplete && gs.SessionID != "" {
		results, err := s.db.GetFormattedSessionResults(gs.SessionID)
		if err != nil {
			logger.Warning("Failed to fetch session results for broadcast: %e", err)
		} else {
			sessionResults = results
			gamesPlayed = len(results)
		}
		playerResults, err := s.db.GetSessionPlayerResults(gs.SessionID)
		if err != nil {
			logger.Warning("Failed to fetch session player results for broadcast: %e", err)
		} else {
			sessionPlayerResults = playerResults
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
			if sessionPlayerResults != nil {
				msgData["session_player_results"] = sessionPlayerResults
			}

			stateMsg := &Message{
				Type: "state_update",
				Data: msgData,
			}
			s.clients.SendToClient(player.ID, stateMsg)
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
	if maxGames < 0 {
		maxGames = game.DefaultMaxGames
	}
	isFinalGame := (maxGames > 0 && gs.GameNumber+1 >= maxGames) || gs.ForfeitedPlayer != nil

	session, err := s.db.GetGameSession(gs.SessionID)
	if err == nil && session.EndedAt != nil {
		existingResults, err := s.db.GetSessionPlayerResults(gs.SessionID)
		if err != nil {
			return fmt.Errorf("failed to check existing session results: %w", err)
		}
		if len(existingResults) > 0 {
			return nil
		}
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
	for _, player := range gs.Players {
		if player != nil {
			rating, err := s.db.GetPlayerRating(player.ID)
			if err != nil {
				return fmt.Errorf("failed to get player rating: %w", err)
			}
			playerRatings[player.ID] = rating.ToGamePlayerRating()
		}
	}

	if err := rating.UpdateRatings(sessionResults, playerRatings, completedHandsForRating(gs, results != nil), declarerStats(gameResults)); err != nil {
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
		ID:               gs.SessionID,
		Code:             string(gs.Code),
		GameID:           gs.ID,
		PlayerCount:      gs.PlayerCount(),
		MaxGames:         maxGames,
		PassPolicy:       string(gs.PassPolicy),
		TimerEnabled:     gs.TimerEnabled,
		CompletionPolicy: string(gs.CompletionPolicy),
		EndedAt:          &endedAt,
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
	assignSessionPositions(sessionResults)
	return sessionResults
}

func assignSessionPositions(results []game.PlayerSessionResultState) {
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].IsForfeit != results[j].IsForfeit {
			return !results[i].IsForfeit
		}
		if results[i].PlayerPoints != results[j].PlayerPoints {
			return results[i].PlayerPoints > results[j].PlayerPoints
		}
		return results[i].RatingChange > results[j].RatingChange
	})
	for i := range results {
		results[i].Position = i + 1
	}
}

func completedHandsForRating(gs *game.GameState, currentGameCompleted bool) int {
	if currentGameCompleted {
		return gs.GameNumber + 1
	}
	return gs.GameNumber
}

func countSessionGames(gameResults []game.PlayerResultState) int {
	gameIDs := make(map[string]struct{})
	for _, result := range gameResults {
		gameIDs[result.GameID] = struct{}{}
	}
	return len(gameIDs)
}

func declarerStats(gameResults []game.PlayerResultState) map[string]rating.PlayerStats {
	stats := make(map[string]rating.PlayerStats)
	for _, result := range gameResults {
		if !result.IsDeclarer {
			continue
		}
		playerStats := stats[result.PlayerID]
		playerStats.GamesPlayed++
		if result.IsWinner {
			playerStats.Wins++
		} else {
			playerStats.Losses++
		}
		stats[result.PlayerID] = playerStats
	}
	return stats
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

const maxStaleAIActionRetries = 3

func (s *Server) BroadcastAIActions(gs *game.GameState) {
	staleRetries := 0
	for {
		action := agent.NextAction(gs)
		if action == nil {
			break
		}
		currentPlayer := gs.CurrentPlayer

		time.Sleep(1 * time.Second)

		response, err := action()
		if err != nil {
			logger.Error("Agent encountered an error: %v", err)
			s.clients.BroadcastToPlayers(gs, &Message{
				Type: "error",
				Data: map[string]any{"message": err.Error()},
			})
			return
		}
		if err := s.cache.SaveGame(gs); err != nil {
			if errors.Is(err, cachepkg.ErrStaleGameState) {
				staleRetries++
				if staleRetries > maxStaleAIActionRetries {
					logger.Warning("Stopping AI action loop after stale retries for game %s", gs.ID)
					return
				}
				latest, loadErr := s.cache.GetGameByID(gs.ID)
				if loadErr != nil {
					logger.Warning("Failed to reload stale AI game %s: %e", gs.ID, loadErr)
					return
				}
				logger.Info("Reloaded stale AI game %s after save conflict", gs.ID)
				gs = latest
				continue
			}
			logger.Error("Failed to save game after AI action: %v", err)
			s.clients.BroadcastToPlayers(gs, &Message{
				Type: "error",
				Data: map[string]any{"message": "failed to save game"},
			})
			return
		}
		staleRetries = 0
		if err := s.maybeSaveGameResults(gs); err != nil {
			logger.Warning("Failed to save game results after AI action: %e", err)
		}
		s.BroadcastStateChange(gs, response, currentPlayer)
	}
}
