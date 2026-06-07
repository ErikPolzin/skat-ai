package server

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"skat/logger"
	cachepkg "skat/server/cache"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type ClientMessageEnvelope struct {
	NodeID     string   `json:"node_id"`
	ProfileIDs []string `json:"profile_ids"`
	Message    Message  `json:"message"`
}

type ClientManagerOption func(*ClientManager)

func WithClientBackend(backend cachepkg.ClientPresenceStore) ClientManagerOption {
	return func(cm *ClientManager) {
		cm.backend = backend
		if bus, ok := backend.(cachepkg.ClientMessageBus); ok {
			cm.messageBus = bus
		}
	}
}

// ClientManager manages all connected clients by profile ID
type ClientManager struct {
	clients    map[string]*Client // profileID -> Client
	mutex      sync.RWMutex
	nodeID     string
	backend    cachepkg.ClientPresenceStore
	messageBus cachepkg.ClientMessageBus
}

// NewClientManager creates a new client manager
func NewClientManager(opts ...ClientManagerOption) *ClientManager {
	cm := &ClientManager{
		clients: make(map[string]*Client),
		nodeID:  uuid.NewString(),
	}
	for _, opt := range opts {
		opt(cm)
	}
	return cm
}

func (cm *ClientManager) StartMessageBus(ctx context.Context) {
	if cm.messageBus == nil {
		return
	}
	messages, err := cm.messageBus.SubscribeClientMessages(ctx)
	if err != nil {
		logger.Warning("Failed to subscribe to client message bus: %e", err)
		return
	}
	go func() {
		for payload := range messages {
			var envelope ClientMessageEnvelope
			if err := json.Unmarshal(payload, &envelope); err != nil {
				logger.Warning("Failed to decode client message envelope: %e", err)
				continue
			}
			if envelope.NodeID == cm.nodeID {
				continue
			}
			cm.deliverToLocalClients(envelope.ProfileIDs, &envelope.Message)
		}
	}()
}

func (cm *ClientManager) StartPresenceHeartbeat(ctx context.Context) {
	if cm.backend == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				for _, profileID := range cm.localProfileIDs() {
					cm.updatePresence(profileID, true)
				}
			}
		}
	}()
}

func (cm *ClientManager) localProfileIDs() []string {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	profileIDs := make([]string, 0, len(cm.clients))
	for profileID := range cm.clients {
		profileIDs = append(profileIDs, profileID)
	}
	return profileIDs
}

// RegisterClient registers or updates a client connection for a profile
func (cm *ClientManager) RegisterClient(profileID string, conn *websocket.Conn) *Client {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	// Check if client already exists
	if existingClient, exists := cm.clients[profileID]; exists {
		// Close old connection if it exists
		if existingClient.conn != nil {
			logger.Info("Closing existing connection for profile")
			closeHandler := existingClient.conn.CloseHandler()
			closeHandler(1000, "closed existing connection")
		}
		// Update connection
		existingClient.conn = conn
		existingClient.send = make(chan []byte, 256)

		cm.updatePresence(profileID, true)

		return existingClient
	}

	// Create new client
	client := &Client{
		profileID: profileID,
		conn:      conn,
		send:      make(chan []byte, 256),
	}
	cm.clients[profileID] = client
	logger.Info("Player %s connected at %s", profileID, conn.RemoteAddr())

	cm.updatePresence(profileID, true)

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
		logger.Info("Removed client for profile %s", profileID)

		cm.updatePresence(profileID, false)
	}
}

// BroadcastToClients sends a message to multiple clients by profile ID
func (cm *ClientManager) BroadcastToClients(profileIDs []string, msg *Message) {
	cm.deliverToLocalClients(profileIDs, msg)
	if err := cm.publishToClients(profileIDs, msg); err != nil {
		logger.Warning("Failed to publish client broadcast: %e", err)
	}
}

func (cm *ClientManager) deliverToLocalClients(profileIDs []string, msg *Message) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	for _, profileID := range profileIDs {
		if client, exists := cm.clients[profileID]; exists {
			if err := client.SendMessage(msg); err != nil {
				logger.Warning("Failed to send message to profile %s: %v", profileID, err)
			}
		}
	}
}

// IsOnline checks if a profile is currently connected
func (cm *ClientManager) IsOnline(profileID string) bool {
	cm.mutex.RLock()
	_, exists := cm.clients[profileID]
	cm.mutex.RUnlock()
	if exists {
		return true
	}
	if cm.backend == nil {
		return false
	}
	online, err := cm.backend.IsOnline(context.Background(), profileID)
	if err != nil {
		logger.Warning("Failed to check distributed presence for %s: %e", profileID, err)
		return false
	}
	return online
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
	client, exists := cm.clients[profileID]
	cm.mutex.RUnlock()
	if !exists {
		if cm.messageBus != nil {
			return cm.publishToClients([]string{profileID}, msg)
		}
		return fmt.Errorf("client not found for profile %s", profileID)
	}

	return client.SendMessage(msg)
}

// GetOnlinePlayerIDs returns a list of all currently connected profile IDs
func (cm *ClientManager) GetOnlinePlayerIDs() []string {
	cm.mutex.RLock()
	playerIDs := make([]string, 0, len(cm.clients))
	for profileID := range cm.clients {
		playerIDs = append(playerIDs, profileID)
	}
	cm.mutex.RUnlock()
	if cm.backend == nil {
		return playerIDs
	}
	ids, err := cm.backend.OnlineIDs(context.Background())
	if err != nil {
		logger.Warning("Failed to list distributed online players: %e", err)
		return playerIDs
	}
	seen := make(map[string]bool, len(playerIDs)+len(ids))
	merged := make([]string, 0, len(playerIDs)+len(ids))
	for _, id := range append(playerIDs, ids...) {
		if seen[id] {
			continue
		}
		seen[id] = true
		merged = append(merged, id)
	}
	return merged
}

func (cm *ClientManager) publishToClients(profileIDs []string, msg *Message) error {
	if cm.messageBus == nil {
		return nil
	}
	payload, err := json.Marshal(ClientMessageEnvelope{
		NodeID:     cm.nodeID,
		ProfileIDs: profileIDs,
		Message:    *msg,
	})
	if err != nil {
		return err
	}
	return cm.messageBus.PublishClientMessage(context.Background(), payload)
}

func (cm *ClientManager) updatePresence(profileID string, isOnline bool) {
	if cm.backend == nil {
		return
	}
	var err error
	if isOnline {
		err = cm.backend.MarkOnline(context.Background(), profileID, cm.nodeID, 2*time.Minute)
	} else {
		err = cm.backend.MarkOffline(context.Background(), profileID, cm.nodeID)
	}
	if err != nil {
		logger.Warning("Failed to update distributed presence for %s: %e", profileID, err)
	}
}
