package agent

import (
	"fmt"
	"skat/game"
	"skat/logger"
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
	MCTSSimulations     int
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
		MCTSSimulations:   config.MCTSSimulations,
		NeuralWeightsPath: config.CardplayWeightsPath,
	}

	return NewHybridAgent(config.ProfileID, hybridConfig)
}

// GetAgentForPlayer creates an agent instance based on the player's configuration
func GetAgentForPlayer(player *game.PlayerState) *SkatAgent {
	if !player.IsAgent {
		return nil
	}
	return GetAgentForPlayerID(player.ID)
}

func GetAgentForPlayerID(playerID string) *SkatAgent {

	// Check cache first
	agentCacheMu.RLock()
	if cached, ok := agentCache[playerID]; ok {
		agentCacheMu.RUnlock()
		return cached
	}
	agentCacheMu.RUnlock()

	// Load config using the configured loader
	configLoaderMu.RLock()
	loader := configLoader
	configLoaderMu.RUnlock()

	if loader == nil {
		logger.Warning("No agent config loader set, using default heuristic agent")
		agent := NewHeuristicAgent(playerID)
		agentCacheMu.Lock()
		agentCache[playerID] = agent
		agentCacheMu.Unlock()
		return agent
	}

	config, err := loader(playerID)
	if err != nil {
		logger.Error("Failed to load agent config for profile %s", playerID)
		logger.Warning("Using default heuristic agent")
		agent := NewHeuristicAgent(playerID)
		agentCacheMu.Lock()
		agentCache[playerID] = agent
		agentCacheMu.Unlock()
		return agent
	}

	agent, err := BuildAgentFromConfig(config)
	if err != nil {
		logger.Error("Failed to build agent from config: %e", err)
		logger.Warning("Using default heuristic agent")
		agent = NewHeuristicAgent(playerID)
	}

	// Cache the agent
	agentCacheMu.Lock()
	agentCache[playerID] = agent
	agentCacheMu.Unlock()

	return agent
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
