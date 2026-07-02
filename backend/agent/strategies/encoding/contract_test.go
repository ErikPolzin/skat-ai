package encoding

import (
	"skat/game"
	"testing"
)

func TestEncodeNeuralContractIncludesDeclaration(t *testing.T) {
	encoded := EncodeNeuralContract(game.Cards{{Suit: game.Clubs, Rank: game.Jack}}, game.ModeGrand, game.NoSuit, true, true, true)
	features := encoded.ToSlice()

	// 32 card flags followed by six mode flags and the three declaration flags.
	if features[38] != 1 || features[39] != 1 || features[40] != 1 {
		t.Fatalf("declaration features = %v, want all set", features[38:41])
	}
	if features[41] != 1 {
		t.Fatal("post-skat feature not set")
	}
	if len(features) != ContractFeatureSize {
		t.Fatalf("feature count = %d, want %d", len(features), ContractFeatureSize)
	}
}
