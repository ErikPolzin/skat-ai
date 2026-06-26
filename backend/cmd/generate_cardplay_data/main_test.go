package main

import (
	"math"
	"testing"

	"skat/agent/strategies/encoding"
	"skat/game"
)

func TestAcceptableMovePolicyMaximizing(t *testing.T) {
	moves := []game.Card{
		{Suit: game.Clubs, Rank: game.Seven},
		{Suit: game.Clubs, Rank: game.Eight},
		{Suit: game.Clubs, Rank: game.Nine},
	}
	policy, best := acceptableMovePolicy(moves, []float64{20, 18, 10}, true, 5)
	if best != 0 {
		t.Fatalf("best = %d, want 0", best)
	}
	assertPolicyProbability(t, policy, moves[0], 0.5)
	assertPolicyProbability(t, policy, moves[1], 0.5)
	assertPolicyProbability(t, policy, moves[2], 0)
}

func TestAcceptableMovePolicyMinimizing(t *testing.T) {
	moves := []game.Card{
		{Suit: game.Spades, Rank: game.Seven},
		{Suit: game.Spades, Rank: game.Eight},
		{Suit: game.Spades, Rank: game.Nine},
	}
	policy, best := acceptableMovePolicy(moves, []float64{-12, -10, 3}, false, 3)
	if best != 0 {
		t.Fatalf("best = %d, want 0", best)
	}
	assertPolicyProbability(t, policy, moves[0], 0.5)
	assertPolicyProbability(t, policy, moves[1], 0.5)
	assertPolicyProbability(t, policy, moves[2], 0)
}

func assertPolicyProbability(t *testing.T, policy [32]float32, card game.Card, want float64) {
	t.Helper()
	got := float64(policy[encoding.CardToIndex(card)])
	if math.Abs(got-want) > 1e-6 {
		t.Fatalf("policy[%v] = %.3f, want %.3f", card, got, want)
	}
}
