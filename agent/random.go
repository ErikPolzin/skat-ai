package agent

import (
	"fmt"
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

func (ra *RandomAgent) Bid(state *game.GameState, currentBid int) int {
	// Random bidding strategy
	if rand.Float32() < 0.3 {
		// Try to bid the next valid value
		validBids := state.GetValidBids()
		for _, bid := range validBids {
			if bid != "pass" && bid != "hold" {
				// Try to parse the bid value
				var bidValue int
				if _, err := fmt.Sscanf(bid, "%d", &bidValue); err == nil {
					return bidValue
				}
			}
		}
		// If we're responding, we can hold
		if currentBid > 0 && rand.Float32() < 0.5 {
			return currentBid // Hold
		}
	}
	return 0 // Pass
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
