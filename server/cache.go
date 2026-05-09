package server

import (
	"skat/game"
	"skat/server/db"
)

type Cache struct {
	games map[string]*game.GameState
	db    db.Database
}

func NewCache(database db.Database) *Cache {
	return &Cache{
		games: make(map[string]*game.GameState),
		db:    database,
	}
}

func (c *Cache) GetGameByID(gameID string) (*game.GameState, error) {
	gs, has := c.games[gameID]
	if has {
		return gs, nil
	}
	gs, err := c.db.GetGameByID(gameID)
	if err == nil {
		c.games[gameID] = gs
	}
	return gs, err
}

func (c *Cache) GetGameBySessionCode(sessionCode string) (*game.GameState, error) {
	var gs *game.GameState
	for _, g := range c.games {
		if g.Code == game.GameCode(sessionCode) {
			gs = g
		}
	}
	if gs != nil {
		return gs, nil
	}
	gs, err := c.db.GetGameBySessionCode(sessionCode)
	if err == nil {
		c.games[gs.ID] = gs
	}
	return gs, err
}

func (c *Cache) SaveGame(gs game.GameState) error {
	err := c.db.SaveGame(gs)
	if err == nil {
		c.games[gs.ID] = &gs
	}
	return err
}
