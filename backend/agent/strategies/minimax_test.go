package strategies

import (
	"math"
	"skat/game"
	"testing"
)

func TestEvaluateMaterialIncludesDefenderPoints(t *testing.T) {
	declarer := game.Dealer
	state := &game.GameState{
		Players: [3]*game.PlayerState{
			{}, {}, {},
		},
		Declarer:     &declarer,
		PlayerScores: [3]int{12, 20, 10},
	}

	got, _, _, _ := NewPerfectInfoMinimaxStrategyWithDepth(1).evaluateMaterialParts(state, declarer)
	if want := -18.0; got != want {
		t.Fatalf("material score = %.1f, want point margin %.1f", got, want)
	}
}

func TestMinimaxRamschAvoidsWinningPoints(t *testing.T) {
	root := game.Listener
	seven := game.Card{Suit: game.Hearts, Rank: game.Seven}
	king := game.Card{Suit: game.Hearts, Rank: game.King}
	ace := game.Card{Suit: game.Hearts, Rank: game.Ace}
	ten := game.Card{Suit: game.Hearts, Rank: game.Ten}
	state := &game.GameState{
		Players: [3]*game.PlayerState{{Hand: game.Cards{}}, {Hand: game.Cards{seven, ace}}, {Hand: game.Cards{ten}}},
		Mode:    game.ModeRamsch, Phase: game.PhasePlaying, CurrentPlayer: root,
		TrickStarter: game.Dealer, Trick: game.Cards{king},
	}
	strategy := NewPerfectInfoMinimaxStrategyWithDepth(1)
	if got := strategy.SelectMove(state, game.Cards{seven, ace}); got != seven {
		t.Fatalf("selected %v, want %v to avoid taking Ramsch points", got, seven)
	}
}

func TestScoreMovesPreservesInputOrder(t *testing.T) {
	root := game.Listener
	seven := game.Card{Suit: game.Hearts, Rank: game.Seven}
	king := game.Card{Suit: game.Hearts, Rank: game.King}
	ace := game.Card{Suit: game.Hearts, Rank: game.Ace}
	ten := game.Card{Suit: game.Hearts, Rank: game.Ten}
	state := &game.GameState{
		Players: [3]*game.PlayerState{{Hand: game.Cards{}}, {Hand: game.Cards{seven, ace}}, {Hand: game.Cards{ten}}},
		Mode:    game.ModeRamsch, Phase: game.PhasePlaying, CurrentPlayer: root,
		TrickStarter: game.Dealer, Trick: game.Cards{king},
	}
	moves := game.Cards{ace, seven}
	scores := NewPerfectInfoMinimaxStrategyWithDepth(1).ScoreMoves(state, moves)
	if len(scores) != len(moves) {
		t.Fatalf("got %d scores, want %d", len(scores), len(moves))
	}
	if scores[1] <= scores[0] {
		t.Fatalf("scores = %v, want seven at index 1 to score above ace at index 0", scores)
	}
	if moves[0] != ace || moves[1] != seven {
		t.Fatalf("ScoreMoves mutated input order: %v", moves)
	}
}

func TestMinimaxFinishesTrickAtDepthCutoff(t *testing.T) {
	declarer := game.Listener
	seven := game.Card{Suit: game.Hearts, Rank: game.Seven}
	king := game.Card{Suit: game.Hearts, Rank: game.King}
	ace := game.Card{Suit: game.Hearts, Rank: game.Ace}
	ten := game.Card{Suit: game.Hearts, Rank: game.Ten}
	state := &game.GameState{
		Players: [3]*game.PlayerState{
			{Hand: game.Cards{}},
			{Hand: game.Cards{king, ace}},
			{Hand: game.Cards{ten}},
		},
		Declarer:      &declarer,
		Mode:          game.ModeGrand,
		Phase:         game.PhasePlaying,
		CurrentPlayer: game.Listener,
		TrickStarter:  game.Dealer,
		Trick:         game.Cards{seven},
		PlayerScores:  [3]int{0, 40, 0},
	}

	strategy := NewPerfectInfoMinimaxStrategyWithConfig(MinimaxSearchConfig{MaxDepth: 1})
	got := strategy.SelectMove(state, game.Cards{king, ace})
	if got != ace {
		t.Fatalf("selected %v, want %v to secure the trick against %v", got, ace, ten)
	}

	// The cutoff extension must reach the completed trick rather than applying
	// static evaluation to the two-card partial trick.
	next := state.Clone()
	if _, err := next.PlayCard(ace); err != nil {
		t.Fatal(err)
	}
	value := strategy.minimax(next, 0, math.Inf(-1), math.Inf(1))
	if value != 1 {
		t.Fatalf("cutoff value = %.3f, want exact terminal win probability after completing the trick", value)
	}
}

func TestDefaultMinimaxSearchDepthSettings(t *testing.T) {
	config := DefaultMinimaxSearchConfig(DefaultMinimaxBaseDepth)
	if config.BaseDepth != 12 {
		t.Fatalf("base depth = %d, want 12", config.BaseDepth)
	}
	if config.DepthIncreasePerTrick != 2 {
		t.Fatalf("depth increase = %d, want 2", config.DepthIncreasePerTrick)
	}
}

func TestMinimaxOrdersNullDeclarerLosingMoveFirst(t *testing.T) {
	declarer := game.Listener
	seven := game.Card{Suit: game.Clubs, Rank: game.Seven}
	eight := game.Card{Suit: game.Clubs, Rank: game.Eight}
	ace := game.Card{Suit: game.Clubs, Rank: game.Ace}
	state := &game.GameState{
		Players: [3]*game.PlayerState{
			{Hand: game.Cards{}},
			{Hand: game.Cards{ace, seven}},
			{Hand: game.Cards{}},
		},
		Declarer: &declarer, Mode: game.ModeNull, Phase: game.PhasePlaying,
		CurrentPlayer: game.Listener, TrickStarter: game.Dealer, Trick: game.Cards{eight},
	}
	moves := game.Cards{ace, seven}
	heuristicOrder(state, moves, true)
	if moves[0] != seven {
		t.Fatalf("ordered moves = %v, want losing card %v first", moves, seven)
	}
}

func TestMinimaxOrdersNullForcingDefenderMoveFirst(t *testing.T) {
	declarer := game.Listener
	clubsSeven := game.Card{Suit: game.Clubs, Rank: game.Seven}
	clubsEight := game.Card{Suit: game.Clubs, Rank: game.Eight}
	clubsNine := game.Card{Suit: game.Clubs, Rank: game.Nine}
	clubsTen := game.Card{Suit: game.Clubs, Rank: game.Ten}
	state := &game.GameState{
		Players: [3]*game.PlayerState{
			{Hand: game.Cards{}},
			{Hand: game.Cards{}},
			{Hand: game.Cards{clubsTen, clubsEight}},
		},
		Declarer: &declarer, Mode: game.ModeNull, Phase: game.PhasePlaying,
		CurrentPlayer: game.Speaker, TrickStarter: game.Dealer,
		Trick: game.Cards{clubsSeven, clubsNine},
	}
	moves := game.Cards{clubsTen, clubsEight}
	heuristicOrder(state, moves, false)
	if moves[0] != clubsEight {
		t.Fatalf("ordered moves = %v, want forcing card %v first", moves, clubsEight)
	}
}

func TestMinimaxNullEvaluationDistinguishesWeakHand(t *testing.T) {
	declarer := game.Speaker
	perfect, err := game.ParseCards("7.♣-8.♣-9.♣-7.♠-8.♠-9.♠-7.♥-8.♥-7.♦-8.♦")
	if err != nil {
		t.Fatal(err)
	}
	weak, err := game.ParseCards("A.♣-K.♠-Q.♥-J.♦-10.♣-9.♠-8.♥-7.♦-8.♣-9.♣")
	if err != nil {
		t.Fatal(err)
	}
	state := &game.GameState{
		Players:  [3]*game.PlayerState{{}, {}, {Hand: perfect}},
		Declarer: &declarer, Mode: game.ModeNull, Phase: game.PhasePlaying,
		CurrentPlayer: game.Listener, TrickStarter: game.Listener,
	}
	strategy := NewPerfectInfoMinimaxStrategyWithDepth(6)
	perfectProbability := strategy.EvaluateState(state)
	state.Players[declarer].Hand = weak
	weakProbability := strategy.EvaluateState(state)

	if perfectProbability < 0.95 {
		t.Fatalf("perfect null probability = %.3f, want at least 0.95", perfectProbability)
	}
	if weakProbability >= 0.60 {
		t.Fatalf("weak null probability = %.3f, want below 0.60", weakProbability)
	}
	if perfectProbability-weakProbability < 0.30 {
		t.Fatalf("null probability separation = %.3f, want at least 0.30", perfectProbability-weakProbability)
	}
}
