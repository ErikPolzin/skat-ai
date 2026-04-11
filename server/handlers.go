package server

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

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
	api.HandleFunc("/games", s.handleListGames).Methods("GET")
	api.HandleFunc("/games", s.handleCreateGame).Methods("POST")
	api.HandleFunc("/games/{id}", s.handleGetGame).Methods("GET")
	api.HandleFunc("/games/{id}/join", s.handleJoinGame).Methods("POST")
	api.HandleFunc("/games/{id}/agents", s.handleAddAgent).Methods("POST")
	api.HandleFunc("/games/{id}/start", s.handleStartGame).Methods("POST")
	api.HandleFunc("/profiles", s.handleCreateProfile).Methods("POST")
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
	allowedOrigins := []string{"http://localhost:3000", "http://192.168.1.125:3000"}

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

// handleListGames returns all available games
func (s *Server) handleListGames(w http.ResponseWriter, r *http.Request) {
	open := r.URL.Query().Get("open") == "true"
	games := s.ListGames(open)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(games)
}

// handleCreateGame creates a new game session
func (s *Server) handleCreateGame(w http.ResponseWriter, r *http.Request) {
	gameID := uuid.New().String()[:8]
	if err := s.CreateGame(gameID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get the game to retrieve the code
	game, _ := s.GetGame(gameID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"game_id": gameID,
		"code":    game.Code,
	})
}

// handleGetGame returns game information
func (s *Server) handleGetGame(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	gameID := vars["id"]
	playerID := r.URL.Query().Get("player_id")

	game, err := s.GetGame(gameID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(game.GetInfo(playerID))
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

	// If no player ID provided, generate a new one
	if playerID == "" {
		playerID = uuid.New().String()
		log.Printf("Creating new profile: %s for %s", playerID, playerName)

		// Store the new profile if database is available
		if s.db != nil {
			if err := s.db.SetPlayerProfile(playerID, playerName); err != nil {
				log.Printf("Failed to store player profile: %v", err)
			}
		}
	} else {
		// Existing player ID - retrieve or update name
		if s.db != nil {
			if storedName, err := s.db.GetPlayerProfile(playerID); err == nil {
				// Profile exists, optionally update name if different
				if storedName != playerName && playerName != "" {
					if err := s.db.SetPlayerProfile(playerID, playerName); err != nil {
						log.Printf("Failed to update player profile: %v", err)
					}
				} else if storedName != "" {
					// Use the stored name if no new name provided
					playerName = storedName
				}
				log.Printf("Retrieved profile: %s (%s)", playerID, playerName)
			} else {
				// Profile doesn't exist, create it
				if err := s.db.SetPlayerProfile(playerID, playerName); err != nil {
					log.Printf("Failed to store player profile: %v", err)
				}
				log.Printf("Created profile for existing ID: %s (%s)", playerID, playerName)
			}
		}
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
	gameID := vars["id"]

	var req struct {
		PlayerID   string `json:"player_id,omitempty"`
		PlayerName string `json:"player_name,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Try to get the game - first check if gameID might be a code
	var game *GameSession
	var err error

	// Check if it looks like a game code (4-5 uppercase chars/numbers)
	if len(gameID) <= 5 {
		// Try treating it as a code first
		game, err = s.GetGameByCode(gameID)
		if err != nil {
			// If not found as code, try as regular ID
			game, err = s.GetGame(gameID)
		} else {
			// Update gameID to the real ID for the response
			gameID = game.ID
		}
	} else {
		// Longer strings are treated as game IDs
		game, err = s.GetGame(gameID)
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	playerID := req.PlayerID
	playerName := req.PlayerName

	// Generate player ID if not provided
	if playerID == "" {
		playerID = uuid.New().String()
		log.Printf("Generated new player ID: %s for %s", playerID, playerName)
	}

	if s.db != nil {
		if req.PlayerID != "" {
			// Existing player - retrieve stored name
			if storedName, err := s.db.GetPlayerProfile(playerID); err == nil {
				playerName = storedName
				log.Printf("Returning player: %s (%s)", playerID, storedName)
			}
		} else {
			// New player - store profile
			if err := s.db.SetPlayerProfile(playerID, playerName); err != nil {
				log.Printf("Failed to store player profile: %v", err)
			}
		}
	}

	// Actually add the player to the game
	if err := game.AddPlayer(playerID, playerName); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"player_id":   playerID,
		"player_name": playerName,
		"game_id":     gameID,
	})

	log.Printf("Player %s (%s) joined game %s via HTTP", playerID, playerName, gameID)
}

// handleAddAgent adds an AI agent to a game
func (s *Server) handleAddAgent(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	gameID := vars["id"]

	var req struct {
		AgentType string `json:"agent_type"`
		AgentName string `json:"agent_name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	game, err := s.GetGame(gameID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if req.AgentName == "" {
		req.AgentName = game.getRandomAgentName()
	}

	if err := game.AddAgent(req.AgentType, req.AgentName); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "agent added"})
}

// handleStartGame starts a game
func (s *Server) handleStartGame(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	gameID := vars["id"]

	game, err := s.GetGame(gameID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if err := game.StartGame(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "game started"})
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
		if err != nil || limit < 1 || limit > 50 {
			limit = 10
		}
	}

	// If no database, return empty array
	if s.db == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]*GameHistoryEntry{})
		return
	}

	history, err := s.db.GetPlayerGameHistory(playerID, limit)
	if err != nil {
		log.Printf("Failed to get player history: %v", err)
		// Return empty array on error rather than failing
		history = []GameHistoryEntry{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(history)
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
