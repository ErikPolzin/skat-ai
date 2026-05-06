package agent

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"skat/game"
)

type BiddingConfiguration int

const (
	FiftyFiftySplit BiddingConfiguration = iota
	ThreeWay
)

// AgentConfig specifies how agents should be positioned during a game
type AgentConfig struct {
	// For FiftyFiftySplit mode
	TestAgent     *SkatAgent
	BaselineAgent *SkatAgent

	// For ThreeWay mode
	Agent1 *SkatAgent
	Agent2 *SkatAgent
	Agent3 *SkatAgent

	// Configuration mode
	Mode BiddingConfiguration
}

// NewFiftyFiftySplitConfig creates a config for 50/50 declarer/defender split
func NewFiftyFiftySplitConfig(testAgent, baselineAgent *SkatAgent) AgentConfig {
	return AgentConfig{
		TestAgent:     testAgent,
		BaselineAgent: baselineAgent,
		Mode:          FiftyFiftySplit,
	}
}

// NewThreeWayConfig creates a config for three different agents
func NewThreeWayConfig(agent1, agent2, agent3 *SkatAgent) AgentConfig {
	return AgentConfig{
		Agent1: agent1,
		Agent2: agent2,
		Agent3: agent3,
		Mode:   ThreeWay,
	}
}

// CloneAll creates a new AgentConfig with all agents cloned
func (c AgentConfig) CloneAll() AgentConfig {
	cloned := AgentConfig{
		Mode: c.Mode,
	}

	switch c.Mode {
	case FiftyFiftySplit:
		if c.TestAgent != nil {
			cloned.TestAgent = c.TestAgent.Clone()
		}
		if c.BaselineAgent != nil {
			cloned.BaselineAgent = c.BaselineAgent.Clone()
		}
	case ThreeWay:
		if c.Agent1 != nil {
			cloned.Agent1 = c.Agent1.Clone()
		}
		if c.Agent2 != nil {
			cloned.Agent2 = c.Agent2.Clone()
		}
		if c.Agent3 != nil {
			cloned.Agent3 = c.Agent3.Clone()
		}
	}

	return cloned
}

// EnableMetrics enables metrics collection on all agents in the config
func (c AgentConfig) EnableMetrics() {
	switch c.Mode {
	case FiftyFiftySplit:
		if c.TestAgent != nil {
			c.TestAgent.EnableMetrics()
		}
		if c.BaselineAgent != nil {
			c.BaselineAgent.EnableMetrics()
		}
	case ThreeWay:
		if c.Agent1 != nil {
			c.Agent1.EnableMetrics()
		}
		if c.Agent2 != nil {
			c.Agent2.EnableMetrics()
		}
		if c.Agent3 != nil {
			c.Agent3.EnableMetrics()
		}
	}
}

// MergeMetrics merges metrics from another config into this config
func (c AgentConfig) MergeMetrics(other AgentConfig) {
	switch c.Mode {
	case FiftyFiftySplit:
		if c.TestAgent != nil && other.TestAgent != nil {
			c.TestAgent.MergeMetrics(other.TestAgent.GetMetrics())
		}
		if c.BaselineAgent != nil && other.BaselineAgent != nil {
			c.BaselineAgent.MergeMetrics(other.BaselineAgent.GetMetrics())
		}
	case ThreeWay:
		if c.Agent1 != nil && other.Agent1 != nil {
			c.Agent1.MergeMetrics(other.Agent1.GetMetrics())
		}
		if c.Agent2 != nil && other.Agent2 != nil {
			c.Agent2.MergeMetrics(other.Agent2.GetMetrics())
		}
		if c.Agent3 != nil && other.Agent3 != nil {
			c.Agent3.MergeMetrics(other.Agent3.GetMetrics())
		}
	}
}

// generatePlayerID creates a unique random player ID
func generatePlayerID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return "agent-" + hex.EncodeToString(bytes)
}

func WithAgentPlayers(gs *game.GameState, config AgentConfig) *game.GameState {
	if gs.Phase != game.PhaseWaitingForPlayers {
		panic("Cannot seed with test agents, game is not waiting for players")
	}
	// Initialize agents array based on configuration
	agents := make([]*SkatAgent, 3)

	switch config.Mode {
	case FiftyFiftySplit:
		// All three test agents bid (no baseline during bidding)
		agents[0] = config.TestAgent
		agents[1] = config.TestAgent
		agents[2] = config.TestAgent
	case ThreeWay:
		// Three different agents
		agents[0] = config.Agent1
		agents[1] = config.Agent2
		agents[2] = config.Agent3
	}
	for j := 0; j < 3; j++ {
		// Generate unique ID for this player to avoid cache collisions
		playerID := generatePlayerID()
		gs.Players[j] = &game.PlayerState{
			ID:      playerID,
			Name:    fmt.Sprintf("agent %d", j),
			Hand:    game.Cards{},
			IsAgent: true,
		}
		SetAgentForPlayer(gs.Players[j], agents[j])
	}
	gs.Phase = game.PhaseDealing
	return gs
}

func WithAgentBidding(gs *game.GameState, config AgentConfig) *game.GameState {
	if gs.Phase != game.PhaseBidding {
		panic("Cannot seed with game type, game is not in bidding phase")
	}
	// Bidding phase
	for gs.Phase == game.PhaseBidding {
		currentAgent := GetAgentForPlayer(gs.GetCurrentPlayer())
		accept := currentAgent.Bid(gs)
		_, err := gs.Bid(accept)
		if err != nil {
			panic(fmt.Sprintf("Bid error: %v", err))
		}
	}
	// After bidding, set up agents for cardplay based on configuration
	declarerPos := *gs.Declarer
	declarer := gs.GetPlayerByPosition(declarerPos)
	defender1 := gs.GetPlayerByPosition((declarerPos + 1) % 3)
	defender2 := gs.GetPlayerByPosition((declarerPos + 2) % 3)

	switch config.Mode {
	case FiftyFiftySplit:
		// Alternate based on gameNum: even games = test as declarer, odd games = baseline as declarer
		if gs.GameNumber%2 == 0 {
			// Want test agent as declarer - fill defenders with baseline
			SetAgentForPlayer(declarer, config.TestAgent)
			SetAgentForPlayer(defender1, config.BaselineAgent)
			SetAgentForPlayer(defender2, config.BaselineAgent.CachedClone())
		} else {
			// Want baseline as declarer, test as defender
			SetAgentForPlayer(declarer, config.BaselineAgent)
			SetAgentForPlayer(defender1, config.TestAgent)
			SetAgentForPlayer(defender2, config.TestAgent.CachedClone())
		}
	case ThreeWay:
		// No repositioning needed - agents stay as they are
	}
	return gs
}

func WithAgentSkatDecision(gs *game.GameState) *game.GameState {
	if gs.Phase != game.PhaseSkatExchange {
		panic("Cannot run agent skat decision, game is not in skat exchange phase")
	}
	declarer := gs.GetCurrentPlayer()
	declarerAgent := GetAgentForPlayer(declarer)
	// Agent always picks up skat
	if _, err := gs.SkatDecision(true); err != nil {
		panic(fmt.Sprintf("Skat decision error: %v", err))
	}
	mode, trumpSuit := declarerAgent.ChooseGame(gs)
	card1, card2 := declarerAgent.ChooseSkatDiscard(declarer.Hand, mode, trumpSuit)
	if _, err := gs.Discard(card1, card2); err != nil {
		panic(fmt.Sprintf("Skat decision error: %v", err))
	}
	return gs
}

func WithAgentGameChoice(gs *game.GameState) (*game.GameState, bool) {
	if gs.Phase != game.PhaseDeclarerChoice {
		panic("Cannot run agent skat decision, game is not in skat exchange phase")
	}
	declarerAgent := GetAgentForPlayer(gs.GetCurrentPlayer())
	mode, trumpSuit := declarerAgent.ChooseGame(gs)
	if _, err := gs.DeclareGame(mode, trumpSuit, false, false); err != nil {
		panic(fmt.Sprintf("DeclareGame error: %v", err))
	}
	return gs, gs.Overbid
}

func WithAgentCardPlay(gs *game.GameState) *game.GameState {
	if gs.Phase != game.PhasePlaying {
		panic("Cannot automate card play, game is not in playing phase")
	}
	// Playing phase
	for gs.Phase == game.PhasePlaying {
		validMoves := gs.GetValidMoves()
		if len(validMoves) == 0 {
			panic("Cannot play game, no valid moves")
		}
		currentAgent := GetAgentForPlayer(gs.GetCurrentPlayer())
		move := currentAgent.SelectMove(gs, validMoves)
		if _, err := gs.PlayCard(move); err != nil {
			panic(fmt.Sprintf("PlayCard error: %v", err))
		}
		// Resolve trick if complete
		if len(gs.Trick) == 3 {
			if _, err := gs.ResolveTrick(); err != nil {
				panic(fmt.Sprintf("ResolveTrick error: %v", err))
			}
		}
	}

	if gs.Phase != game.PhaseComplete {
		panic(fmt.Sprintf("Tried to play game to completion but phase is: %s", gs.Phase))
	}
	return gs
}

// PlayFullGame plays a complete game from deal to finish.
// The game is played with proper agent positioning based on the AgentConfig.
func PlayFullGame(gs *game.GameState, config AgentConfig) {
	gs = gs.WithCardsDealt()
	gs = WithAgentBidding(gs, config)
	gs = WithAgentSkatDecision(gs)
	gs, overbid := WithAgentGameChoice(gs)
	if !overbid {
		gs = WithAgentCardPlay(gs)
	}
	recordGameResults(gs)
}

// PlayGameWithMode plays a game with a pre-configured declarer, hand, and game mode.
// This is useful for testing specific scenarios.
func PlayGameWithMode(gs *game.GameState, config AgentConfig, declarerHand game.Cards, mode game.GameMode, trumpSuit game.Suit) {
	gs = gs.WithPlayerHand(game.Speaker, declarerHand)
	gs = gs.WithDeclarer(game.Speaker, 0)
	gs = gs.WithSkatPickedUp(false)
	gs = gs.WithGame(mode, trumpSuit)
	if !gs.Overbid {
		gs = WithAgentCardPlay(gs)
	}
	recordGameResults(gs)
}

func recordGameResults(g *game.GameState) {
	// Check if all players passed (Zwangspiel - forced game)
	// In Skat, when all pass, listener is forced to play at bid 18
	if g.SpeakerPassed && g.ListenerPassed && g.DealerPassed {

		// Record passed game (Zwangspiel) for all agents
		for _, player := range g.Players {
			if player != nil && player.IsAgent {
				agent := GetAgentForPlayerID(player.ID)
				agent.RecordPassedGame()
			}
		}
		// Still record the actual game result since listener was forced to play
		if g.Declarer != nil {
			playerResults := g.PlayerResults()
			if playerResults != nil {
				for _, r := range playerResults {
					agent := GetAgentForPlayerID(r.PlayerID)
					agent.RecordGameResult(g, r)
				}
			}
		}
		return
	}

	// Record normal game results if agents have them enabled
	if g.Declarer != nil {
		playerResults := g.PlayerResults()
		if playerResults != nil {
			for _, r := range playerResults {
				agent := GetAgentForPlayerID(r.PlayerID)
				agent.RecordGameResult(g, r)
			}
		}
	}
}
