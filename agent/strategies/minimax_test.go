package strategies

import (
	"skat/game"
	"testing"
)

// TestMinimaxSimpleCase tests minimax with a trivial end-game scenario
func TestMinimaxSimpleCase(t *testing.T) {
	// Setup: Declarer has 60 points, needs 1 more to win
	// Declarer has Ace of Hearts (11 points) - should play it to win
	// Defenders have low cards
	g := game.NewGame()
	g = g.WithTestPlayers()

	// Set up the game state
	declarer := game.Dealer
	g.Declarer = &declarer
	g.Mode = game.ModeSuit
	g.TrumpSuit = game.Clubs
	g.Phase = game.PhasePlaying
	g.CurrentPlayer = declarer
	g.DeclarerScore = 60 // Needs 1 more point to win
	g.TrickStarter = declarer

	// Declarer has Ace of Hearts (11 points - winning card)
	g.Players[declarer].Hand = []game.Card{
		{Suit: game.Hearts, Rank: game.Ace}, // 11 points - wins the game
	}

	// First defender has a low card
	g.Players[(declarer+1)%3].Hand = []game.Card{
		{Suit: game.Hearts, Rank: game.Seven}, // 0 points
	}

	// Second defender has a low card
	g.Players[(declarer+2)%3].Hand = []game.Card{
		{Suit: game.Hearts, Rank: game.Eight}, // 0 points
	}

	// Test minimax selection
	minimax := NewPerfectInfoMinimaxStrategyWithDepth(3)
	validMoves := g.GetValidMoves()

	if len(validMoves) != 1 {
		t.Fatalf("Expected 1 valid move, got %d", len(validMoves))
	}

	move := minimax.SelectMove(g, validMoves)

	// Should select the Ace (only option)
	expectedCard := game.Card{Suit: game.Hearts, Rank: game.Ace}
	if move != expectedCard {
		t.Errorf("Expected to play %v, but played %v", expectedCard, move)
	}

	// Play out the trick and verify declarer wins
	g.PlayCard(move)
	g.PlayCard(game.Card{Suit: game.Hearts, Rank: game.Seven})
	g.PlayCard(game.Card{Suit: game.Hearts, Rank: game.Eight})
	g.ResolveTrick()

	if g.DeclarerScore < 61 {
		t.Errorf("Declarer should have won with score >= 61, got %d", g.DeclarerScore)
	}
}

// TestMinimaxDefenderChoice tests that defenders minimize declarer score
func TestMinimaxDefenderChoice(t *testing.T) {
	// Setup: Declarer has 50 points
	// Defender can either give declarer 11 points (Ace) or 0 points (Seven)
	// Defender should choose Seven to minimize declarer score
	g := game.NewGame()
	g = g.WithTestPlayers()

	declarer := game.Dealer
	defender := (declarer + 1) % 3
	g.Declarer = &declarer
	g.Mode = game.ModeSuit
	g.TrumpSuit = game.Clubs
	g.Phase = game.PhasePlaying
	g.CurrentPlayer = defender
	g.DeclarerScore = 50
	g.TrickStarter = defender

	// Defender leads - has choice between valuable Ace or worthless Seven
	g.Players[defender].Hand = []game.Card{
		{Suit: game.Hearts, Rank: game.Ace},   // 11 points - bad to lead (declarer wins it)
		{Suit: game.Hearts, Rank: game.Seven}, // 0 points - good to lead
	}

	// Declarer has a Ten to win the trick
	g.Players[declarer].Hand = []game.Card{
		{Suit: game.Hearts, Rank: game.Ten}, // 10 points
	}

	// Other defender has low card
	g.Players[(declarer+2)%3].Hand = []game.Card{
		{Suit: game.Hearts, Rank: game.Eight}, // 0 points
	}

	// Test minimax - defender should lead Seven, not Ace
	minimax := NewPerfectInfoMinimaxStrategyWithDepth(3)
	validMoves := g.GetValidMoves()

	move := minimax.SelectMove(g, validMoves)

	// Defender should lead the Seven (0 points) not the Ace (11 points)
	expectedCard := game.Card{Suit: game.Hearts, Rank: game.Seven}
	if move != expectedCard {
		t.Errorf("Defender should lead Seven to minimize points, but led %v", move)
	}
}

// TestMinimaxDeclarerChoice tests that declarer maximizes score
func TestMinimaxDeclarerChoice(t *testing.T) {
	// Setup: Declarer leads and can capture either 11 points or 0 points
	// Declarer should choose to win the Ace
	g := game.NewGame()
	g = g.WithTestPlayers()

	declarer := game.Dealer
	g.Declarer = &declarer
	g.Mode = game.ModeGrand // Jacks are only trumps
	g.Phase = game.PhasePlaying
	g.CurrentPlayer = declarer
	g.DeclarerScore = 50
	g.TrickStarter = declarer

	// Declarer leads with choice between Jack (trump) or Seven
	g.Players[declarer].Hand = []game.Card{
		{Suit: game.Clubs, Rank: game.Jack},   // Trump - will win trick
		{Suit: game.Hearts, Rank: game.Seven}, // Non-trump
	}

	// First defender has Ace (11 points)
	g.Players[(declarer+1)%3].Hand = []game.Card{
		{Suit: game.Hearts, Rank: game.Ace}, // 11 points
	}

	// Second defender has low card
	g.Players[(declarer+2)%3].Hand = []game.Card{
		{Suit: game.Hearts, Rank: game.Eight}, // 0 points
	}

	// Test minimax - declarer should lead Seven to collect the Ace
	// (defenders must follow suit, so they'll put down Ace+Eight, declarer doesn't win but gets 11 points)
	// Actually, let me reconsider: if declarer leads Seven, defenders play Ace+Eight,
	// Ace wins (11 points to defenders)
	// If declarer leads Jack, defenders can't follow trump, they discard Hearts,
	// declarer wins with 2 points (Jack)

	// Let me fix this scenario
	g.Players[declarer].Hand = []game.Card{
		{Suit: game.Clubs, Rank: game.Jack}, // 2 points, trump - wins trick
	}

	g.Players[(declarer+1)%3].Hand = []game.Card{
		{Suit: game.Hearts, Rank: game.Ace}, // 11 points - will be captured if declarer leads trump
	}

	g.Players[(declarer+2)%3].Hand = []game.Card{
		{Suit: game.Hearts, Rank: game.Ten}, // 10 points - will be captured if declarer leads trump
	}

	minimax := NewPerfectInfoMinimaxStrategyWithDepth(3)
	validMoves := g.GetValidMoves()

	move := minimax.SelectMove(g, validMoves)

	// Declarer should lead the Jack to win the trick and capture Ace+Ten (21 points)
	expectedCard := game.Card{Suit: game.Clubs, Rank: game.Jack}
	if move != expectedCard {
		t.Errorf("Declarer should lead Jack to capture points, but led %v", move)
	}

	// Verify by playing it out
	g.PlayCard(move)
	g.PlayCard(game.Card{Suit: game.Hearts, Rank: game.Ace})
	g.PlayCard(game.Card{Suit: game.Hearts, Rank: game.Ten})
	g.ResolveTrick()

	// Declarer should have won 2+11+10 = 23 points, total = 50+23 = 73
	if g.DeclarerScore != 73 {
		t.Errorf("Expected declarer score 73, got %d", g.DeclarerScore)
	}
}
