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
	if !(Card{Suit: Clubs, Rank: Jack}).IsTrump(ModeSuit, Hearts) {
		t.Fatal("club jack should be trump in suit games")
	}
	if (Card{Suit: Clubs, Rank: Jack}).IsTrump(ModeNull, NoSuit) {
		t.Fatal("jack should not be trump in null games")
	}
}

func TestCardContractRules(t *testing.T) {
	clubJack := Card{Suit: Clubs, Rank: Jack}
	spadeJack := Card{Suit: Spades, Rank: Jack}
	heartAce := Card{Suit: Hearts, Rank: Ace}
	heartSeven := Card{Suit: Hearts, Rank: Seven}
	diamondAce := Card{Suit: Diamonds, Rank: Ace}

	if got := clubJack.EffectiveSuit(ModeGrand, NoSuit); got != NoSuit {
		t.Fatalf("grand jack effective suit = %v, want NoSuit", got)
	}
	if got := clubJack.EffectiveSuit(ModeNull, NoSuit); got != Clubs {
		t.Fatalf("null jack effective suit = %v, want Clubs", got)
	}
	if got := clubJack.TrumpValue(ModeNull, NoSuit); got != 0 {
		t.Fatalf("null jack trump value = %d, want 0", got)
	}
	if got := heartAce.TrumpValue(ModeSuit, Hearts); got != 7 {
		t.Fatalf("heart ace trump value = %d, want 7", got)
	}
	if !spadeJack.Beats(heartAce, ModeSuit, Hearts) {
		t.Fatal("spade jack should beat heart ace in hearts")
	}
	if diamondAce.Beats(heartSeven, ModeSuit, Hearts) {
		t.Fatal("off-suit diamond ace should not beat heart seven without being trump")
	}
	if got := ModeSuit.TrumpCount(); got != 11 {
		t.Fatalf("suit trump count = %d, want 11", got)
	}
	if got := ModeGrand.TrumpCount(); got != 4 {
		t.Fatalf("grand trump count = %d, want 4", got)
	}
	if got := ModeNull.TrumpCount(); got != 0 {
		t.Fatalf("null trump count = %d, want 0", got)
	}

	trick := Cards{heartAce, spadeJack, clubJack}
	if got := trick.TrickWinner(Listener, ModeSuit, Hearts); got != Dealer {
		t.Fatalf("trick winner = %v, want dealer", got)
	}
}
