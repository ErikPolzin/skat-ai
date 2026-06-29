package strategies

import (
	"math"
	"path/filepath"
	"skat/game"
	"testing"
)

func smallHandWinState() *game.GameState {
	declarer := game.Dealer
	return &game.GameState{
		Players: [3]*game.PlayerState{
			{Hand: game.Cards{{Suit: game.Hearts, Rank: game.Ace}}},
			{Hand: game.Cards{{Suit: game.Spades, Rank: game.Ten}}},
			{Hand: game.Cards{{Suit: game.Diamonds, Rank: game.King}}},
		},
		Declarer:      &declarer,
		Mode:          game.ModeSuit,
		TrumpSuit:     game.Clubs,
		Phase:         game.PhasePlaying,
		CurrentPlayer: game.Listener,
		TrickStarter:  game.Listener,
		PlayerScores:  [3]int{50, 30, 20},
		Skat: game.SkatCards{
			{Suit: game.Clubs, Rank: game.Seven},
			{Suit: game.Clubs, Rank: game.Eight},
		},
	}
}

func TestHandWinPredictorAggregatesWinProbability(t *testing.T) {
	predictor := NewHandWinPredictor(6)
	state := smallHandWinState()
	for i := 0; i < 7; i++ {
		predictor.Observe(state, true)
	}
	for i := 0; i < 3; i++ {
		predictor.Observe(state, false)
	}

	estimate, ok := predictor.Lookup(state, 8)
	if !ok {
		t.Fatal("expected populated leaf bucket")
	}
	if want := 8.0 / 12.0; math.Abs(estimate.WinProbability-want) > 1e-9 {
		t.Fatalf("probability = %.6f, want beta-smoothed %.6f", estimate.WinProbability, want)
	}
	if estimate.Error <= 0 || estimate.Samples != 10 {
		t.Fatalf("estimate error/samples = %.4f/%d", estimate.Error, estimate.Samples)
	}

	differentScore := state.Clone()
	differentScore.PlayerScores[game.Dealer]++
	key, _ := predictor.key(state)
	otherKey, _ := predictor.key(differentScore)
	if key == otherKey {
		t.Fatal("detailed positions needing different points must not share a bucket")
	}
}

func TestHandWinPredictorKeyCanonicalizesNonTrumpSuits(t *testing.T) {
	predictor := NewHandWinPredictor(6)
	state := smallHandWinState()
	swapped := state.Clone()
	for _, player := range swapped.Players {
		for i := range player.Hand {
			switch player.Hand[i].Suit {
			case game.Hearts:
				player.Hand[i].Suit = game.Spades
			case game.Spades:
				player.Hand[i].Suit = game.Hearts
			}
		}
	}
	key, ok := predictor.key(state)
	otherKey, otherOK := predictor.key(swapped)
	if !ok || !otherOK || key != otherKey {
		t.Fatalf("suit-equivalent keys = %d/%d (valid %v/%v)", key, otherKey, ok, otherOK)
	}
}

func TestHandWinPredictorCoarseKeyKeepsFourPointScoreBands(t *testing.T) {
	predictor := NewHandWinPredictor(6)
	state := smallHandWinState()
	state.PlayerScores[game.Dealer] = 51 // 10 points needed
	other := state.Clone()
	other.PlayerScores[game.Dealer] = 48 // 13 points needed

	key, ok := predictor.keyAtLevel(state, 2)
	otherKey, otherOK := predictor.keyAtLevel(other, 2)
	if !ok || !otherOK || key == otherKey {
		t.Fatalf("level-2 keys = %d/%d (valid %v/%v), want distinct score bands", key, otherKey, ok, otherOK)
	}
}

func TestHandWinPredictorPersistenceAndEvaluatorLookup(t *testing.T) {
	predictor := NewHandWinPredictor(6)
	state := smallHandWinState()
	for i := 0; i < 8; i++ {
		predictor.Observe(state, i < 6)
	}
	path := filepath.Join(t.TempDir(), "hand_win_predictor.gob")
	if err := predictor.Save(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadHandWinPredictor(path)
	if err != nil {
		t.Fatal(err)
	}
	config := DefaultMinimaxSearchConfig(3)
	config.HandWinPredictor = loaded
	config.HandWinMinSamples = 8
	strategy := NewPerfectInfoMinimaxStrategyWithConfig(config)
	estimate := strategy.EvaluateStateEstimate(state)
	if !estimate.FromHandWinPredictor || estimate.Samples != 8 {
		t.Fatalf("evaluator did not use hand win predictor: %+v", estimate)
	}
}

func TestHandWinPredictorMaxBucketsKeepsCountingExistingKeys(t *testing.T) {
	predictor := NewHandWinPredictor(6)
	predictor.MaxBuckets = 2
	known := smallHandWinState()
	predictor.Observe(known, true)
	if got := len(predictor.Buckets); got != predictor.MaxBuckets {
		t.Fatalf("bucket count = %d, want cap %d", got, predictor.MaxBuckets)
	}

	countsBefore := uint64(0)
	for _, stats := range predictor.Buckets {
		countsBefore += stats.Count
	}
	predictor.Observe(known, false)
	countsAfter := uint64(0)
	for _, stats := range predictor.Buckets {
		countsAfter += stats.Count
	}
	if countsAfter != countsBefore+uint64(predictor.MaxBuckets) {
		t.Fatalf("existing observations = %d, want %d", countsAfter, countsBefore+uint64(predictor.MaxBuckets))
	}

	unknown := known.Clone()
	unknown.PlayerScores[game.Dealer] = 1
	predictor.Observe(unknown, true)
	if got := len(predictor.Buckets); got != predictor.MaxBuckets {
		t.Fatalf("bucket count grew beyond cap to %d", got)
	}
}
