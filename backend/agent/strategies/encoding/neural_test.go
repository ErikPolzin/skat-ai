package encoding

import (
	"math"
	"testing"

	"skat/game"
)

func TestHigherUnseenCountsCardsThatCanBeatCandidate(t *testing.T) {
	declarer := game.Dealer
	clubJack := game.Card{Suit: game.Clubs, Rank: game.Jack}
	diamondJack := game.Card{Suit: game.Diamonds, Rank: game.Jack}
	clubAce := game.Card{Suit: game.Clubs, Rank: game.Ace}
	gs := &game.GameState{
		Mode:       game.ModeGrand,
		Declarer:   &declarer,
		PlayedHand: true,
		Players: [3]*game.PlayerState{
			{Hand: game.Cards{clubJack, diamondJack, clubAce}},
			{},
			{},
		},
	}

	encoding := EncodeNeuralCardPlay(gs, game.Dealer, gs.Players[game.Dealer].Hand)

	assertFloat32(t, encoding.HigherUnseen[CardToIndex(clubJack)], 0)
	// The two unseen higher jacks can beat the diamond jack. The club jack is
	// also higher, but it is known to be in our hand and is therefore excluded.
	assertFloat32(t, encoding.HigherUnseen[CardToIndex(diamondJack)], 2.0/31.0)
	// Side-suit aces are only standing once all outstanding trumps are known.
	assertFloat32(t, encoding.HigherUnseen[CardToIndex(clubAce)], 2.0/31.0)
}

func TestCardCountsIncludeKnownSkatOnlyForDeclarer(t *testing.T) {
	declarer := game.Dealer
	clubJack := game.Card{Suit: game.Clubs, Rank: game.Jack}
	spadeJack := game.Card{Suit: game.Spades, Rank: game.Jack}
	heartJack := game.Card{Suit: game.Hearts, Rank: game.Jack}
	diamondJack := game.Card{Suit: game.Diamonds, Rank: game.Jack}
	gs := &game.GameState{
		Mode:     game.ModeGrand,
		Declarer: &declarer,
		Skat:     game.SkatCards{clubJack, spadeJack},
		Players: [3]*game.PlayerState{
			{Hand: game.Cards{heartJack, diamondJack}},
			{Hand: game.Cards{{Suit: game.Clubs, Rank: game.Ace}}},
			{},
		},
	}

	declarerEncoding := EncodeNeuralCardPlay(gs, game.Dealer, gs.Players[game.Dealer].Hand)
	defenderEncoding := EncodeNeuralCardPlay(gs, game.Listener, gs.Players[game.Listener].Hand)

	assertFloat32(t, declarerEncoding.RemainingByClass[4], 0)
	assertFloat32(t, defenderEncoding.RemainingByClass[4], 1)
}

func TestToSliceIncludesHigherUnseen(t *testing.T) {
	var encoding NeuralCardPlayEncoding
	encoding.HigherUnseen[31] = 0.5

	state := encoding.ToSlice()
	if got := state[StateFeatureSize-3]; got != 0.5 {
		t.Fatalf("higher-unseen feature missing from state: got %v", got)
	}
}

func TestRemainingCardsUseEffectiveSuitClasses(t *testing.T) {
	declarer := game.Dealer
	clubJack := game.Card{Suit: game.Clubs, Rank: game.Jack}
	clubAce := game.Card{Suit: game.Clubs, Rank: game.Ace}
	gs := &game.GameState{
		Mode:       game.ModeGrand,
		Declarer:   &declarer,
		PlayedHand: true,
		Players: [3]*game.PlayerState{
			{Hand: game.Cards{clubJack, clubAce}},
			{},
			{},
		},
	}

	encoding := EncodeNeuralCardPlay(gs, game.Dealer, gs.Players[game.Dealer].Hand)

	// The printed club jack belongs to trump, so neither known card remains in
	// the unseen club bucket and only the other three jacks remain as trumps.
	assertFloat32(t, encoding.RemainingByClass[0], 6.0/8.0)
	assertFloat32(t, encoding.RemainingByClass[4], 3.0/4.0)
}

func TestRelativeContextAndFollowCount(t *testing.T) {
	declarer := game.Speaker
	gs := &game.GameState{
		Mode:         game.ModeSuit,
		TrumpSuit:    game.Hearts,
		Declarer:     &declarer,
		TrickStarter: game.Dealer,
		Trick: game.Cards{
			{Suit: game.Hearts, Rank: game.Seven},
		},
		Players: [3]*game.PlayerState{
			{},
			{Hand: game.Cards{
				{Suit: game.Clubs, Rank: game.Jack},
				{Suit: game.Hearts, Rank: game.Ace},
				{Suit: game.Clubs, Rank: game.Ace},
			}},
			{},
		},
	}

	encoding := EncodeNeuralCardPlay(gs, game.Listener, gs.Players[game.Listener].Hand)

	assertFloat32(t, encoding.FollowCount, 0.2) // jack and heart ace are both trump
	assertFloat32(t, encoding.DeclarerRelative[1], 1)
	assertFloat32(t, encoding.TrickLeaderRelative[2], 1)
	assertFloat32(t, encoding.CurrentWinnerRelative[2], 1)
}

func assertFloat32(t *testing.T, got, want float32) {
	t.Helper()
	if math.Abs(float64(got-want)) > 1e-6 {
		t.Fatalf("got %v, want %v", got, want)
	}
}
