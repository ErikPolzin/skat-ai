package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

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
