package db

import (
	"skat/game"
	"time"
)

type ProfileEntry struct {
	ID          string
	Name        string
	IsAgent     bool
	ProfileIcon string
	IsOnline    bool
}

type PlayerEntry struct {
	GameID      string
	ProfileID   string
	Name        string
	IsAgent     bool
	ProfileIcon string
	IsOnline    bool
	Hand        game.Cards
	Position    game.GamePosition
}

func (pe *PlayerEntry) ToPlayerState() *game.PlayerState {
	return &game.PlayerState{
		ID:          pe.ProfileID,
		Name:        pe.Name,
		Hand:        pe.Hand,
		IsAgent:     pe.IsAgent,
		ProfileIcon: pe.ProfileIcon,
		IsOnline:    pe.IsOnline,
	}
}

func FromPlayerState(ps *game.PlayerState, gameID string, position game.GamePosition) PlayerEntry {
	return PlayerEntry{
		GameID:      gameID,
		ProfileID:   ps.ID,
		Name:        ps.Name,
		IsAgent:     ps.IsAgent,
		ProfileIcon: ps.ProfileIcon,
		IsOnline:    ps.IsOnline,
		Hand:        ps.Hand,
		Position:    position,
	}
}

type PlayerRating struct {
	ProfileID   string
	Rating      int
	GamesPlayed int
	Wins        int
	Losses      int
	PeakRating  int
	LastUpdated time.Time
}

type AgentConfig struct {
	ProfileID string
	// Bidding strategy configuration
	BiddingType      string
	BiddingThreshold float64 // For weighted heuristic bidding
	// Game choice strategy configuration
	GameChoiceType string
	// Card play strategy configuration
	CardPlayType         string
	MCTSSimulations      *int    // For MCTS card play (nullable)
	CardplayWeightsPath  *string // Path to combined neural network weights (nullable)
	CreatedAt            time.Time
	UpdatedAt            time.Time
}
