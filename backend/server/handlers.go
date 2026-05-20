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

// cloudRunDelayMiddleware adds artificial delay in non-Cloud Run environments
func (s *Server) cloudRunDelayMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.IsCloudRun() {
			time.Sleep(0 * time.Second)
		}
		next.ServeHTTP(w, r)
	})
}

// SetupRoutes configures HTTP routes
func (s *Server) SetupRoutes() http.Handler {
	r := mux.NewRouter()

	// Health check for Cloud Run (before catch-all)
	r.HandleFunc("/health", s.handleHealth).Methods("GET")

	// WebSocket endpoint
	r.HandleFunc("/ws", s.HandleWebSocket)

	// REST API endpoints
	api := r.PathPrefix("/api").Subrouter()
	api.Use(s.cloudRunDelayMiddleware)
	api.HandleFunc("/profiles", s.handleCreateProfile).Methods("POST")

	authAPI := api.PathPrefix("").Subrouter()
	authAPI.Use(s.basicAuthMiddleware)
	authAPI.HandleFunc("/profile", s.handleCurrentProfile).Methods("GET")
	authAPI.HandleFunc("/games", s.handleListOpenSessions).Methods("GET")
	authAPI.HandleFunc("/games", s.handleCreateGame).Methods("POST")
	authAPI.HandleFunc("/games/{id}", s.handleGetGame).Methods("GET")
	authAPI.HandleFunc("/games/{code}/join", s.handleJoinGame).Methods("POST")
	authAPI.HandleFunc("/games/{id}/leave", s.handleLeaveGame).Methods("POST")
	authAPI.HandleFunc("/games/{id}/agents", s.handleAddAgent).Methods("POST")
	authAPI.HandleFunc("/games/{id}/results", s.handleGetSessionResults).Methods("GET")
	authAPI.HandleFunc("/games/{id}/deal", s.handleDeal).Methods("POST")
	authAPI.HandleFunc("/games/{id}/play_card", s.handlePlayCard).Methods("POST")
	authAPI.HandleFunc("/games/{id}/bid", s.handleBid).Methods("POST")
	authAPI.HandleFunc("/games/{id}/choose_game", s.handleChooseGame).Methods("POST")
	authAPI.HandleFunc("/games/{id}/skat_decision", s.handleSkatDecision).Methods("POST")
	authAPI.HandleFunc("/games/{id}/discard_cards", s.handleDiscardCards).Methods("POST")
	authAPI.HandleFunc("/games/{id}/ready_for_next", s.handleReadyForNext).Methods("POST")
	authAPI.HandleFunc("/games/{id}/timeout", s.handleTimeout).Methods("POST")
	authAPI.HandleFunc("/profiles/{id}/avatar", s.handleUploadAvatar).Methods("POST")
	authAPI.HandleFunc("/players/{id}/history", s.handleGetPlayerHistory).Methods("GET")
	authAPI.HandleFunc("/players/{id}/active_games", s.handleGetActiveGames).Methods("GET")
	authAPI.HandleFunc("/players/{id}/rating", s.handleGetPlayerRating).Methods("GET")
	authAPI.HandleFunc("/leaderboard", s.handleGetLeaderboard).Methods("GET")
	authAPI.HandleFunc("/agents", s.handleListAgents).Methods("GET")

	// Wrap with CORS middleware
	allowedOrigins := []string{"http://localhost:5173", "https://skat.erikpolzin.com"}

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
	profile, err := currentProfile(r)
	if err != nil {
		writeAuthRequired(w)
		return
	}
	playerID := profile.ID

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
	profile, err := currentProfile(r)
	if err != nil {
		writeAuthRequired(w)
		return
	}
	playerID := profile.ID

	// Check if player is already in an active game before creating
	activeGames, err := s.db.GetActiveGamesByPlayer(playerID)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to check active games: %v", err), http.StatusInternalServerError)
		return
	}
	if len(activeGames) > 0 {
		http.Error(w, "player is already in an active game", http.StatusConflict)
		return
	}

	// Create empty session
	gs := game.NewGame()
	var req struct {
		MaxGames   int    `json:"max_games"`
		PassPolicy string `json:"pass_policy"`
	}
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	if req.MaxGames == 0 {
		req.MaxGames = game.DefaultMaxGames
	}
	if req.MaxGames < 1 || req.MaxGames > 100 {
		http.Error(w, "max_games must be between 1 and 100", http.StatusBadRequest)
		return
	}
	passPolicy := game.PassPolicy(req.PassPolicy)
	if passPolicy == "" {
		passPolicy = game.DefaultPassPolicy
	}
	if passPolicy != game.PassPolicyReshuffle && passPolicy != game.PassPolicyForceListener && passPolicy != game.PassPolicyRamsch {
		http.Error(w, "invalid pass_policy", http.StatusBadRequest)
		return
	}
	gs.MaxGames = req.MaxGames
	gs.PassPolicy = passPolicy

	// Save the session to the database
	if err := s.db.SaveGameSession(game.GameSessionState{
		ID:         gs.SessionID,
		Code:       string(gs.Code),
		GameID:     gs.ID,
		MaxGames:   gs.MaxGames,
		PassPolicy: string(gs.PassPolicy),
	}); err != nil {
		http.Error(w, fmt.Sprintf("failed to save game session: %v", err), http.StatusInternalServerError)
		return
	}

	// Save to database
	go s.cache.SaveGame(*gs)

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
	profile, err := currentProfile(r)
	if err != nil {
		writeAuthRequired(w)
		return
	}
	playerID := profile.ID

	gs, err := s.cache.GetGameByID(gameID)
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
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	playerName := strings.TrimSpace(req.PlayerName)
	if playerName == "" {
		playerName = "Player"
	}

	basicUsername, password, ok := r.BasicAuth()
	if !ok || strings.TrimSpace(basicUsername) == "" || password == "" {
		writeAuthRequired(w)
		return
	}
	if basicUsername != playerName {
		http.Error(w, "basic auth username must match player_name", http.StatusBadRequest)
		return
	}

	if existingProfile, err := s.db.GetProfileByName(playerName); err == nil {
		if existingProfile.IsAgent {
			http.Error(w, "Username is reserved for an agent", http.StatusConflict)
			return
		}
		if existingProfile.PasswordHash == "" {
			passwordHash, err := hashPassword(password)
			if err != nil {
				http.Error(w, "failed to hash password", http.StatusInternalServerError)
				return
			}
			existingProfile.PasswordHash = passwordHash
			if err := s.db.SaveProfile(*existingProfile); err != nil {
				logger.Warning("Failed to set password for existing profile: %e", err)
				http.Error(w, "failed to update player profile", http.StatusInternalServerError)
				return
			}
		} else if !passwordHashMatches(existingProfile.PasswordHash, password) {
			writeAuthRequired(w)
			return
		}
		logger.Info("Returning existing profile %s for %s", existingProfile.ID, existingProfile.Name)
		writeJSON(w, profileToResponse(existingProfile))
		return
	}

	passwordHash, err := hashPassword(password)
	if err != nil {
		http.Error(w, "failed to hash password", http.StatusInternalServerError)
		return
	}

	profile := db.ProfileEntry{
		ID:           uuid.New().String(),
		Name:         playerName,
		PasswordHash: passwordHash,
	}
	logger.Info("Creating new profile %s for %s", profile.ID, profile.Name)
	if err := s.db.SaveProfile(profile); err != nil {
		logger.Warning("Failed to store player profile: %e", err)
		http.Error(w, "failed to store player profile", http.StatusInternalServerError)
		return
	}

	writeJSON(w, profileToResponse(&profile))
}

func (s *Server) handleCurrentProfile(w http.ResponseWriter, r *http.Request) {
	profile, err := currentProfile(r)
	if err != nil {
		writeAuthRequired(w)
		return
	}
	writeJSON(w, profileToResponse(profile))
}

// handleJoinGame adds a human player to a game
func (s *Server) handleJoinGame(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	code := vars["code"]

	profile, err := currentProfile(r)
	if err != nil {
		writeAuthRequired(w)
		return
	}
	playerID := profile.ID

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
	gs, game_err := s.cache.GetGameBySessionCode(code)
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

	go s.cache.SaveGame(*gs)
	s.clients.BroadcastStateChange(gs, response, gs.GetPositionForPlayer(playerID))
	go s.BroadcastAIActions(gs)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"game_id": gs.ID,
	})

	logger.Info("Player %s joined game %s via HTTP", profile.Name, gs.Code)
}

// handleLeaveGame removes a player from a game
func (s *Server) handleLeaveGame(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	gameID := vars["id"]

	playerID, err := currentProfileID(r)
	if err != nil {
		writeAuthRequired(w)
		return
	}

	// Fetch game
	gs, err := s.cache.GetGameByID(gameID)
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

		go s.cache.SaveGame(*gs)
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

		logger.Info("Player %s forfeited game %s", playerID, gs.Code)
	} else {
		// Game hasn't started yet - just remove the player
		gs.Players[position] = nil

		// Remove player from database
		if err := s.db.RemovePlayer(gs.ID, playerID); err != nil {
			logger.Warning("Failed to remove player from database: %e", err)
		}

		// If no players left, delete the game
		if gs.PlayerCount() == 0 {
			if err := s.db.DeleteGame(gs.ID); err != nil {
				logger.Warning("Failed to delete empty game: %e", err)
			}
			logger.Info("All players left game %s, deleted", gs.Code)
		} else {
			// Save the updated game
			go s.cache.SaveGame(*gs)

			// Broadcast to other players that someone left
			s.clients.BroadcastToPlayers(gs, &Message{
				Type: "player_left",
				Data: map[string]any{
					"player_id":   playerID,
					"player_name": playerName,
					"game_id":     gs.ID,
				},
			})

			logger.Info("Player %s left game %s (lobby). Remaining players: %d", playerID, gs.Code, gs.PlayerCount())
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "left"})
}

// handleAddAgent adds an AI agent to a game
func (s *Server) handleAddAgent(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	gameID := vars["id"]

	var req struct {
		AgentID string `json:"agent_id,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	gs, err := s.cache.GetGameByID(gameID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	var agentProfile db.ProfileEntry

	if req.AgentID != "" {
		// Add specific agent
		profile, err := s.db.GetProfile(req.AgentID)
		if err != nil {
			http.Error(w, "agent not found", http.StatusNotFound)
			return
		}
		if !profile.IsAgent {
			http.Error(w, "profile is not an agent", http.StatusBadRequest)
			return
		}
		// Check if agent is already in the game
		for _, player := range gs.Players {
			if player != nil && player.ID == req.AgentID {
				http.Error(w, "agent already in game", http.StatusBadRequest)
				return
			}
		}
		agentProfile = *profile
	} else {
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
		agentProfile = availableProfiles[rand.Int()%len(availableProfiles)]
	}

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

	go s.cache.SaveGame(*gs)
	s.clients.BroadcastStateChange(gs, response, gs.GetPositionForPlayer(agentProfile.ID))
	go s.BroadcastAIActions(gs)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "agent added"})
}

// handleGetPlayerHistory returns a player's game history
func (s *Server) handleGetPlayerHistory(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	playerID := vars["id"]
	if authPlayerID, err := currentProfileID(r); err != nil {
		writeAuthRequired(w)
		return
	} else if playerID != authPlayerID {
		http.Error(w, "cannot access another player's history", http.StatusForbidden)
		return
	}

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

	results, err := s.db.GetPlayerSessionResults(playerID, limit)
	if err != nil {
		logger.Warning("Failed to get player session results: %e", err)
		// Return empty array on error rather than failing
		results = []game.PlayerSessionResultState{}
	}

	// Rating changes are now included in player results
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// handleGetActiveGames returns active games for a player
func (s *Server) handleGetActiveGames(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	playerID := vars["id"]
	if authPlayerID, err := currentProfileID(r); err != nil {
		writeAuthRequired(w)
		return
	} else if playerID != authPlayerID {
		http.Error(w, "cannot access another player's active games", http.StatusForbidden)
		return
	}

	games, err := s.db.GetActiveGamesByPlayer(playerID)
	if err != nil {
		logger.Warning("Failed to get active games: %e", err)
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
		logger.Warning("Failed to get session results: %e", err)
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
	if authPlayerID, err := currentProfileID(r); err != nil {
		writeAuthRequired(w)
		return
	} else if playerID != authPlayerID {
		http.Error(w, "cannot update another player's avatar", http.StatusForbidden)
		return
	}

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
			logger.Warning("Failed to create GCS client: %e", err)
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
			logger.Warning("Failed to upload to GCS: %e", err)
			http.Error(w, "Failed to upload avatar", http.StatusInternalServerError)
			return
		}

		if err := writer.Close(); err != nil {
			logger.Warning("Failed to close GCS writer: %e", err)
			http.Error(w, "Failed to upload avatar", http.StatusInternalServerError)
			return
		}

		// Make the object publicly readable
		if err := obj.ACL().Set(ctx, storage.AllUsers, storage.RoleReader); err != nil {
			logger.Warning("Failed to set ACL: %e", err)
		}

		avatarURL = fmt.Sprintf("https://storage.googleapis.com/%s/%s", gcsBucket, objectPath)
		logger.Info("Avatar uploaded to GCS at %s", avatarURL)
	} else {
		// Store locally in frontend/public/res/avatars
		uploadDir := "./frontend/public/res/avatars"
		if err := os.MkdirAll(uploadDir, 0755); err != nil {
			logger.Warning("Failed to create avatars directory: %e", err)
			http.Error(w, "Failed to save avatar", http.StatusInternalServerError)
			return
		}

		localPath := filepath.Join(uploadDir, filename)
		outFile, err := os.Create(localPath)
		if err != nil {
			logger.Warning("Failed to create file: %e", err)
			http.Error(w, "Failed to save avatar", http.StatusInternalServerError)
			return
		}
		defer outFile.Close()

		if _, err := io.Copy(outFile, file); err != nil {
			logger.Warning("Failed to write file: %e", err)
			http.Error(w, "Failed to save avatar", http.StatusInternalServerError)
			return
		}

		avatarURL = fmt.Sprintf("/res/avatars/%s", filename)
		logger.Info("Avatar saved locally at %s", localPath)
	}

	// Update profile with avatar URL
	profile.ProfileIcon = avatarURL
	if err := s.db.SaveProfile(*profile); err != nil {
		logger.Warning("Failed to update profile: %e", err)
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
	return currentProfileID(r)
}

// GameActionFunc is a function that performs a game action
type GameActionFunc func(gs *game.GameState, playerID string, r *http.Request) (string, error)

// handleGameAction is a common handler for game actions
func (s *Server) handleGameAction(w http.ResponseWriter, r *http.Request, validateTurn bool, action GameActionFunc) {

	vars := mux.Vars(r)
	gameID := vars["id"]

	playerID, err := getAuthenticatedPlayerID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	gs, err := s.cache.GetGameByID(gameID)
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

	go s.cache.SaveGame(*gs)
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

// handleReadyForNext marks a player as ready for the next game
func (s *Server) handleReadyForNext(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	gameID := vars["id"]

	playerID, err := getAuthenticatedPlayerID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	gs, err := s.cache.GetGameByID(gameID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Find the player and mark them as ready
	position := gs.GetPositionForPlayer(playerID)
	if position == -1 {
		http.Error(w, "player not in this game", http.StatusBadRequest)
		return
	}

	player := gs.Players[position]
	player.ReadyForNext = true
	maxGames := gs.MaxGames
	if maxGames <= 0 {
		maxGames = game.DefaultMaxGames
	}
	if gs.GameNumber+1 >= maxGames {
		http.Error(w, "session is complete", http.StatusBadRequest)
		return
	}

	// Check if all human players are ready
	allReady := true
	for _, p := range gs.Players {
		if p != nil && !p.IsAgent && !p.ReadyForNext {
			allReady = false
			break
		}
	}

	// Broadcast the updated state to all players BEFORE checking if all are ready
	s.clients.BroadcastStateChange(gs, fmt.Sprintf("%s is ready for the next game", player.Name), gs.CurrentPlayer)

	go s.cache.SaveGame(*gs)

	// If all human players are ready, automatically start the next game
	if allReady {
		response, err := gs.NextGame()
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		newGameID := gs.ID
		go s.cache.SaveGame(*gs)

		// Send start_next_game message to trigger navigation
		s.clients.BroadcastToPlayers(gs, &Message{
			Type: "start_next_game",
			Data: map[string]any{"game_id": newGameID},
		})

		// Also broadcast the state change
		s.clients.BroadcastStateChange(gs, response, gs.CurrentPlayer)
		go s.BroadcastAIActions(gs)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "game_started",
			"game_id": newGameID,
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
}

func (s *Server) handleTimeout(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	gameID := vars["id"]

	_, err := getAuthenticatedPlayerID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	gs, err := s.cache.GetGameByID(gameID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Check if game is already complete
	if gs.Phase == game.PhaseComplete {
		// Game is complete, just remove the player instead of forfeiting
		// Find the player who timed out
		var timeoutPlayerID string

		// Try to identify the player from the deadline or request
		if gs.CurrentPlayerDeadline != "" {
			currentPlayer := gs.GetCurrentPlayer()
			if currentPlayer != nil {
				timeoutPlayerID = currentPlayer.ID
			}
		}

		if timeoutPlayerID != "" {
			position := gs.GetPositionForPlayer(timeoutPlayerID)
			if position != -1 {
				gs.Players[position] = nil
				gs.CurrentPlayerDeadline = ""

				// Remove player from database
				if err := s.db.RemovePlayer(gs.ID, timeoutPlayerID); err != nil {
					logger.Warning("Failed to remove inactive player from completed game: %e", err)
				}
				// Save the updated game state
				go s.cache.SaveGame(*gs)

				logger.Info("Removed inactive player %s from completed game %s (client-reported)", timeoutPlayerID, gs.Code)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "left"})
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

	logger.Info("Game %s timeout reported by client for %s", gs.Code, currentPlayer.Name)

	// Forfeit the game
	gs.ForfeitDueToInactivity()
	logger.Info("Set game %s phase to complete", gs.Code)

	// Save the updated game state
	go func() {
		if err := s.cache.SaveGame(*gs); err != nil {
			logger.Warning("Failed to save timeout forfeit game: %e", err)
		}
		if err := s.maybeSaveGameResults(gs); err != nil {
			logger.Warning("Failed to save timeout forfeit results: %e", err)
		}
	}()

	// Broadcast the updated game state so clients show game over screen
	timeoutMsg := fmt.Sprintf("%s was inactive for 2 minutes and has forfeited the game", currentPlayer.Name)
	s.clients.BroadcastStateChange(gs, timeoutMsg, gs.CurrentPlayer)

	logger.Info("Game %s forfeited due to timeout from %s (client-reported)", gs.Code, currentPlayer.Name)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleGetPlayerRating returns the rating for a player
func (s *Server) handleGetPlayerRating(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	playerID := vars["id"]
	if authPlayerID, err := currentProfileID(r); err != nil {
		writeAuthRequired(w)
		return
	} else if playerID != authPlayerID {
		http.Error(w, "cannot access another player's rating", http.StatusForbidden)
		return
	}

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

	results, err := s.db.GetPlayerSessionResults(playerID, 200)
	if err != nil {
		logger.Warning("Failed to get player session results: %e", err)
		// Return empty array on error rather than failing
		results = []game.PlayerSessionResultState{}
	}
	timeline := []int{}
	for i := len(results) - 1; i >= 0; i-- {
		r := results[i]
		if r.RatingBefore > 0 {
			timeline = append(timeline, r.RatingBefore)
		}
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
		"timeline":     timeline,
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

// handleListAgents returns all available agent profiles with their configurations
func (s *Server) handleListAgents(w http.ResponseWriter, r *http.Request) {
	agents, err := s.db.ListAgentProfiles()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type AgentInfo struct {
		ID               string  `json:"id"`
		Name             string  `json:"name"`
		ProfileIcon      string  `json:"profile_icon"`
		BiddingType      string  `json:"bidding_type"`
		BiddingThreshold float64 `json:"bidding_threshold"`
		GameChoiceType   string  `json:"game_choice_type"`
		CardPlayType     string  `json:"card_play_type"`
		MCTSSimulations  int     `json:"mcts_simulations,omitempty"`
	}

	agentInfos := make([]AgentInfo, 0)
	for _, agent := range agents {
		config, err := s.db.GetAgentConfig(agent.ID)
		if err != nil {
			logger.Warning("Failed to get agent config: %e", err)
			continue
		}

		mctsSimulations := 0
		if config.MCTSSimulations != nil {
			mctsSimulations = *config.MCTSSimulations
		}

		agentInfos = append(agentInfos, AgentInfo{
			ID:               agent.ID,
			Name:             agent.Name,
			ProfileIcon:      agent.ProfileIcon,
			BiddingType:      config.BiddingType,
			BiddingThreshold: config.BiddingThreshold,
			GameChoiceType:   config.GameChoiceType,
			CardPlayType:     config.CardPlayType,
			MCTSSimulations:  mctsSimulations,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(agentInfos)
}
