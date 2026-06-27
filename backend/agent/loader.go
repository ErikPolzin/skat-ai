package agent

import (
	"fmt"
	"skat/game"
	"sync"
)

// AgentConfigLoader is a function type that loads agent configuration for a player
type AgentConfigLoader func(profileID string) (*AgentConfigData, error)

// AgentConfigData holds the configuration data for an agent (matches db.AgentConfig)
type AgentConfigData struct {
	ProfileID           string
	BiddingType         string
	BiddingThreshold    float64
	GameChoiceType      string
	CardPlayType        string
	CardplayWeightsPath string
}

var (
	// Agent cache for reusing agent instances
	agentCache     = make(map[string]*SkatAgent)
	agentCacheMu   sync.RWMutex
	configLoader   AgentConfigLoader
	configLoaderMu sync.RWMutex
)

// SetAgentConfigLoader sets the function used to load agent configurations
func SetAgentConfigLoader(loader AgentConfigLoader) {
	configLoaderMu.Lock()
	defer configLoaderMu.Unlock()
	configLoader = loader
}

// BuildAgentFromConfig creates a SkatAgent from configuration data
func BuildAgentFromConfig(config *AgentConfigData) (*SkatAgent, error) {
	if config == nil {
		return nil, fmt.Errorf("config is nil")
	}

	hybridConfig := HybridAgentConfig{
		BiddingType:       config.BiddingType,
		BiddingThreshold:  config.BiddingThreshold,
		GameChoiceType:    config.GameChoiceType,
		CardPlayType:      config.CardPlayType,
		NeuralWeightsPath: config.CardplayWeightsPath,
	}

	return NewHybridAgent(config.ProfileID, hybridConfig)
}

// GetAgentForPlayer creates an agent instance based on the player's configuration
func GetAgentForPlayer(player *game.PlayerState) (*SkatAgent, error) {
	if player == nil {
		return nil, fmt.Errorf("player is nil")
	}
	if !player.IsAgent {
		return nil, nil
	}
	return GetAgentForPlayerID(player.ID)
}

// MustGetAgentForPlayer returns the configured agent or panics when it cannot
// be loaded. It is intended for offline simulations and data-generation tools.
func MustGetAgentForPlayer(player *game.PlayerState) *SkatAgent {
	agent, err := GetAgentForPlayer(player)
	if err != nil {
		panic(fmt.Sprintf("failed to load agent: %v", err))
	}
	return agent
}

// GetAgentForPlayerID loads and builds an agent, returning any failure to the
// caller. Only successfully loaded agents are cached.
func GetAgentForPlayerID(playerID string) (*SkatAgent, error) {

	// Check cache first
	agentCacheMu.RLock()
	if cached, ok := agentCache[playerID]; ok {
		agentCacheMu.RUnlock()
		return cached, nil
	}
	agentCacheMu.RUnlock()

	// Load config using the configured loader
	configLoaderMu.RLock()
	loader := configLoader
	configLoaderMu.RUnlock()

	if loader == nil {
		return nil, fmt.Errorf("no agent config loader configured")
	}

	config, err := loader(playerID)
	if err != nil {
		return nil, fmt.Errorf("failed to load agent config for profile %s: %w", playerID, err)
	}

	agent, err := BuildAgentFromConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to build agent for profile %s: %w", playerID, err)
	}

	// Cache the agent
	agentCacheMu.Lock()
	agentCache[playerID] = agent
	agentCacheMu.Unlock()

	return agent, nil
}

func SetAgentForPlayer(player *game.PlayerState, agent *SkatAgent) {
	agentCacheMu.Lock()
	agentCache[player.ID] = agent
	agentCacheMu.Unlock()
}

// ClearAgentCache clears the agent cache (useful for testing or hot-reloading configs)
func ClearAgentCache() {
	agentCacheMu.Lock()
	defer agentCacheMu.Unlock()
	agentCache = make(map[string]*SkatAgent)
}
