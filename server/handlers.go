package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"skat/game"
	"skat/logger"
	"skat/server/db"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
)

// SetupRoutes configures HTTP routes
func (s *Server) SetupRoutes() http.Handler {
	r := mux.NewRouter()

	// Health check for Cloud Run (before catch-all)
	r.HandleFunc("/health", s.handleHealth).Methods("GET")

	// WebSocket endpoint
	r.HandleFunc("/ws", s.HandleWebSocket)

	// REST API endpoints
	api := r.PathPrefix("/api").Subrouter()
	api.HandleFunc("/games", s.handleListOpenSessions).Methods("GET")
	api.HandleFunc("/games", s.handleCreateGame).Methods("POST")
	api.HandleFunc("/games/{id}", s.handleGetGame).Methods("GET")
	api.HandleFunc("/games/{code}/join", s.handleJoinGame).Methods("POST")
	api.HandleFunc("/games/{id}/leave", s.handleLeaveGame).Methods("POST")
	api.HandleFunc("/games/{id}/agents", s.handleAddAgent).Methods("POST")
	api.HandleFunc("/games/{id}/results", s.handleGetSessionResults).Methods("GET")
	api.HandleFunc("/games/{id}/deal", s.handleDeal).Methods("POST")
	api.HandleFunc("/games/{id}/play_card", s.handlePlayCard).Methods("POST")
	api.HandleFunc("/games/{id}/bid", s.handleBid).Methods("POST")
	api.HandleFunc("/games/{id}/choose_game", s.handleChooseGame).Methods("POST")
	api.HandleFunc("/games/{id}/skat_decision", s.handleSkatDecision).Methods("POST")
	api.HandleFunc("/games/{id}/discard_cards", s.handleDiscardCards).Methods("POST")
	api.HandleFunc("/games/{id}/start_next_game", s.handleStartNextGame).Methods("POST")
	api.HandleFunc("/games/{id}/timeout", s.handleTimeout).Methods("POST")
	api.HandleFunc("/profiles", s.handleCreateProfile).Methods("POST")
	api.HandleFunc("/profiles/{id}/avatar", s.handleUploadAvatar).Methods("POST")
	api.HandleFunc("/players/{id}/history", s.handleGetPlayerHistory).Methods("GET")
	api.HandleFunc("/players/{id}/active_games", s.handleGetActiveGames).Methods("GET")
	api.HandleFunc("/players/{id}/rating", s.handleGetPlayerRating).Methods("GET")
	api.HandleFunc("/leaderboard", s.handleGetLeaderboard).Methods("GET")

	// Serve React build files (must be last - catch-all)
	spa := http.FileServer(http.Dir("./frontend/build"))
	r.PathPrefix("/").Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to serve the requested file
		path := "./frontend/build" + r.URL.Path
		if r.URL.Path == "/" || !fileExists(path) {
			// Serve index.html for root or non-existent files (SPA routing)
			http.ServeFile(w, r, "./frontend/build/index.html")
		} else {
			spa.ServeHTTP(w, r)
		}
	}))

	// Wrap with CORS middleware
	allowedOrigins := []string{"http://localhost:3000", "http://192.168.1.125:3000", "https://skat.erikpolzin.com"}

	// Add production CORS origins from environment variable (comma-separated)
	if corsOrigins := os.Getenv("CORS_ORIGIN"); corsOrigins != "" {
		for _, origin := range strings.Split(corsOrigins, ",") {
			origin = strings.TrimSpace(origin)
			if origin != "" {
				allowedOrigins = append(allowedOrigins, origin)
			}
		}
	}

	c := cors.New(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
	})

	return c.Handler(r)
}

// handleHealth returns server health status
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

// handleListOpenSessions returns all open games (accepting players)
// Optionally filters out games where the specified player is already playing
func (s *Server) handleListOpenSessions(w http.ResponseWriter, r *http.Request) {
	// Get optional player_id from query params to filter out games they're already in
	playerID := r.URL.Query().Get("exclude_player_id")

	games, err := s.db.ListOpenSessions()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Filter out games where the player is already playing
	if playerID != "" {
		activeGames, err := s.db.GetActiveGamesByPlayer(playerID)
		if err == nil {
			// Create a map of active game IDs for quick lookup
			activeGameIDs := make(map[string]bool)
			for _, ag := range activeGames {
				activeGameIDs[ag.ID] = true
			}

			// Filter out games where player is already playing
			filteredGames := []game.GameSessionState{}
			for _, g := range games {
				if !activeGameIDs[g.GameID] {
					filteredGames = append(filteredGames, g)
				}
			}
			games = filteredGames
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(games)
}

// handleCreateGame creates a new game session
func (s *Server) handleCreateGame(w http.ResponseWriter, r *http.Request) {
	// Get optional player_id from query params to check if they can create a game
	playerID := r.URL.Query().Get("player_id")

	// Check if player is already in an active game before creating
	if playerID != "" {
		activeGames, err := s.db.GetActiveGamesByPlayer(playerID)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to check active games: %v", err), http.StatusInternalServerError)
			return
		}
		if len(activeGames) > 0 {
			http.Error(w, "player is already in an active game", http.StatusConflict)
			return
		}
	}

	// Create empty session
	gs := game.NewGame()

	// Save the session to the database
	if err := s.db.SaveGameSession(game.GameSessionState{
		ID:     gs.SessionID,
		Code:   string(gs.Code),
		GameID: gs.ID,
	}); err != nil {
		http.Error(w, fmt.Sprintf("failed to save game session: %v", err), http.StatusInternalServerError)
		return
	}

	// Save to database
	if err := s.db.SaveGame(*gs); err != nil {
		http.Error(w, fmt.Sprintf("failed to save game: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"game_id": gs.ID,
		"code":    string(gs.Code),
	})
}

// handleGetGame returns game information
func (s *Server) handleGetGame(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	gameID := vars["id"]
	playerID := r.URL.Query().Get("player_id")

	gs, err := s.db.GetGameByID(gameID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(gs.SerializeForPlayer(playerID))
}

// handleCreateProfile creates or retrieves a player profile
func (s *Server) handleCreateProfile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PlayerName string `json:"player_name"`
		PlayerID   string `json:"player_id,omitempty"` // Optional, for existing profiles
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	playerID := req.PlayerID
	playerName := req.PlayerName

	// If no player ID provided, check if username already exists
	if playerID == "" {
		// Check if a profile with this name already exists
		if existingProfile, err := s.db.GetProfileByName(playerName); err == nil {
			// Profile with this name exists
			if existingProfile.IsAgent {
				// Username belongs to an agent, reject it
				http.Error(w, "Username is reserved for an agent", http.StatusConflict)
				return
			}
			// Return the existing profile
			logger.Info("Returning existing profile", "player_id", existingProfile.ID, "player_name", existingProfile.Name)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"player_id":    existingProfile.ID,
				"player_name":  existingProfile.Name,
				"profile_icon": existingProfile.ProfileIcon,
			})
			return
		}

		// No existing profile, create a new one
		playerID = uuid.New().String()
		logger.Info("Creating new profile", "player_id", playerID, "player_name", playerName)

		// Store the new profile
		profile := db.ProfileEntry{
			ID:   playerID,
			Name: playerName,
		}
		if err := s.db.SaveProfile(profile); err != nil {
			logger.Warning("Failed to store player profile", "error", err)
		}
	} else {
		// Existing player ID - retrieve or update name
		var profileIcon string
		if profile, err := s.db.GetProfile(playerID); err == nil {
			// Profile exists, optionally update name if different
			profileIcon = profile.ProfileIcon
			if profile.Name != playerName && playerName != "" {
				profile.Name = playerName
				if err := s.db.SaveProfile(*profile); err != nil {
					logger.Warning("Failed to update player profile", "error", err)
				}
			} else if profile.Name != "" {
				// Use the stored name if no new name provided
				playerName = profile.Name
			}
		} else {
			// Profile doesn't exist, create it
			profile := db.ProfileEntry{
				ID:   playerID,
				Name: playerName,
			}
			if err := s.db.SaveProfile(profile); err != nil {
				logger.Warning("Failed to store player profile", "error", err)
			}
			logger.Info("Created profile for existing ID", "player_id", playerID, "player_name", playerName)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"player_id":    playerID,
			"player_name":  playerName,
			"profile_icon": profileIcon,
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"player_id":   playerID,
		"player_name": playerName,
	})
}

// handleJoinGame adds a human player to a game
func (s *Server) handleJoinGame(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	code := vars["code"]

	var req struct {
		PlayerID string `json:"player_id,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	playerID := req.PlayerID
	if playerID == "" {
		http.Error(w, "player ID is empty", http.StatusBadRequest)
		return
	}
	// Fetch profile
	profile, profile_err := s.db.GetProfile(playerID)
	if profile_err != nil {
		http.Error(w, profile_err.Error(), http.StatusNotFound)
		return
	}

	// Check if player is already in an active game
	activeGames, err := s.db.GetActiveGamesByPlayer(playerID)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to check active games: %v", err), http.StatusInternalServerError)
		return
	}
	if len(activeGames) > 0 {
		http.Error(w, "player is already in an active game", http.StatusConflict)
		return
	}
	// Fetch game
	gs, game_err := s.db.GetGameBySessionCode(code)
	if game_err != nil {
		http.Error(w, game_err.Error(), http.StatusNotFound)
		return
	}
	// Actually add the player to the game
	response, err := gs.AddPlayer(&game.PlayerState{
		ID:          playerID,
		Name:        profile.Name,
		ProfileIcon: profile.ProfileIcon,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.db.SaveGame(*gs)
	s.clients.BroadcastStateChange(gs, response, gs.GetPositionForPlayer(playerID))
	go s.BroadcastAIActions(gs)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"game_id": gs.ID,
	})

	logger.Info("Player joined game via HTTP", "player_id", playerID, "player_name", profile.Name, "game_id", gs.ID)
}

// handleLeaveGame removes a player from a game
func (s *Server) handleLeaveGame(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	gameID := vars["id"]

	var req struct {
		PlayerID string `json:"player_id,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	playerID := req.PlayerID
	if playerID == "" {
		http.Error(w, "player ID is empty", http.StatusBadRequest)
		return
	}

	// Fetch game
	gs, err := s.db.GetGameByID(gameID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Find the player
	position := gs.GetPositionForPlayer(playerID)
	if position == -1 {
		http.Error(w, "player not in this game", http.StatusBadRequest)
		return
	}

	// Get player name for broadcast messages
	playerName := gs.Players[position].Name

	// Check if game is in progress (not waiting for players)
	if gs.Phase != game.PhaseWaitingForPlayers && gs.Phase != game.PhaseComplete {
		// Game is in progress - end it and award points to remaining players
		// Player who left forfeits and loses maximum points
		gs.Phase = game.PhaseComplete
		gs.ForfeitedPlayer = &position

		s.db.SaveGame(*gs)
		s.maybeSaveGameResults(gs)

		// Broadcast forfeit to other players
		s.clients.BroadcastToPlayers(gs, &Message{
			Type: "player_forfeit",
			Data: map[string]any{
				"player_id":   playerID,
				"player_name": playerName,
				"game_id":     gs.ID,
			},
		})

		logger.Info("Player forfeited game", "player_id", playerID, "game_id", gs.ID)
	} else {
		// Game hasn't started yet - just remove the player
		gs.Players[position] = nil

		// If no players left, delete the game
		if gs.PlayerCount() == 0 {
			if err := s.db.DeleteGame(gs.ID); err != nil {
				logger.Warning("Failed to delete empty game", "game_id", gs.ID, "error", err)
			}
			logger.Info("Player left game, game deleted (no players remaining)", "player_id", playerID, "game_id", gs.ID)
		} else {
			// Save the updated game
			s.db.SaveGame(*gs)

			// Broadcast to other players that someone left
			s.clients.BroadcastToPlayers(gs, &Message{
				Type: "player_left",
				Data: map[string]any{
					"player_id":   playerID,
					"player_name": playerName,
					"game_id":     gs.ID,
				},
			})

			logger.Info("Player left game (lobby)", "player_id", playerID, "game_id", gs.ID, "remaining_players", gs.PlayerCount())
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "left"})
}

// handleAddAgent adds an AI agent to a game
func (s *Server) handleAddAgent(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	gameID := vars["id"]

	gs, err := s.db.GetGameByID(gameID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Get all available agent profiles from database
	allAgentProfiles, err := s.db.ListAgentProfiles()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Filter out agents already in the game
	var availableProfiles []db.ProfileEntry
	for _, profile := range allAgentProfiles {
		inUse := false
		for _, player := range gs.Players {
			if player != nil && player.ID == profile.ID {
				inUse = true
				break
			}
		}
		if !inUse {
			availableProfiles = append(availableProfiles, profile)
		}
	}

	if len(availableProfiles) == 0 {
		http.Error(w, "no available agents", http.StatusBadRequest)
		return
	}

	// Pick a random available agent
	agentProfile := availableProfiles[rand.Int()%len(availableProfiles)]

	response, err := gs.AddPlayer(&game.PlayerState{
		ID:          agentProfile.ID,
		Name:        agentProfile.Name,
		IsAgent:     true,
		ProfileIcon: agentProfile.ProfileIcon,
		IsOnline:    true, // Agents are always online
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.db.SaveGame(*gs)
	s.clients.BroadcastStateChange(gs, response, gs.GetPositionForPlayer(agentProfile.ID))
	go s.BroadcastAIActions(gs)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "agent added"})
}

// handleGetPlayerHistory returns a player's game history
func (s *Server) handleGetPlayerHistory(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	playerID := vars["id"]

	// Get limit from query params, default to 10
	limitStr := r.URL.Query().Get("limit")
	limit := 10
	if limitStr != "" {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit < 1 || limit > 100 {
			limit = 10
		}
	}

	results, err := s.db.GetPlayerResults(playerID, limit)
	if err != nil {
		logger.Warning("Failed to get player results", "player_id", playerID, "error", err)
		// Return empty array on error rather than failing
		results = []game.PlayerResultState{}
	}

	// Rating changes are now included in player results
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// handleGetActiveGames returns active games for a player
func (s *Server) handleGetActiveGames(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	playerID := vars["id"]

	games, err := s.db.GetActiveGamesByPlayer(playerID)
	if err != nil {
		logger.Warning("Failed to get active games", "player_id", playerID, "error", err)
		// Return empty array on error rather than failing
		games = []game.GameState{}
	}

	// Transform to session-like response for consistency
	type ActiveGameResponse struct {
		ID          string   `json:"id"`
		Code        string   `json:"code"`
		SessionID   string   `json:"session_id"`
		GameNumber  int      `json:"game_number"`
		PlayerCount int      `json:"player_count"`
		Phase       string   `json:"phase"`
		PlayerNames []string `json:"player_names"`
	}

	response := make([]ActiveGameResponse, len(games))
	for i, gs := range games {
		playerCount := 0
		playerNames := []string{}
		for _, p := range gs.Players {
			if p != nil {
				playerCount++
				playerNames = append(playerNames, p.Name)
			}
		}
		response[i] = ActiveGameResponse{
			ID:          gs.ID,
			Code:        string(gs.Code),
			SessionID:   gs.SessionID,
			GameNumber:  gs.GameNumber,
			PlayerCount: playerCount,
			Phase:       string(gs.Phase),
			PlayerNames: playerNames,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleGetSessionResults returns all game results for a session
func (s *Server) handleGetSessionResults(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["id"]

	results, err := s.db.GetFormattedSessionResults(sessionID)
	if err != nil {
		logger.Warning("Failed to get session results", "session_id", sessionID, "error", err)
		http.Error(w, fmt.Sprintf("Failed to get session results: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"session_id": sessionID,
		"results":    results,
	})
}

// handleUploadAvatar handles avatar image upload for a player profile
func (s *Server) handleUploadAvatar(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	playerID := vars["id"]

	// Verify the profile exists
	profile, err := s.db.GetProfile(playerID)
	if err != nil {
		http.Error(w, "Profile not found", http.StatusNotFound)
		return
	}

	// Parse multipart form (10 MB max)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("avatar")
	if err != nil {
		http.Error(w, "Failed to get file from form", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Validate file type
	contentType := header.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		http.Error(w, "File must be an image", http.StatusBadRequest)
		return
	}

	// Generate unique filename
	ext := filepath.Ext(header.Filename)
	if ext == "" {
		ext = ".jpg" // Default extension
	}
	filename := fmt.Sprintf("%s%s", playerID, ext)

	var avatarURL string

	// Check if GCS bucket is configured
	gcsBucket := os.Getenv("GCS_BUCKET")
	if gcsBucket != "" {
		// Upload to GCS
		ctx := context.Background()
		client, err := storage.NewClient(ctx)
		if err != nil {
			logger.Warning("Failed to create GCS client", "error", err)
			http.Error(w, "Failed to upload avatar", http.StatusInternalServerError)
			return
		}
		defer client.Close()

		objectPath := fmt.Sprintf("avatars/%s", filename)
		bucket := client.Bucket(gcsBucket)
		obj := bucket.Object(objectPath)
		writer := obj.NewWriter(ctx)
		writer.ContentType = contentType

		if _, err := io.Copy(writer, file); err != nil {
			writer.Close()
			logger.Warning("Failed to upload to GCS", "error", err)
			http.Error(w, "Failed to upload avatar", http.StatusInternalServerError)
			return
		}

		if err := writer.Close(); err != nil {
			logger.Warning("Failed to close GCS writer", "error", err)
			http.Error(w, "Failed to upload avatar", http.StatusInternalServerError)
			return
		}

		// Make the object publicly readable
		if err := obj.ACL().Set(ctx, storage.AllUsers, storage.RoleReader); err != nil {
			logger.Warning("Failed to set ACL", "error", err)
		}

		avatarURL = fmt.Sprintf("https://storage.googleapis.com/%s/%s", gcsBucket, objectPath)
		logger.Info("Avatar uploaded to GCS", "player_id", playerID, "url", avatarURL)
	} else {
		// Store locally in frontend/public/res/avatars
		uploadDir := "./frontend/public/res/avatars"
		if err := os.MkdirAll(uploadDir, 0755); err != nil {
			logger.Warning("Failed to create avatars directory", "error", err)
			http.Error(w, "Failed to save avatar", http.StatusInternalServerError)
			return
		}

		localPath := filepath.Join(uploadDir, filename)
		outFile, err := os.Create(localPath)
		if err != nil {
			logger.Warning("Failed to create file", "error", err)
			http.Error(w, "Failed to save avatar", http.StatusInternalServerError)
			return
		}
		defer outFile.Close()

		if _, err := io.Copy(outFile, file); err != nil {
			logger.Warning("Failed to write file", "error", err)
			http.Error(w, "Failed to save avatar", http.StatusInternalServerError)
			return
		}

		avatarURL = fmt.Sprintf("/res/avatars/%s", filename)
		logger.Info("Avatar saved locally", "player_id", playerID, "path", localPath)
	}

	// Update profile with avatar URL
	profile.ProfileIcon = avatarURL
	if err := s.db.SaveProfile(*profile); err != nil {
		logger.Warning("Failed to update profile", "error", err)
		http.Error(w, "Failed to save profile", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"profile_icon": avatarURL,
	})
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Helper to get authenticated player ID from request
func getAuthenticatedPlayerID(r *http.Request) (string, error) {
	playerID := r.URL.Query().Get("player_id")
	if playerID == "" {
		return "", fmt.Errorf("player_id required")
	}
	return playerID, nil
}

// GameActionFunc is a function that performs a game action
type GameActionFunc func(gs *game.GameState, playerID string, r *http.Request) (string, error)

// handleGameAction is a common handler for game actions
func (s *Server) handleGameAction(w http.ResponseWriter, r *http.Request, validateTurn bool, action GameActionFunc) {
	// Add 2 second delay for local development to test loading states
	if !s.IsCloudRun() {
		time.Sleep(2 * time.Second)
	}

	vars := mux.Vars(r)
	gameID := vars["id"]

	playerID, err := getAuthenticatedPlayerID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	gs, err := s.db.GetGameByID(gameID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Validate it's the current player's turn (if required)
	if validateTurn {
		currentPlayer := gs.GetCurrentPlayer()
		if currentPlayer.ID != playerID {
			http.Error(w, "not your turn", http.StatusForbidden)
			return
		}
	}

	currentPosition := gs.CurrentPlayer
	response, err := action(gs, playerID, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.db.SaveGame(*gs)
	s.clients.BroadcastStateChange(gs, response, currentPosition)
	go s.BroadcastAIActions(gs)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleDeal handles the deal action
func (s *Server) handleDeal(w http.ResponseWriter, r *http.Request) {
	s.handleGameAction(w, r, true, func(gs *game.GameState, playerID string, r *http.Request) (string, error) {
		return gs.Deal()
	})
}

// handlePlayCard handles playing a card
func (s *Server) handlePlayCard(w http.ResponseWriter, r *http.Request) {
	s.handleGameAction(w, r, true, func(gs *game.GameState, playerID string, r *http.Request) (string, error) {
		var req struct {
			Card string `json:"card"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return "", err
		}

		card, err := game.ParseCard(req.Card)
		if err != nil {
			return "", fmt.Errorf("invalid card: %v", err)
		}

		response, err := gs.PlayCard(card)
		if err == nil {
			s.maybeSaveGameResults(gs)
		}
		return response, err
	})
}

// handleBid handles bidding actions
func (s *Server) handleBid(w http.ResponseWriter, r *http.Request) {
	s.handleGameAction(w, r, true, func(gs *game.GameState, playerID string, r *http.Request) (string, error) {
		var req struct {
			Accept bool `json:"accept"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return "", err
		}
		return gs.Bid(req.Accept)
	})
}

// handleChooseGame handles game mode selection
func (s *Server) handleChooseGame(w http.ResponseWriter, r *http.Request) {
	s.handleGameAction(w, r, true, func(gs *game.GameState, playerID string, r *http.Request) (string, error) {
		var req struct {
			Mode              string `json:"mode"`
			Trump             string `json:"trump"`
			AnnounceSchneider bool   `json:"announce_schneider"`
			AnnounceSchwarz   bool   `json:"announce_schwarz"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return "", err
		}

		mode := game.GameMode(req.Mode)
		trump, err := game.ParseSuit(req.Trump)
		if err != nil {
			return "", fmt.Errorf("invalid trump suit: %v", err)
		}

		return gs.DeclareGame(mode, trump, req.AnnounceSchneider, req.AnnounceSchwarz)
	})
}

// handleSkatDecision handles the declarer's skat pickup decision
func (s *Server) handleSkatDecision(w http.ResponseWriter, r *http.Request) {
	s.handleGameAction(w, r, true, func(gs *game.GameState, playerID string, r *http.Request) (string, error) {
		var req struct {
			Pickup bool `json:"pickup"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return "", err
		}
		return gs.SkatDecision(req.Pickup)
	})
}

// handleDiscardCards handles card discarding
func (s *Server) handleDiscardCards(w http.ResponseWriter, r *http.Request) {
	s.handleGameAction(w, r, true, func(gs *game.GameState, playerID string, r *http.Request) (string, error) {
		var req struct {
			Cards string `json:"cards"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return "", err
		}

		cards, err := game.ParseSkatCards(req.Cards)
		if err != nil {
			return "", fmt.Errorf("invalid cards: %v", err)
		}

		return gs.Discard(cards[0], cards[1])
	})
}

// handleStartNextGame handles starting the next game
func (s *Server) handleStartNextGame(w http.ResponseWriter, r *http.Request) {
	// Add 2 second delay for local development to test loading states
	if !s.IsCloudRun() {
		time.Sleep(2 * time.Second)
	}

	vars := mux.Vars(r)
	gameID := vars["id"]

	_, err := getAuthenticatedPlayerID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	gs, err := s.db.GetGameByID(gameID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// For start_next_game, we don't validate current player since any player can trigger it
	currentPosition := gs.CurrentPlayer
	response, err := gs.NextGame()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	newGameID := gs.ID
	s.db.SaveGame(*gs)

	// Send start_next_game message to trigger navigation
	s.clients.BroadcastToPlayers(gs, &Message{
		Type: "start_next_game",
		Data: map[string]any{"game_id": newGameID},
	})

	// Also broadcast the state change
	s.clients.BroadcastStateChange(gs, response, currentPosition)
	go s.BroadcastAIActions(gs)

	// Custom response with new game ID
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"game_id": newGameID,
	})
}

func (s *Server) handleTimeout(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	gameID := vars["id"]

	_, err := getAuthenticatedPlayerID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	gs, err := s.db.GetGameByID(gameID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Check if game is already complete
	if gs.Phase == game.PhaseComplete {
		http.Error(w, "Game is already complete", http.StatusBadRequest)
		return
	}

	// Check if there's a deadline set
	if gs.CurrentPlayerDeadline == "" {
		http.Error(w, "No deadline is set", http.StatusBadRequest)
		return
	}

	currentPlayer := gs.GetCurrentPlayer()
	if currentPlayer == nil {
		http.Error(w, "No current player", http.StatusBadRequest)
		return
	}

	logger.Info("Game timeout reported by client",
		"game_id", gs.ID,
		"inactive_player", currentPlayer.Name,
		"player_id", currentPlayer.ID,
		"deadline", gs.CurrentPlayerDeadline)

	// Forfeit the game
	results := gs.ForfeitDueToInactivity()
	logger.Info("Set game phase to complete", "game_id", gs.ID, "phase", gs.Phase)

	// Save results to database
	if err := s.db.SavePlayerResults(results); err != nil {
		logger.Warning("Failed to save timeout forfeit results", "game_id", gs.ID, "error", err)
	}

	// Save the updated game state
	if err := s.db.SaveGame(*gs); err != nil {
		logger.Warning("Failed to save game after timeout", "game_id", gs.ID, "error", err)
	} else {
		logger.Info("Saved game with phase complete", "game_id", gs.ID)
	}

	// Broadcast the updated game state so clients show game over screen
	timeoutMsg := fmt.Sprintf("%s was inactive for 2 minutes and has forfeited the game", currentPlayer.Name)
	s.clients.BroadcastStateChange(gs, timeoutMsg, gs.CurrentPlayer)

	logger.Info("Game forfeited due to timeout (client-reported)", "game_id", gs.ID, "player_id", currentPlayer.ID, "phase", gs.Phase)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleGetPlayerRating returns the rating for a player
func (s *Server) handleGetPlayerRating(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	playerID := vars["id"]

	rating, err := s.db.GetPlayerRating(playerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get player's rank
	leaderboard, err := s.db.GetLeaderboard(0) // Get all to calculate rank
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	rank := 0
	for i, r := range leaderboard {
		if r.ProfileID == playerID {
			rank = i + 1
			break
		}
	}

	// Get profile for name
	profile, err := s.db.GetProfile(playerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]any{
		"profile_id":   rating.ProfileID,
		"name":         profile.Name,
		"rating":       rating.Rating,
		"games_played": rating.GamesPlayed,
		"wins":         rating.Wins,
		"losses":       rating.Losses,
		"peak_rating":  rating.PeakRating,
		"rank":         rank,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleGetLeaderboard returns the leaderboard
func (s *Server) handleGetLeaderboard(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	ratings, err := s.db.GetLeaderboard(limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Enrich with profile information
	type LeaderboardEntry struct {
		Rank        int     `json:"rank"`
		ProfileID   string  `json:"profile_id"`
		Name        string  `json:"name"`
		ProfileIcon string  `json:"profile_icon"`
		Rating      int     `json:"rating"`
		GamesPlayed int     `json:"games_played"`
		Wins        int     `json:"wins"`
		Losses      int     `json:"losses"`
		WinRate     float64 `json:"win_rate"`
	}

	var leaderboard []LeaderboardEntry
	for i, rating := range ratings {
		profile, err := s.db.GetProfile(rating.ProfileID)
		if err != nil {
			continue // Skip if profile not found
		}

		winRate := 0.0
		if rating.GamesPlayed > 0 {
			winRate = float64(rating.Wins) / float64(rating.GamesPlayed) * 100
		}

		leaderboard = append(leaderboard, LeaderboardEntry{
			Rank:        i + 1,
			ProfileID:   rating.ProfileID,
			Name:        profile.Name,
			ProfileIcon: profile.ProfileIcon,
			Rating:      rating.Rating,
			GamesPlayed: rating.GamesPlayed,
			Wins:        rating.Wins,
			Losses:      rating.Losses,
			WinRate:     winRate,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(leaderboard)
}
