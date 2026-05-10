package game

func (gs *GameState) WithCardsDealt() *GameState {
	_, err := gs.Deal()
	if err != nil {
		panic(err)
	}
	return gs
}

func (gs *GameState) WithPlayerHand(pos GamePosition, hand Cards) *GameState {
	if gs.Phase != PhaseDealing {
		panic("Cannot seed with declarer hand, game is not in dealing phase")
	}
	remaining := hand.GetRemainingCards()
	remaining.Shuffle()
	gs.Players[0].Hand = make(Cards, 10)
	gs.Players[1].Hand = make(Cards, 10)
	gs.Players[2].Hand = make(Cards, 10)
	copy(gs.Players[pos].Hand, hand)
	copy(gs.Players[(pos+1)%3].Hand, remaining[:10])
	copy(gs.Players[(pos+2)%3].Hand, remaining[10:20])
	gs.Skat = SkatCards{remaining[20], remaining[21]}
	gs.Phase = PhaseBidding
	gs.CurrentPlayer = Speaker
	return gs
}

func (gs *GameState) WithDeclarer(declarer GamePosition, bidValue int) *GameState {
	if gs.Phase != PhaseBidding {
		panic("Cannot seed with game type, game is not in bidding phase")
	}
	gs.BidValue = bidValue
	gs.Declarer = &declarer
	gs.Phase = PhaseSkatExchange
	return gs
}

func (gs *GameState) WithSkatPickedUp(pickup bool) *GameState {
	_, err := gs.SkatDecision(pickup)
	if err != nil {
		panic(err)
	}
	return gs
}

func (gs *GameState) WithGame(gameMode GameMode, gameSuit Suit) *GameState {
	_, err := gs.DeclareGame(gameMode, gameSuit, false, false)
	if err != nil {
		panic(err)
	}
	return gs
}
