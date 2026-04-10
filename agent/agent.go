package agent

import "skat/game"

// Agent represents a player that can make decisions
type Agent interface {
	// SelectMove chooses a card to play from valid moves
	SelectMove(state *game.GameState, validMoves []game.Card) game.Card

	// Bid decides whether to bid and at what value
	Bid(state *game.GameState, currentBid int) int

	// ChooseGame selects the game mode after winning the bid
	ChooseGame(state *game.GameState) (game.GameMode, game.Suit)

	// Name returns the agent's identifier
	Name() string
}
