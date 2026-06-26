package game

import "testing"

func TestCardsSummaryHelpers(t *testing.T) {
	cards := Cards{
		{Suit: Clubs, Rank: Jack},
		{Suit: Spades, Rank: Jack},
		{Suit: Hearts, Rank: Ace},
		{Suit: Hearts, Rank: Ten},
		{Suit: Hearts, Rank: King},
		{Suit: Diamonds, Rank: Seven},
	}

	if got := cards.TotalPoints(); got != 29 {
		t.Fatalf("TotalPoints() = %d, want 29", got)
	}
	if got := cards.CountRank(Jack); got != 2 {
		t.Fatalf("CountRank(Jack) = %d, want 2", got)
	}
	if got := cards.CountRanks(Seven, Eight, Nine); got != 1 {
		t.Fatalf("CountRanks(low null cards) = %d, want 1", got)
	}
	if got := cards.CountTopJacks(); got != 2 {
		t.Fatalf("CountTopJacks() = %d, want 2", got)
	}
	if got := cards.CountAceTenPairs(); got != 1 {
		t.Fatalf("CountAceTenPairs() = %d, want 1", got)
	}
	if got := cards.MaxSuitLength(); got != 3 {
		t.Fatalf("MaxSuitLength() = %d, want 3", got)
	}
	if got := cards.ContractTrumpCount(ModeSuit, Hearts); got != 5 {
		t.Fatalf("ContractTrumpCount(hearts) = %d, want 5", got)
	}
	if got := cards.ContractTrumpPoints(ModeSuit, Hearts); got != 29 {
		t.Fatalf("ContractTrumpPoints(hearts) = %d, want 29", got)
	}
	if !cards.HasTopTrumpControl(ModeSuit, Hearts) {
		t.Fatal("HasTopTrumpControl(hearts) = false, want true")
	}
	if got := cards.SideAceCount(ModeSuit, Hearts); got != 0 {
		t.Fatalf("SideAceCount(hearts) = %d, want 0", got)
	}
	if got := cards.VoidSuitCount(ModeSuit, Hearts); got != 3 {
		t.Fatalf("VoidSuitCount(hearts) = %d, want 3", got)
	}
	if got := cards.SingletonSuitCount(ModeSuit, Hearts); got != 1 {
		t.Fatalf("SingletonSuitCount(hearts) = %d, want 1", got)
	}
	if !cards.IsContractTrump(Card{Suit: Clubs, Rank: Jack}, ModeSuit, Hearts) {
		t.Fatal("club jack should be trump in suit games")
	}
	if cards.IsContractTrump(Card{Suit: Clubs, Rank: Jack}, ModeNull, NoSuit) {
		t.Fatal("jack should not be trump in null games")
	}
}
