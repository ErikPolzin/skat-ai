package strategies

import (
	"skat/game"
	"testing"
)

func TestContractEvaluatorBestUsesBidAndSharedScores(t *testing.T) {
	choice := NewHeuristicGameChoiceStrategy()
	hand := []game.Card{
		{Suit: game.Clubs, Rank: game.Jack},
		{Suit: game.Spades, Rank: game.Jack},
		{Suit: game.Clubs, Rank: game.Ace},
		{Suit: game.Clubs, Rank: game.Ten},
		{Suit: game.Clubs, Rank: game.King},
		{Suit: game.Clubs, Rank: game.Queen},
		{Suit: game.Clubs, Rank: game.Nine},
		{Suit: game.Hearts, Rank: game.Ace},
		{Suit: game.Spades, Rank: game.Ten},
		{Suit: game.Diamonds, Rank: game.Seven},
	}

	best, ok := choice.evaluator.Best(hand, 24)
	if !ok {
		t.Fatalf("expected a playable contract")
	}
	if best.GameValue < 24 {
		t.Fatalf("expected game value to satisfy bid, got %d", best.GameValue)
	}
	if best.WinProbability < DefaultContractEvaluatorConfig().MinWinProbability {
		t.Fatalf("expected acceptable win probability, got %.3f", best.WinProbability)
	}
}

func TestContractEvaluatorRejectsUnplayableBid(t *testing.T) {
	choice := NewHeuristicGameChoiceStrategy()
	hand := []game.Card{
		{Suit: game.Clubs, Rank: game.Seven},
		{Suit: game.Clubs, Rank: game.Eight},
		{Suit: game.Spades, Rank: game.Nine},
		{Suit: game.Spades, Rank: game.Queen},
		{Suit: game.Hearts, Rank: game.King},
		{Suit: game.Hearts, Rank: game.Seven},
		{Suit: game.Diamonds, Rank: game.Eight},
		{Suit: game.Diamonds, Rank: game.Nine},
		{Suit: game.Diamonds, Rank: game.Queen},
		{Suit: game.Hearts, Rank: game.Ten},
	}

	best, ok := choice.evaluator.Best(hand, 63)
	if ok {
		t.Fatalf("expected no acceptable contract, got %+v", best)
	}
}

func TestContractStrategiesShareEvaluatorDecision(t *testing.T) {
	hand := []game.Card{
		{Suit: game.Clubs, Rank: game.Jack},
		{Suit: game.Spades, Rank: game.Jack},
		{Suit: game.Hearts, Rank: game.Jack},
		{Suit: game.Clubs, Rank: game.Ace},
		{Suit: game.Clubs, Rank: game.Ten},
		{Suit: game.Clubs, Rank: game.King},
		{Suit: game.Spades, Rank: game.Ace},
		{Suit: game.Hearts, Rank: game.Ace},
		{Suit: game.Diamonds, Rank: game.Ten},
		{Suit: game.Diamonds, Rank: game.Seven},
	}

	choice := NewHeuristicGameChoiceStrategy()
	expected, ok := choice.evaluator.Best(hand, 24)
	if !ok {
		t.Fatalf("expected evaluator to find a contract")
	}
	mode, suit := choice.ChooseGame(hand, 24)
	if mode != expected.Mode || suit != expected.TrumpSuit {
		t.Fatalf("game choice did not use evaluator result: got %s/%s, want %s/%s", mode, suit, expected.Mode, expected.TrumpSuit)
	}
}
