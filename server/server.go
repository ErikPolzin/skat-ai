package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

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
	clients *ClientManager // Centralized client management
}

func NewServer(database db.Database) *Server {
	return &Server{
		db:      database,
		clients: NewClientManager(),
	}
}

// HandleWebSocket upgrades HTTP connections to WebSocket
func (s *Server) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error("WebSocket upgrade failed", "error", err)
		return
	}

	// Profile ID should be provided as query parameter
	profileID := r.URL.Query().Get("profile_id")
	if profileID == "" {
		logger.Warning("WebSocket connection without profile_id")
		conn.Close()
		return
	}

	// Register or update client in ClientManager
	client := s.clients.RegisterClient(profileID, conn)

	go client.readPump(s)
	go client.writePump()
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
			logger.Debug("WebSocket read error", "error", err, "profile_id", c.profileID)
			break
		}

		var msg Message
		if err := json.Unmarshal(message, &msg); err != nil {
			logger.Warning("JSON unmarshal error", "error", err, "profile_id", c.profileID)
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
			logger.Debug("WebSocket write error", "error", err, "profile_id", c.profileID)
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
		logger.Warning("Error finding games for offline player", "profile_id", profileID, "error", err)
		return
	}

	// Notify other players in each game
	for _, gs := range games {
		// Get profile info for the offline player
		profile, err := s.db.GetProfile(profileID)
		if err != nil {
			logger.Warning("Error getting profile for offline player", "profile_id", profileID, "error", err)
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

	logger.Info("Player went offline", "profile_id", profileID, "games_notified", len(games))
}

// StartCleanupTask starts a background task that periodically cleans up stale games
func (s *Server) StartCleanupTask(intervalMinutes int, inactiveMinutes int) {
	ticker := time.NewTicker(time.Duration(intervalMinutes) * time.Minute)
	go func() {
		for range ticker.C {
			s.cleanupStaleGames(inactiveMinutes)
		}
	}()
	logger.Info("Started cleanup task", "check_interval_minutes", intervalMinutes, "inactive_threshold_minutes", inactiveMinutes)
}

// cleanupStaleGames removes games that have been inactive and have no online human players
func (s *Server) cleanupStaleGames(inactiveMinutes int) {
	// Get list of online player IDs
	onlinePlayerIDs := s.clients.GetOnlinePlayerIDs()

	// Call database cleanup
	deleted, err := s.db.CleanupStaleGames(inactiveMinutes, onlinePlayerIDs)
	if err != nil {
		logger.Error("Error during cleanup", "error", err)
		return
	}

	if deleted > 0 {
		logger.Info("Cleaned up stale games", "count", deleted)
	}
}
