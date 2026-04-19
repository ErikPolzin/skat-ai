package db

import (
	"skat/game"
)

type ProfileEntry struct {
	ID      string
	Name    string
	IsAgent bool
}

type PlayerEntry struct {
	GameID    string
	ProfileID string
	Name      string
	IsAgent   bool
	Hand      game.Cards
	Position  game.GamePosition
}

func (pe *PlayerEntry) ToPlayerState() *game.PlayerState {
	return &game.PlayerState{
		ID:      pe.ProfileID,
		Name:    pe.Name,
		Hand:    pe.Hand,
		IsAgent: pe.IsAgent,
	}
}

func FromPlayerState(ps *game.PlayerState, gameID string, position game.GamePosition) PlayerEntry {
	return PlayerEntry{
		GameID:    gameID,
		ProfileID: ps.ID,
		Name:      ps.Name,
		IsAgent:   ps.IsAgent,
		Hand:      ps.Hand,
		Position:  position,
	}
}
