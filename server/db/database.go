package db

import (
	"skat/game"
)

type Database interface {
	Close() error
	InitSchema() error
	GetProfile(profileID string) (*ProfileEntry, error)
	SaveProfile(profile ProfileEntry) error
	SaveGameSession(session game.GameSessionState) error
	GetGameSession(sessionID string) (*game.GameSessionState, error)
	GetGameByID(gameId string) (*game.GameState, error)
	GetGameBySessionCode(sessionCode string) (*game.GameState, error)
	SaveGame(game game.GameState) error
	DeleteGame(gameID string) error
	ListOpenSessions() ([]game.GameSessionState, error)
	ListPlayers(gameID string) ([3]*game.PlayerState, error)
	SavePlayerResults(result []game.PlayerResultState) error
	GetPlayerResults(playerID string, limit int) ([]game.PlayerResultState, error)
	CountGamesInSession(sessionID string) (int, error)
	GetSessionResults(sessionID string) ([]game.PlayerResultState, error)
	ListAgentProfiles() ([]ProfileEntry, error)
}
