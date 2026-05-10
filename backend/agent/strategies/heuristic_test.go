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
		{Suit: game.Clubs, Rank: game.Jack},  // Trump (Jack)
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
		{Suit: game.Clubs, Rank: game.Ace},   // Should lead this first
		{Suit: game.Hearts, Rank: game.Ace},  // Trump ace
		{Suit: game.Hearts, Rank: game.Ten},  // Trump
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
