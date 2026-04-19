package server

import (
	"fmt"
	"sync"

	"skat/logger"

	"github.com/gorilla/websocket"
)

// ClientManager manages all connected clients by profile ID
type ClientManager struct {
	clients map[string]*Client // profileID -> Client
	mutex   sync.RWMutex
}

// NewClientManager creates a new client manager
func NewClientManager() *ClientManager {
	return &ClientManager{
		clients: make(map[string]*Client),
	}
}

// RegisterClient registers or updates a client connection for a profile
func (cm *ClientManager) RegisterClient(profileID string, conn *websocket.Conn) *Client {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	// Check if client already exists
	if existingClient, exists := cm.clients[profileID]; exists {
		// Close old connection if it exists
		if existingClient.conn != nil {
			logger.Info("Closing existing connection for profile", "profile_id", profileID)
			existingClient.conn.Close()
		}
		// Update connection
		existingClient.conn = conn
		existingClient.send = make(chan []byte, 256)
		logger.Info("Updated connection for profile", "profile_id", profileID)
		return existingClient
	}

	// Create new client
	client := &Client{
		profileID: profileID,
		conn:      conn,
		send:      make(chan []byte, 256),
	}
	cm.clients[profileID] = client
	logger.Info("Registered new client for profile", "profile_id", profileID)
	return client
}

// GetClient retrieves a client by profile ID
func (cm *ClientManager) GetClient(profileID string) (*Client, bool) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	client, exists := cm.clients[profileID]
	return client, exists
}

// RemoveClient removes a client from the manager
func (cm *ClientManager) RemoveClient(profileID string) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if client, exists := cm.clients[profileID]; exists {
		close(client.send)
		delete(cm.clients, profileID)
		logger.Info("Removed client for profile", "profile_id", profileID)
	}
}

// BroadcastToClients sends a message to multiple clients by profile ID
func (cm *ClientManager) BroadcastToClients(profileIDs []string, msg *Message) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	for _, profileID := range profileIDs {
		if client, exists := cm.clients[profileID]; exists {
			if err := client.SendMessage(msg); err != nil {
				logger.Warning("Failed to send message to profile", "profile_id", profileID, "error", err)
			}
		}
	}
}

// IsOnline checks if a profile is currently connected
func (cm *ClientManager) IsOnline(profileID string) bool {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	_, exists := cm.clients[profileID]
	return exists
}

// GetOnlineCount returns the number of connected clients
func (cm *ClientManager) GetOnlineCount() int {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	return len(cm.clients)
}

// SendToClient sends a message to a specific client
func (cm *ClientManager) SendToClient(profileID string, msg *Message) error {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	client, exists := cm.clients[profileID]
	if !exists {
		return fmt.Errorf("client not found for profile %s", profileID)
	}

	return client.SendMessage(msg)
}

// GetOnlinePlayerIDs returns a list of all currently connected profile IDs
func (cm *ClientManager) GetOnlinePlayerIDs() []string {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	playerIDs := make([]string, 0, len(cm.clients))
	for profileID := range cm.clients {
		playerIDs = append(playerIDs, profileID)
	}
	return playerIDs
}
