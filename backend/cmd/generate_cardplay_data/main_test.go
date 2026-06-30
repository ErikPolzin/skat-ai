package main

import (
	"encoding/csv"
	"math"
	"os"
	"path/filepath"
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

func TestSearchStrategyUsesConfiguredDepthIncrease(t *testing.T) {
	strategy := newSearchStrategy(9, 1, nil, 8)
	state := &game.GameState{
		Players: [3]*game.PlayerState{
			{Hand: make(game.Cards, 9)},
			{Hand: make(game.Cards, 9)},
			{Hand: make(game.Cards, 9)},
		},
		CardsPlayed: [][]game.Card{{{}, {}, {}}},
	}
	if got := strategy.SearchDepth(state); got != 10 {
		t.Fatalf("search depth after one trick = %d, want 10", got)
	}
}

func TestDatasetProgressResumesAppendedBuckets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cardplay.csv")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := csv.NewWriter(file)
	if err := writer.Write(cardPlayHeader()); err != nil {
		t.Fatal(err)
	}
	examples := []CardPlayExample{
		{Role: roleDeclarer, GameMode: game.ModeSuit},
		{Role: roleDefender, GameMode: game.ModeSuit},
		{Role: roleDeclarer, GameMode: game.ModeGrand},
		{Role: roleDefender, GameMode: game.ModeGrand},
		{Role: roleDeclarer, GameMode: game.ModeNull},
		{Role: roleDefender, GameMode: game.ModeNull},
		{Role: roleRamsch, GameMode: game.ModeRamsch},
	}
	for _, example := range examples {
		if err := writer.Write(cardPlayRecord(example)); err != nil {
			t.Fatal(err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	counts, hasHeader, err := loadDatasetProgress(path, true)
	if err != nil {
		t.Fatal(err)
	}
	if !hasHeader || totalBucketCount(counts) != len(bucketOrder) {
		t.Fatalf("loaded header/counts = %v/%v", hasHeader, counts)
	}
	for _, key := range bucketOrder {
		if counts[key] != 1 {
			t.Fatalf("bucket %s count = %d, want 1", key, counts[key])
		}
	}

	file, writer, err = openDatasetWriter(path, true, hasHeader)
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Write(cardPlayRecord(CardPlayExample{Role: roleDeclarer, GameMode: game.ModeSuit})); err != nil {
		t.Fatal(err)
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	counts, _, err = loadDatasetProgress(path, true)
	if err != nil {
		t.Fatal(err)
	}
	if counts["suit_declarer"] != 2 || totalBucketCount(counts) != len(bucketOrder)+1 {
		t.Fatalf("appended counts = %v", counts)
	}
}

func assertPolicyProbability(t *testing.T, policy [32]float32, card game.Card, want float64) {
	t.Helper()
	got := float64(policy[encoding.CardToIndex(card)])
	if math.Abs(got-want) > 1e-6 {
		t.Fatalf("policy[%v] = %.3f, want %.3f", card, got, want)
	}
}
