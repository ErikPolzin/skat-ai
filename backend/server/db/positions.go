package db

import (
	"database/sql"

	"skat/game"
)

func nullablePosition(position *game.GamePosition) any {
	if position == nil {
		return nil
	}
	return int(*position)
}

func applyNullablePositions(gs *game.GameState, declarer, trickWinner, forfeitedPlayer sql.NullInt64) {
	gs.Declarer = gamePositionPtr(declarer)
	gs.TrickWinner = gamePositionPtr(trickWinner)
	gs.ForfeitedPlayer = gamePositionPtr(forfeitedPlayer)
}

func gamePositionPtr(value sql.NullInt64) *game.GamePosition {
	if !value.Valid {
		return nil
	}
	position := game.GamePosition(value.Int64)
	return &position
}
