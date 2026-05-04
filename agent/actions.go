package agent

import (
	"fmt"
	"skat/game"
	"skat/logger"
	"sync"
)

type Action func() (string, error)

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
	DeclarerWeightsPath string
	DefenderWeightsPath string
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
		BiddingType:      config.BiddingType,
		BiddingThreshold: config.BiddingThreshold,
		GameChoiceType:   config.GameChoiceType,
		CardPlayType:     config.CardPlayType,
		MCTSSimulations:  config.MCTSSimulations,
		DQNDeclarerPath:  config.DeclarerWeightsPath,
		DQNDefenderPath:  config.DefenderWeightsPath,
	}

	return NewHybridAgent(config.ProfileID, hybridConfig)
}

// getAgentForPlayer creates an agent instance based on the player's configuration
func getAgentForPlayer(player *game.PlayerState) *SkatAgent {
	if !player.IsAgent {
		return nil
	}

	// Check cache first
	agentCacheMu.RLock()
	if cached, ok := agentCache[player.ID]; ok {
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
		agent := NewHeuristicAgent(player.Name)
		agentCacheMu.Lock()
		agentCache[player.ID] = agent
		agentCacheMu.Unlock()
		return agent
	}

	config, err := loader(player.ID)
	if err != nil {
		logger.Error("Failed to load agent config", "playerID", player.ID, "error", err)
		logger.Warning("Using default heuristic agent")
		agent := NewHeuristicAgent(player.Name)
		agentCacheMu.Lock()
		agentCache[player.ID] = agent
		agentCacheMu.Unlock()
		return agent
	}

	agent, err := BuildAgentFromConfig(config)
	if err != nil {
		logger.Error("Failed to build agent from config", "playerID", player.ID, "error", err)
		logger.Warning("Using default heuristic agent")
		agent = NewHeuristicAgent(player.Name)
	} else {
		logger.Info("Built agent from config", "playerID", player.ID, "bidding", config.BiddingType, "cardPlay", config.CardPlayType)
	}

	// Cache the agent
	agentCacheMu.Lock()
	agentCache[player.ID] = agent
	agentCacheMu.Unlock()

	return agent
}

// ClearAgentCache clears the agent cache (useful for testing or hot-reloading configs)
func ClearAgentCache() {
	agentCacheMu.Lock()
	defer agentCacheMu.Unlock()
	agentCache = make(map[string]*SkatAgent)
}

// gameLoop manages the game flow
func NextAction(gs *game.GameState) Action {
	if gs.Phase == game.PhaseComplete {
		return nil
	}

	phase := gs.Phase
	currentPlayer := gs.GetCurrentPlayer()

	if len(gs.Trick) == 3 && phase == game.PhasePlaying {
		return generateResolveTrickAction(gs)
	}

	if currentPlayer != nil {
		logger.Debug("Game loop", "phase", phase, "currentPlayer", currentPlayer.Name, "position", gs.CurrentPlayer, "isAgent", currentPlayer.IsAgent)
	} else {
		logger.Debug("Game loop", "phase", phase, "currentPlayer", "nil")
		return nil
	}
	if !currentPlayer.IsAgent {
		logger.Debug("Skipping game loop: waiting for human player input")
		return nil
	}

	switch phase {
	case game.PhaseDealing:
		return generateAgentDealAction(gs, currentPlayer)
	case game.PhaseBidding:
		return generateAgentBidAction(gs, currentPlayer)
	case game.PhaseSkatExchange:
		return generateAgentSkatExchangeAction(gs, currentPlayer)
	case game.PhaseDeclarerChoice:
		return generateAgentDeclarationAction(gs, currentPlayer)
	case game.PhasePlaying:
		return generateAgentPlayAction(gs, currentPlayer)
	default:
		logger.Error("Unknown agent game phase", phase)
		return nil
	}
}

func generateAgentDealAction(gs *game.GameState, player *game.PlayerState) Action {
	return func() (string, error) {
		result, err := gs.Deal()

		// Reset card tracking for all agents at start of new game
		if err == nil {
			for i := range gs.Players {
				if gs.Players[i].IsAgent {
					agent := getAgentForPlayer(gs.Players[i])
					if agent != nil {
						agent.OnGameStart()
					}
				}
			}
		}

		return result, err
	}
}

func generateResolveTrickAction(gs *game.GameState) Action {
	return func() (string, error) {
		// Track the trick before it gets cleared
		trick := make([]game.Card, len(gs.Trick))
		copy(trick, gs.Trick)

		result, err := gs.ResolveTrick()

		// Notify all agents that a trick was completed (for card tracking)
		if err == nil && len(trick) == 3 {
			for i := range gs.Players {
				if gs.Players[i].IsAgent {
					agent := getAgentForPlayer(gs.Players[i])
					if agent != nil {
						agent.OnTrickComplete(trick)
					}
				}
			}
		}

		return result, err
	}
}

func generateAgentSkatExchangeAction(gs *game.GameState, player *game.PlayerState) Action {
	// Check if agent has already picked up skat
	if len(player.Hand) == 12 {
		// Agent has picked up skat, needs to discard 2 cards
		// Use game-aware discard strategy: pre-decide game mode, then discard optimally

		// Get shared agent instance for game mode decision
		agentInstance := getAgentForPlayer(player)

		// Agent will use Q-learning to choose game mode
		mode, trumpSuit := agentInstance.ChooseGame(gs)

		// Now choose optimal discard for that game mode
		card1, card2 := agentInstance.ChooseSkatDiscard(player.Hand, mode, trumpSuit)

		return func() (string, error) {
			return gs.Discard(card1, card2)
		}
	} else {
		// Agent hasn't picked up skat yet - always pick it up
		return func() (string, error) {
			return gs.SkatDecision(true)
		}
	}
}

// processAgentDeclaration handles an AI agent's declaration
func generateAgentDeclarationAction(gs *game.GameState, player *game.PlayerState) Action {
	// Get shared agent instance for Q-learning game choice
	agentInstance := getAgentForPlayer(player)

	// Use Q-learning to choose the best game mode and trump suit
	mode, trumpSuit := agentInstance.ChooseGame(gs)

	// AI agents don't announce schneider/schwarz for now (conservative strategy)
	// TODO: In the future, could add a strategy for when to make announcements
	announceSchneider := false
	announceSchwarz := false

	return func() (string, error) {
		return gs.DeclareGame(mode, trumpSuit, announceSchneider, announceSchwarz)
	}
}

func generateAgentPlayAction(gs *game.GameState, player *game.PlayerState) Action {
	validMoves := gs.GetValidMoves()

	if len(validMoves) == 0 {
		logger.Warning("No valid moves for AI", "player", player.Name)
		return nil
	}

	// Get the agent for this player
	agent := getAgentForPlayer(player)
	if agent == nil {
		logger.Warning("No agent found for player", "player", player.Name)
		return nil
	}
	move := agent.SelectMove(gs, validMoves)

	return func() (string, error) {
		return gs.PlayCard(move)
	}
}

func generateAgentBidAction(gs *game.GameState, player *game.PlayerState) Action {
	// Get the agent for this player
	agent := getAgentForPlayer(player)
	if agent == nil {
		logger.Warning("No agent found for player", "player", player.Name)
		return nil
	}
	// Get a copy of the game state for the agent
	stateCopy := gs // Make a copy
	// Call the agent's Bid method
	accept := agent.Bid(stateCopy)
	logger.Debug("AI choosing bid", "player", player.Name, "accept", accept, "currentBid", gs.BidValue)

	return func() (string, error) {
		return gs.Bid(accept)
	}
}
