package server

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"skat/agent"
	"skat/game"
	"skat/logger"
	"skat/server/db"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for now
	},
}

// Server manages all game sessions and client connections
type Server struct {
	db      db.Database
	cache   *Cache
	clients *ClientManager // Centralized client management
}

func NewServer(database db.Database) *Server {
	server := &Server{
		db:      database,
		cache:   NewCache(database),
		clients: NewClientManager(database),
	}

	// Set up agent config loader with error recovery
	agent.SetAgentConfigLoader(func(profileID string) (result *agent.AgentConfigData, err error) {
		// Recover from panics in config loading
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic loading agent config for %s: %v", profileID, r)
			}
		}()

		config, err := database.GetAgentConfig(profileID)
		if err != nil {
			return nil, err
		}

		// Convert db.AgentConfig to agent.AgentConfigData
		mctsSimulations := 0
		if config.MCTSSimulations != nil {
			mctsSimulations = *config.MCTSSimulations
		}

		cardplayWeightsPath := ""
		if config.CardplayWeightsPath != nil {
			cardplayWeightsPath = *config.CardplayWeightsPath
		}

		return &agent.AgentConfigData{
			ProfileID:           config.ProfileID,
			BiddingType:         config.BiddingType,
			BiddingThreshold:    config.BiddingThreshold,
			GameChoiceType:      config.GameChoiceType,
			CardPlayType:        config.CardPlayType,
			MCTSSimulations:     mctsSimulations,
			CardplayWeightsPath: cardplayWeightsPath,
		}, nil
	})

	return server
}

// IsCloudRun returns true if running on Google Cloud Run
// Cloud Run automatically sets K_SERVICE environment variable
func (s *Server) IsCloudRun() bool {
	return os.Getenv("K_SERVICE") != ""
}

// HandleWebSocket upgrades HTTP connections to WebSocket
func (s *Server) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	username, password, ok := r.BasicAuth()
	if !ok {
		username, password, ok = webSocketBasicAuth(r)
	}
	if !ok {
		writeAuthRequired(w)
		return
	}

	profile, authErr := s.authenticateProfile(username, password)
	if authErr != nil {
		logger.Warning("WebSocket authentication failed for %s", username)
		writeAuthRequired(w)
		return
	}
	profileID := profile.ID

	// Register or update client in ClientManager
	responseHeader := http.Header{}
	if webSocketRequestedProtocol(r, "skat-auth") {
		responseHeader.Set("Sec-WebSocket-Protocol", "skat-auth")
	}
	conn, err := upgrader.Upgrade(w, r, responseHeader)
	if err != nil {
		logger.Error("WebSocket upgrade failed: %e", err)
		return
	}

	client := s.clients.RegisterClient(profileID, conn)

	go client.readPump(s)
	go client.writePump()
}

func webSocketBasicAuth(r *http.Request) (string, string, bool) {
	for _, protocol := range websocket.Subprotocols(r) {
		token, ok := strings.CutPrefix(protocol, "auth.")
		if !ok {
			continue
		}
		decoded, err := base64.RawURLEncoding.DecodeString(token)
		if err != nil {
			return "", "", false
		}
		username, password, ok := strings.Cut(string(decoded), ":")
		return username, password, ok
	}
	return "", "", false
}

func webSocketRequestedProtocol(r *http.Request, expected string) bool {
	for _, protocol := range websocket.Subprotocols(r) {
		if protocol == expected {
			return true
		}
	}
	return false
}

// Client represents a connected player
type Client struct {
	conn      *websocket.Conn
	send      chan []byte
	profileID string // Primary identifier (profile ID)
}

// readPump reads messages from the WebSocket connection
func (c *Client) readPump(s *Server) {
	defer func() {
		c.conn.Close()

		// Notify other players in games with this player
		s.notifyPlayerOffline(c.profileID)

		// Remove client from ClientManager
		s.clients.RemoveClient(c.profileID)
	}()

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			logger.Debug("WebSocket read error: %e", err)
			break
		}

		var msg Message
		if err := json.Unmarshal(message, &msg); err != nil {
			logger.Warning("JSON unmarshal error: %e", err)
			continue
		}

		s.handleMessage(c, &msg)
	}
}

// writePump writes messages to the WebSocket connection
func (c *Client) writePump() {
	defer c.conn.Close()

	for message := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
			logger.Debug("WebSocket write error: %e", err)
			break
		}
	}
}

// SendMessage sends a message to the client
func (c *Client) SendMessage(msg *Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	select {
	case c.send <- data:
		return nil
	default:
		return fmt.Errorf("client send channel full")
	}
}

// notifyPlayerOffline broadcasts to other players in a game that a player has gone offline
func (s *Server) notifyPlayerOffline(profileID string) {
	// Find all active games this player is in
	games, err := s.db.GetActiveGamesByPlayer(profileID)
	if err != nil {
		logger.Warning("Error finding games for offline player: %e", err)
		return
	}

	// Notify other players in each game
	for _, gs := range games {
		// Get profile info for the offline player
		profile, err := s.db.GetProfile(profileID)
		if err != nil {
			logger.Warning("Error getting profile for offline player: %e", err)
			continue
		}

		// Broadcast to other players in the game
		for _, player := range gs.Players {
			if player != nil && !player.IsAgent && player.ID != profileID {
				s.clients.SendToClient(player.ID, &Message{
					Type: "player_offline",
					Data: map[string]any{
						"player_id":   profileID,
						"player_name": profile.Name,
						"game_id":     gs.ID,
					},
				})
			}
		}
	}
}

// StartCleanupTask starts a background task that periodically cleans up stale games and checks for timeouts
func (s *Server) StartCleanupTask(intervalMinutes int, inactiveMinutes int) {
	ticker := time.NewTicker(time.Duration(intervalMinutes) * time.Minute)
	go func() {
		for range ticker.C {
			s.cleanupStaleGames(inactiveMinutes)
			s.checkInactivityTimeouts()
		}
	}()
	logger.Info("Started cleanup task (includes timeout checking)")
}

// checkInactivityTimeouts checks all active games for inactivity and forfeits if needed
func (s *Server) checkInactivityTimeouts() {
	games, err := s.db.GetAllExpiredGames()
	if err != nil {
		logger.Warning("Failed to get expired games for timeout check: %e", err)
		return
	}

	for _, gs := range games {
		if gs.IsDeadlinePassed() {
			currentPlayer := gs.GetCurrentPlayer()
			if currentPlayer == nil {
				continue
			}

			logger.Info("Game %s timed out - player %s deadline passed", gs.Code, currentPlayer.ID)

			// If game is already complete, just remove the player instead of forfeiting
			if gs.Phase == game.PhaseComplete {
				// Find the player's position
				position := gs.GetPositionForPlayer(currentPlayer.ID)
				if position != -1 {
					gs.Players[position] = nil
					// Clear the deadline since we're removing the player
					gs.CurrentPlayerDeadline = ""

					// Remove player from database
					if err := s.db.RemovePlayer(gs.ID, currentPlayer.ID); err != nil {
						logger.Warning("Failed to remove inactive player from completed game: %e", err)
					}

					// Save the updated game state
					if err := s.cache.SaveGame(gs); err != nil {
						logger.Warning("Failed to save game after removing inactive player: %e", err)
					}

					logger.Info("Removed inactive player from completed game %s", gs.Code)
				}
				continue
			}

			// Forfeit the game
			gs.ForfeitDueToInactivity()

			// Save the updated game state
			if err := s.cache.SaveGame(gs); err != nil {
				logger.Warning("Failed to save game after timeout: %e", err)
			}
			if err := s.maybeSaveGameResults(&gs); err != nil {
				logger.Warning("Failed to save timeout forfeit results: %e", err)
			}

			// Broadcast the updated game state so clients show game over screen
			timeoutMsg := fmt.Sprintf("%s was inactive for 2 minutes and has forfeited the game", currentPlayer.Name)
			s.clients.BroadcastStateChange(&gs, timeoutMsg, gs.CurrentPlayer)

			logger.Info("Game %s forfeited due to timeout from %s", gs.Code, currentPlayer.ID)
		}
	}
}

// cleanupStaleGames removes games that have been inactive and have no online human players
func (s *Server) cleanupStaleGames(inactiveMinutes int) {
	// Get list of online player IDs
	onlinePlayerIDs := s.clients.GetOnlinePlayerIDs()

	// Call database cleanup
	deleted, err := s.db.CleanupStaleGames(inactiveMinutes, onlinePlayerIDs)
	if err != nil {
		logger.Error("Error during cleanup: %s", err)
		return
	}

	if deleted > 0 {
		logger.Info("Cleaned up %d stale games", deleted)
	}
}
