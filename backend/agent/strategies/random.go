package strategies

import (
	"math/rand"
	"skat/game"
)

// RandomBiddingStrategy makes random bidding decisions
type RandomBiddingStrategy struct{}

func (r *RandomBiddingStrategy) GetName() string {
	return "RandomBidding"
}

func (r *RandomBiddingStrategy) ShouldBid(gs *game.GameState, hand game.Cards, currentBid int) bool {
	return rand.Float64() < 0.5
}

// RandomGameChoiceStrategy makes random game choice decisions
type RandomGameChoiceStrategy struct{}

func (r *RandomGameChoiceStrategy) GetName() string {
	return "RandomGameChoice"
}

func (r *RandomGameChoiceStrategy) ChooseGame(hand game.Cards, bidValue int) GameChoice {
	modes := []game.GameMode{game.ModeSuit, game.ModeGrand}
	mode := modes[rand.Intn(len(modes))]

	var trumpSuit game.Suit
	if mode == game.ModeSuit {
		suits := []game.Suit{game.Clubs, game.Spades, game.Hearts, game.Diamonds}
		trumpSuit = suits[rand.Intn(len(suits))]
	}

	return GameChoice{Mode: mode, TrumpSuit: trumpSuit}
}

func (r *RandomGameChoiceStrategy) ChooseGameAndSkatDiscard(hand game.Cards, bidValue int) GameChoice {
	if len(hand) != 12 {
		panic("ChooseGameAndSkatDiscard requires the 12-card post-skat hand")
	}
	choice := r.ChooseGame(hand, bidValue)
	choice.Discard = [2]game.Card{hand[0], hand[1]}
	return choice
}

// RandomCardPlayStrategy makes random card play decisions
type RandomCardPlayStrategy struct{}

func (r *RandomCardPlayStrategy) GetName() string {
	return "RandomCardPlay"
}

func (r *RandomCardPlayStrategy) SelectMove(gs *game.GameState, validMoves []game.Card) game.Card {
	return validMoves[rand.Intn(len(validMoves))]
}
