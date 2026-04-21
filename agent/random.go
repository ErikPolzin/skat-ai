package agent

import (
	"math/rand"
	"skat/game"
)

// RandomAgent makes random valid moves
type RandomAgent struct {
	name string
}

func NewRandomAgent(name string) *RandomAgent {
	return &RandomAgent{name: name}
}

func (ra *RandomAgent) SelectMove(state *game.GameState, validMoves []game.Card) game.Card {
	if len(validMoves) == 0 {
		panic("no valid moves")
	}
	return validMoves[rand.Intn(len(validMoves))]
}

func (ra *RandomAgent) Bid(state *game.GameState) bool {
	// Random bidding strategy: 50% chance to accept
	return rand.Float32() < 0.5
}

func (ra *RandomAgent) ChooseGame(state *game.GameState) (game.GameMode, game.Suit) {
	// Randomly choose game type
	mode := game.GameMode(rand.Intn(2))
	suit := game.Suit(rand.Intn(4))
	return mode, suit
}

func (ra *RandomAgent) Name() string {
	return ra.name
}
