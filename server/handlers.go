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
	api.HandleFunc("/games/{id}/agents", s.handleAddAgent).Methods("POST")
	api.HandleFunc("/games/{id}/results", s.handleGetSessionResults).Methods("GET")
	api.HandleFunc("/games/{id}/deal", s.handleDeal).Methods("POST")
	api.HandleFunc("/games/{id}/play_card", s.handlePlayCard).Methods("POST")
	api.HandleFunc("/games/{id}/bid", s.handleBid).Methods("POST")
	api.HandleFunc("/games/{id}/choose_game", s.handleChooseGame).Methods("POST")
	api.HandleFunc("/games/{id}/skat_decision", s.handleSkatDecision).Methods("POST")
	api.HandleFunc("/games/{id}/discard_cards", s.handleDiscardCards).Methods("POST")
	api.HandleFunc("/games/{id}/start_next_game", s.handleStartNextGame).Methods("POST")
	api.HandleFunc("/profiles", s.handleCreateProfile).Methods("POST")
	api.HandleFunc("/profiles/{id}/avatar", s.handleUploadAvatar).Methods("POST")
	api.HandleFunc("/players/{id}/history", s.handleGetPlayerHistory).Methods("GET")

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
func (s *Server) handleListOpenSessions(w http.ResponseWriter, r *http.Request) {
	games, err := s.db.ListOpenSessions()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(games)
}

// handleCreateGame creates a new game session
func (s *Server) handleCreateGame(w http.ResponseWriter, r *http.Request) {
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// handleGetSessionResults returns all game results for a session
func (s *Server) handleGetSessionResults(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["id"]

	playerResults, err := s.db.GetSessionResults(sessionID)
	if err != nil {
		logger.Warning("Failed to get session results", "session_id", sessionID, "error", err)
		http.Error(w, fmt.Sprintf("Failed to get session results: %v", err), http.StatusInternalServerError)
		return
	}

	// Group results by game
	gameResultsMap := make(map[string]map[string]interface{})
	for _, pr := range playerResults {
		if _, exists := gameResultsMap[pr.GameID]; !exists {
			// Fetch game to get additional info
			gs, err := s.db.GetGameByID(pr.GameID)
			if err != nil {
				logger.Warning("Failed to get game", "game_id", pr.GameID, "error", err)
				continue
			}

			declarer := gs.Players[gs.Declarer]
			declarerWon, _, _ := gs.GetGameResult()

			gameResultsMap[pr.GameID] = map[string]interface{}{
				"game_id":        pr.GameID,
				"game_number":    gs.GameNumber,
				"declarer_name":  declarer.Name,
				"declarer_won":   declarerWon,
				"game_mode":      string(gs.Mode),
				"trump_suit":     gs.TrumpSuit.String(),
				"player_results": make(map[string]int),
				"player_names":   make(map[string]string),
			}

			// Add all player names
			for _, player := range gs.Players {
				if player != nil {
					gameResultsMap[pr.GameID]["player_names"].(map[string]string)[player.ID] = player.Name
				}
			}
		}

		// Add player points
		gameResultsMap[pr.GameID]["player_results"].(map[string]int)[pr.PlayerID] = pr.PlayerPoints
	}

	// Convert map to slice
	var results []map[string]interface{}
	for _, result := range gameResultsMap {
		results = append(results, result)
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
			Mode  string `json:"mode"`
			Trump string `json:"trump"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return "", err
		}

		mode := game.GameMode(req.Mode)
		trump, err := game.ParseSuit(req.Trump)
		if err != nil {
			return "", fmt.Errorf("invalid trump suit: %v", err)
		}

		return gs.DeclareGame(mode, trump)
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
