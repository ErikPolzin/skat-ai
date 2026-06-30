package strategies

import (
	"skat/game"
	"testing"
)

type contractEstimatorFunc func(game.Cards, game.GameMode, game.Suit) float64

func (f contractEstimatorFunc) EstimateWinProbability(hand game.Cards, mode game.GameMode, suit game.Suit) float64 {
	return f(hand, mode, suit)
}

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

func TestEstimateRamschWinProbabilitiesAreRelative(t *testing.T) {
	safe := []game.Card{{Suit: game.Hearts, Rank: game.Seven}, {Suit: game.Spades, Rank: game.Eight}}
	dangerous := []game.Card{{Suit: game.Clubs, Rank: game.Jack}, {Suit: game.Hearts, Rank: game.Ace}, {Suit: game.Spades, Rank: game.Ten}}
	probabilities := EstimateRamschWinProbabilities([3][]game.Card{safe, dangerous, safe})
	if probabilities[0] <= probabilities[1] || probabilities[2] <= probabilities[1] {
		t.Fatalf("dangerous Ramsch hand should be less likely to win: %v", probabilities)
	}
	if total := probabilities[0] + probabilities[1] + probabilities[2]; total < 0.999999 || total > 1.000001 {
		t.Fatalf("probabilities sum to %v, want 1", total)
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

func TestAtomicGameChoiceEvaluatesPostDiscardHand(t *testing.T) {
	hand := game.NewDeck()[:12]
	estimator := contractEstimatorFunc(func(cards game.Cards, mode game.GameMode, _ game.Suit) float64 {
		if mode == game.ModeNull && len(cards) == 10 {
			return 0.9
		}
		return 0.6
	})
	choice := NewHeuristicGameChoiceStrategyWithEstimator(DefaultContractEvaluatorConfig(), estimator)

	decision := choice.ChooseGameAndSkatDiscard(hand, 18)
	if decision.Mode != game.ModeNull {
		t.Fatalf("ChooseGameAndSkatDiscard mode = %s, want Null scored from post-discard hand", decision.Mode)
	}
	if remaining := handWithoutCards(hand, decision.Discard[:]...); len(remaining) != 10 {
		t.Fatalf("atomic choice leaves %d cards, want 10", len(remaining))
	}
}

func TestBiddingDoesNotAbandonStrongerContractToRaise(t *testing.T) {
	hand := game.NewDeck()[:10]
	estimator := contractEstimatorFunc(func(_ game.Cards, mode game.GameMode, _ game.Suit) float64 {
		if mode == game.ModeNull {
			return 0.9
		}
		return 0.6
	})
	bidding := NewHeuristicBiddingStrategyWithEstimator(DefaultContractEvaluatorConfig(), estimator)
	state := &game.GameState{BidValue: 23}

	if bidding.ShouldBid(state, hand, 23) {
		t.Fatal("ShouldBid raised past Null 23 into a weaker fallback contract")
	}
}
