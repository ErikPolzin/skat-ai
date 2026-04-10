package server

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for now
	},
}

// Server manages all game sessions and client connections
type Server struct {
	games      map[string]*GameSession
	codeToID   map[string]string // Map game codes to game IDs
	gamesMutex sync.RWMutex
	db         *Database
	clients    *ClientManager // Centralized client management
}

func NewServer(db *Database) *Server {
	return &Server{
		games:    make(map[string]*GameSession),
		codeToID: make(map[string]string),
		db:       db,
		clients:  NewClientManager(),
	}
}

// HandleWebSocket upgrades HTTP connections to WebSocket
func (s *Server) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	// Profile ID should be provided as query parameter
	profileID := r.URL.Query().Get("profile_id")
	if profileID == "" {
		log.Printf("WebSocket connection without profile_id")
		conn.Close()
		return
	}

	// Register or update client in ClientManager
	client := s.clients.RegisterClient(profileID, conn)

	go client.readPump(s)
	go client.writePump()
}

// generateGameCode creates a unique 4-character game code
func (s *Server) generateGameCode() string {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	for attempts := 0; attempts < 100; attempts++ {
		code := ""
		for i := 0; i < 4; i++ {
			code += string(chars[rand.Intn(len(chars))])
		}

		// Check if code is already in use
		if _, exists := s.codeToID[code]; !exists {
			return code
		}
	}

	// Fallback to longer code if we can't find a unique 4-char code
	return fmt.Sprintf("%08X", rand.Uint32())
}

// CreateGame creates a new game session
func (s *Server) CreateGame(gameID string) error {
	s.gamesMutex.Lock()
	defer s.gamesMutex.Unlock()

	if _, exists := s.games[gameID]; exists {
		return fmt.Errorf("game %s already exists", gameID)
	}

	// Generate a unique game code
	code := s.generateGameCode()

	game := NewGame(gameID, code, s)
	s.games[gameID] = game
	s.codeToID[code] = gameID

	// Persist to database if available
	if s.db != nil {
		if err := s.db.SaveGame(game); err != nil {
			log.Printf("Failed to save game to database: %v", err)
		}
	}

	log.Printf("Created game: %s with code: %s", gameID, code)
	return nil
}

// GetGame retrieves a game session by ID
func (s *Server) GetGame(gameID string) (*GameSession, error) {
	s.gamesMutex.RLock()
	defer s.gamesMutex.RUnlock()

	game, exists := s.games[gameID]
	if !exists {
		return nil, fmt.Errorf("game %s not found", gameID)
	}
	return game, nil
}

// GetGameByCode retrieves a game session by its code
func (s *Server) GetGameByCode(code string) (*GameSession, error) {
	s.gamesMutex.RLock()
	defer s.gamesMutex.RUnlock()

	gameID, exists := s.codeToID[code]
	if !exists {
		return nil, fmt.Errorf("game with code %s not found", code)
	}

	game, exists := s.games[gameID]
	if !exists {
		return nil, fmt.Errorf("game %s not found", gameID)
	}
	return game, nil
}

// ListGames returns all available games
func (s *Server) ListGames(open bool) []*GameInfo {
	s.gamesMutex.RLock()
	defer s.gamesMutex.RUnlock()

	games := make([]*GameInfo, 0, len(s.games))
	for _, game := range s.games {
		if 3 != len(game.Players) || !open {
			games = append(games, game.GetInfo("")) // No player-specific info for list
		}
	}
	return games
}

// RemoveGame deletes a game
func (s *Server) RemoveGame(gameID string) {
	s.gamesMutex.Lock()
	defer s.gamesMutex.Unlock()

	delete(s.games, gameID)

	// Remove from database if available
	if s.db != nil {
		if err := s.db.DeleteGame(gameID); err != nil {
			log.Printf("Failed to delete game from database: %v", err)
		}
	}

	log.Printf("Removed game: %s", gameID)
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
		// Remove client from ClientManager
		s.clients.RemoveClient(c.profileID)
	}()

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			log.Printf("Read error: %v", err)
			break
		}

		var msg Message
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("JSON unmarshal error: %v", err)
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
			log.Printf("Write error: %v", err)
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
