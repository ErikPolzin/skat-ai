package strategies

import (
	"skat/game"
	"testing"
)

func TestCardTracking(t *testing.T) {
	strategy := NewHeuristicCardPlayStrategy()

	// Create a mock game state
	gs := &game.GameState{
		Mode:      game.ModeSuit,
		TrumpSuit: game.Hearts,
	}

	// Test initial state
	if strategy.cardsPlayed == nil {
		t.Error("cardsPlayed map should be initialized")
	}

	// Track some cards
	trick := []game.Card{
		{Suit: game.Clubs, Rank: game.Ace},
		{Suit: game.Clubs, Rank: game.Ten},
		{Suit: game.Clubs, Rank: game.King},
	}
	strategy.OnTrickComplete(trick)

	// Verify cards were tracked
	for _, card := range trick {
		if !strategy.cardsPlayed[card] {
			t.Errorf("Card %v should be tracked as played", card)
		}
	}

	// Create a hand to test remaining trump counting
	hand := []game.Card{
		{Suit: game.Hearts, Rank: game.Ace}, // Trump
		{Suit: game.Clubs, Rank: game.Jack}, // Trump (Jack)
	}

	// Count remaining trumps (should not count our trumps or played cards)
	remaining := strategy.countRemainingTrumps(gs, hand)

	// In suit mode with Hearts trump:
	// Total trumps: 4 Jacks + 7 Hearts cards (excl Jack) = 11 trumps
	// Our trumps: Hearts Ace + Clubs Jack = 2
	// Remaining: 11 - 2 = 9
	if remaining != 9 {
		t.Errorf("Expected 9 remaining trumps, got %d", remaining)
	}

	// Test reset
	strategy.Reset()
	if len(strategy.cardsPlayed) != 0 {
		t.Error("cardsPlayed should be empty after reset")
	}
}

func TestDeclarer_CashesAcesFirst(t *testing.T) {
	strategy := NewHeuristicCardPlayStrategy()

	gs := &game.GameState{
		Mode:      game.ModeSuit,
		TrumpSuit: game.Hearts,
		Trick:     []game.Card{}, // Leading
	}

	// Hand with Ace and trumps
	validMoves := []game.Card{
		{Suit: game.Clubs, Rank: game.Ace},  // Should lead this first
		{Suit: game.Hearts, Rank: game.Ace}, // Trump ace
		{Suit: game.Hearts, Rank: game.Ten}, // Trump
		{Suit: game.Diamonds, Rank: game.Ten},
	}

	move := strategy.selectDeclarerMove(gs, validMoves)

	// Should lead the non-trump Ace first
	if move.Suit != game.Clubs || move.Rank != game.Ace {
		t.Errorf("Expected to lead Clubs Ace first, got %v", move)
	}
}

func TestDefender_DoesNotLeadTrump(t *testing.T) {
	strategy := NewHeuristicCardPlayStrategy()

	gs := &game.GameState{
		Mode:      game.ModeSuit,
		TrumpSuit: game.Hearts,
		Trick:     []game.Card{}, // Leading
	}

	// Hand with only a few trumps (not strong control)
	validMoves := []game.Card{
		{Suit: game.Hearts, Rank: game.Ten},  // Trump
		{Suit: game.Hearts, Rank: game.King}, // Trump
		{Suit: game.Clubs, Rank: game.Ace},   // Should lead this
		{Suit: game.Diamonds, Rank: game.Ten},
	}

	move := strategy.selectDefenderMove(gs, validMoves)

	// Should NOT lead trump (defender shouldn't help declarer draw trumps)
	// Should lead the Ace instead
	if move.Suit == game.Hearts {
		t.Errorf("Defender should not lead trump with weak holdings, got %v", move)
	}
}

func TestNullDefenderLeavesDeclarerWinning(t *testing.T) {
	declarer := game.Listener
	strategy := NewHeuristicCardPlayStrategy()
	clubsEight := game.Card{Suit: game.Clubs, Rank: game.Eight}
	clubsTen := game.Card{Suit: game.Clubs, Rank: game.Ten}
	gs := nullTestState(declarer, game.Speaker, game.Dealer, game.Cards{
		{Suit: game.Clubs, Rank: game.Seven},
		{Suit: game.Clubs, Rank: game.Nine},
	})
	gs.Players[game.Speaker].Hand = game.Cards{clubsEight, clubsTen}

	move := strategy.selectNullDefenderMove(gs, gs.GetValidMoves())
	if move != clubsEight {
		t.Fatalf("defender move = %v, want %v to leave declarer winning", move, clubsEight)
	}
}

func TestNullDefenderPlaysUnderPartnerBeforeDeclarer(t *testing.T) {
	declarer := game.Speaker
	strategy := NewHeuristicCardPlayStrategy()
	clubsSeven := game.Card{Suit: game.Clubs, Rank: game.Seven}
	clubsTen := game.Card{Suit: game.Clubs, Rank: game.Ten}
	gs := nullTestState(declarer, game.Listener, game.Dealer, game.Cards{
		{Suit: game.Clubs, Rank: game.Eight},
	})
	gs.Players[game.Listener].Hand = game.Cards{clubsSeven, clubsTen}
	gs.Players[game.Speaker].Hand = game.Cards{{Suit: game.Clubs, Rank: game.Nine}}

	move := strategy.selectNullDefenderMove(gs, gs.GetValidMoves())
	if move != clubsSeven {
		t.Fatalf("defender move = %v, want %v to force declarer over partner", move, clubsSeven)
	}
}

func TestNullDefenderLeadsLowFromLongestSuit(t *testing.T) {
	declarer := game.Listener
	strategy := NewHeuristicCardPlayStrategy()
	clubsSeven := game.Card{Suit: game.Clubs, Rank: game.Seven}
	clubsNine := game.Card{Suit: game.Clubs, Rank: game.Nine}
	spadesEight := game.Card{Suit: game.Spades, Rank: game.Eight}
	gs := nullTestState(declarer, game.Dealer, game.Dealer, nil)
	gs.Players[game.Dealer].Hand = game.Cards{spadesEight, clubsNine, clubsSeven}

	move := strategy.selectNullDefenderMove(gs, gs.GetValidMoves())
	if move != clubsSeven {
		t.Fatalf("defender lead = %v, want low card %v from longest suit", move, clubsSeven)
	}
}

func TestNullDiscardCanEliminateDangerousSuit(t *testing.T) {
	strategy := NewHeuristicGameChoiceStrategy()
	clubsAce := game.Card{Suit: game.Clubs, Rank: game.Ace}
	clubsKing := game.Card{Suit: game.Clubs, Rank: game.King}
	hand := game.Cards{
		clubsAce, clubsKing,
		{Suit: game.Spades, Rank: game.Ace}, {Suit: game.Spades, Rank: game.Queen},
		{Suit: game.Hearts, Rank: game.Seven}, {Suit: game.Hearts, Rank: game.Eight},
		{Suit: game.Hearts, Rank: game.Nine}, {Suit: game.Hearts, Rank: game.Ten},
		{Suit: game.Diamonds, Rank: game.Seven}, {Suit: game.Diamonds, Rank: game.Eight},
		{Suit: game.Diamonds, Rank: game.Nine}, {Suit: game.Diamonds, Rank: game.Ten},
	}

	first, second := strategy.chooseNullSkatDiscard(hand)
	if !((first == clubsAce && second == clubsKing) || (first == clubsKing && second == clubsAce)) {
		t.Fatalf("Null discard = %v/%v, want to eliminate clubs with %v/%v", first, second, clubsAce, clubsKing)
	}
}

func TestNullEstimatorRewardsEscapableSuitShape(t *testing.T) {
	estimator := NewHeuristicContractWinProbabilityEstimator()
	safe := game.Cards{
		{Suit: game.Clubs, Rank: game.Seven}, {Suit: game.Clubs, Rank: game.Eight},
		{Suit: game.Spades, Rank: game.Seven}, {Suit: game.Spades, Rank: game.Nine},
		{Suit: game.Spades, Rank: game.Jack}, {Suit: game.Hearts, Rank: game.Seven},
		{Suit: game.Hearts, Rank: game.Eight}, {Suit: game.Hearts, Rank: game.Ten},
		{Suit: game.Hearts, Rank: game.Queen}, {Suit: game.Hearts, Rank: game.King},
	}
	unsafe := game.Cards{
		{Suit: game.Clubs, Rank: game.Seven}, {Suit: game.Clubs, Rank: game.Queen},
		{Suit: game.Spades, Rank: game.Eight}, {Suit: game.Spades, Rank: game.King},
		{Suit: game.Hearts, Rank: game.Seven}, {Suit: game.Hearts, Rank: game.Ten},
		{Suit: game.Diamonds, Rank: game.Eight}, {Suit: game.Diamonds, Rank: game.Nine},
		{Suit: game.Diamonds, Rank: game.Jack}, {Suit: game.Diamonds, Rank: game.King},
	}

	safeProbability := estimator.EstimateWinProbability(safe, game.ModeNull, game.NoSuit, false, false, false)
	unsafeProbability := estimator.EstimateWinProbability(unsafe, game.ModeNull, game.NoSuit, false, false, false)
	if safeProbability <= unsafeProbability {
		t.Fatalf("escapable Null hand probability %.3f <= unsafe hand %.3f", safeProbability, unsafeProbability)
	}
}

func nullTestState(declarer, current, starter game.GamePosition, trick game.Cards) *game.GameState {
	gs := &game.GameState{
		Mode:          game.ModeNull,
		Phase:         game.PhasePlaying,
		Declarer:      &declarer,
		CurrentPlayer: current,
		TrickStarter:  starter,
		Trick:         trick,
	}
	for position := game.Dealer; position <= game.Speaker; position++ {
		gs.Players[position] = &game.PlayerState{Hand: game.Cards{}}
	}
	return gs
}
