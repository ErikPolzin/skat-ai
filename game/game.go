package game

import (
	"fmt"
	"math/rand"
)

// GameMode represents the type of Skat game being played
type GameMode int

const (
	ModeGrand GameMode = iota // Only Jacks are trump
	ModeSuit                  // One suit is trump (plus Jacks)
	ModeNull                  // No trumps, declarer tries to lose
)

type GamePosition int

const (
	Dealer GamePosition = iota
	Listener
	Speaker
)

// GameState represents the current state of a Skat game
type GameState struct {
	Players       [3]*PlayerState
	Skat          [2]Card // The two cards set aside
	CurrentPlayer GamePosition
	Declarer      GamePosition // Player who won the bid (-1 if not determined yet)
	Mode          GameMode
	TrumpSuit     Suit
	Trick         []Card       // Current trick being played
	TrickWinner   GamePosition // Who won the last trick
	TrickStarter  GamePosition // Who started the current trick
	CardsPlayed   [][]Card     // History of all tricks
	Phase         GamePhase    // Current phase of the game
	GameValue     int          // Value of the current game
	DeclarerScore int          // Current score for declarer
	OpponentScore int          // Current score for opponents

	// Bidding state
	BidValue       int  // Current bid value
	ListenerPassed bool // Has listener passed?
	SpeakerPassed  bool // Has speaker passed?
	DealerPassed   bool // Has dealer passed?

	// Null game state
	DeclarerTricks int // Number of tricks taken by declarer in null games
}

type GamePhase int

const (
	PhaseDealing        GamePhase = iota // Waiting for dealer to deal cards
	PhaseBidding                         // Bidding phase
	PhaseSkatExchange                    // Declarer decides whether to pick up skat
	PhaseDeclarerChoice                  // Declarer chooses game mode after skat exchange
	PhasePlaying
	PhaseComplete
)

// PlayerState represents one player's state
type PlayerState struct {
	Hand        []Card
	TricksTaken [][]Card
	IsActive    bool
}

// NewGame creates a new Skat game
func NewGame() *GameState {
	gs := &GameState{
		Players: [3]*PlayerState{
			{IsActive: true},
			{IsActive: true},
			{IsActive: true},
		},
		Phase:          PhaseDealing, // Start with dealing phase
		Declarer:       -1,           // Not determined yet
		CurrentPlayer:  0,            // Dealer starts as the current player
		BidValue:       0,            // No bid yet, Speaker will bid 18 first
		ListenerPassed: false,
		SpeakerPassed:  false,
		DealerPassed:   false,
	}
	// Don't deal cards yet - wait for dealer to click deal button
	return gs
}

// Deal shuffles and deals cards to players
func (gs *GameState) Deal() {
	deck := NewDeck()
	// Shuffle
	rand.Shuffle(len(deck), func(i, j int) {
		deck[i], deck[j] = deck[j], deck[i]
	})

	// Deal: 3-4-3 pattern to each player, then skat
	idx := 0
	for round := 0; round < 3; round++ {
		for p := 0; p < 3; p++ {
			count := 3
			if round == 1 {
				count = 4
			}
			for i := 0; i < count; i++ {
				gs.Players[p].Hand = append(gs.Players[p].Hand, deck[idx])
				idx++
			}
		}
	}
	// Skat (2 cards)
	gs.Skat[0] = deck[30]
	gs.Skat[1] = deck[31]
}

// GetValidMoves returns all legal moves for the current player
func (gs *GameState) GetValidMoves() []Card {
	if gs.Phase != PhasePlaying {
		return nil
	}

	player := gs.Players[gs.CurrentPlayer]
	if len(gs.Trick) == 0 {
		// Lead player can play any card
		return append([]Card{}, player.Hand...)
	}

	// Must follow suit if possible
	leadCard := gs.Trick[0]
	leadSuit := gs.effectiveSuit(leadCard)

	var valid []Card
	for _, card := range player.Hand {
		if gs.effectiveSuit(card) == leadSuit {
			valid = append(valid, card)
		}
	}

	// If can't follow suit, can play anything
	if len(valid) == 0 {
		return append([]Card{}, player.Hand...)
	}
	return valid
}

// effectiveSuit returns the effective suit for following rules
func (gs *GameState) effectiveSuit(card Card) Suit {
	// Jacks are their own "suit" (trump) in Grand and Suit games
	if gs.Mode != ModeNull && card.Rank == Jack {
		// Use a special marker - we'll treat all jacks as same "suit"
		return gs.TrumpSuit
	}

	// In Suit games, trump suit cards are considered trump suit
	if gs.Mode == ModeSuit && card.Suit == gs.TrumpSuit {
		return gs.TrumpSuit
	}

	// Otherwise return actual suit
	return card.Suit
}

// PlayCard plays a card for the current player
func (gs *GameState) PlayCard(card Card) error {
	valid := gs.GetValidMoves()
	found := false
	for _, c := range valid {
		if c == card {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("invalid move")
	}

	// Remove card from hand
	player := gs.Players[gs.CurrentPlayer]
	for i, c := range player.Hand {
		if c == card {
			player.Hand = append(player.Hand[:i], player.Hand[i+1:]...)
			break
		}
	}

	// If this is the first card of the trick, record who started it
	if len(gs.Trick) == 0 {
		gs.TrickStarter = gs.CurrentPlayer
	}

	gs.Trick = append(gs.Trick, card)

	if len(gs.Trick) != 3 {
		// Next player
		gs.CurrentPlayer = (gs.CurrentPlayer + 1) % 3
	}

	return nil
}

// resolveTrick determines the winner of the trick
func (gs *GameState) ResolveTrick() {
	winner := Dealer
	winCard := gs.Trick[0]

	for i := Listener; i < 3; i++ {
		if gs.cardBeats(gs.Trick[i], winCard) {
			winner = i
			winCard = gs.Trick[i]
		}
	}

	// Adjust winner index relative to who led
	leadPlayer := (gs.CurrentPlayer + 3 - 2) % 3
	actualWinner := (leadPlayer + winner) % 3

	// Award trick
	gs.Players[actualWinner].TricksTaken = append(
		gs.Players[actualWinner].TricksTaken,
		append([]Card{}, gs.Trick...),
	)

	// Calculate points based on game mode
	if gs.Mode == ModeNull {
		// In null games, if declarer takes any trick, game ends immediately
		if actualWinner == gs.Declarer {
			gs.DeclarerTricks = 1    // Mark that declarer took a trick
			gs.Phase = PhaseComplete // Game ends immediately - declarer loses
			gs.CardsPlayed = append(gs.CardsPlayed, gs.Trick)
			gs.Trick = nil
			gs.TrickWinner = actualWinner
			return // Exit early - game is over
		}
	} else {
		// In normal games, calculate card points
		for _, card := range gs.Trick {
			if actualWinner == gs.Declarer {
				gs.DeclarerScore += card.Value()
			} else {
				gs.OpponentScore += card.Value()
			}
		}
	}

	gs.CardsPlayed = append(gs.CardsPlayed, gs.Trick)
	gs.Trick = nil
	gs.TrickWinner = actualWinner
	gs.CurrentPlayer = actualWinner
	gs.TrickStarter = actualWinner

	// Check if game is over
	if len(gs.Players[0].Hand) == 0 {
		gs.Phase = PhaseComplete
		// Add skat cards to declarer's score in normal games
		if gs.Mode != ModeNull && gs.Declarer >= 0 {
			gs.DeclarerScore += gs.Skat[0].Value() + gs.Skat[1].Value()
		}
		// In null games, if we get here, declarer won (took no tricks)
		if gs.Mode == ModeNull {
			gs.DeclarerTricks = 0 // Explicitly mark as 0 tricks taken
		}
	}
}

// cardBeats returns true if card a beats card b
func (gs *GameState) cardBeats(a, b Card) bool {
	// Null games: no trumps, Ace is highest
	if gs.Mode == ModeNull {
		return gs.cardBeatsNull(a, b)
	}

	// Get trump values (higher = stronger)
	aValue := gs.trumpValue(a)
	bValue := gs.trumpValue(b)

	// If both are trump, compare trump values
	if aValue > 0 && bValue > 0 {
		return aValue > bValue
	}

	// Trump beats non-trump
	if aValue > 0 {
		return true
	}
	if bValue > 0 {
		return false
	}

	// Neither is trump - must follow suit
	aSuit := a.Suit
	bSuit := b.Suit

	// Same suit: compare by rank order (Ace > King > Queen > Ten > Nine > Eight > Seven)
	if aSuit == bSuit {
		return gs.compareNonTrumpRank(a.Rank, b.Rank)
	}

	// Different non-trump suits: first card wins
	return false
}

// cardBeatsNull handles card comparison for Null games
// In Null: No trumps, rank order is A > K > Q > J > 10 > 9 > 8 > 7
func (gs *GameState) cardBeatsNull(a, b Card) bool {
	// Must follow suit
	if a.Suit != b.Suit {
		return false // Can't beat if different suit
	}

	// Null rank order (no special jack handling)
	nullOrder := map[Rank]int{
		Ace:   8,
		King:  7,
		Queen: 6,
		Jack:  5,
		Ten:   4,
		Nine:  3,
		Eight: 2,
		Seven: 1,
	}

	return nullOrder[a.Rank] > nullOrder[b.Rank]
}

// trumpValue returns the trump hierarchy value (0 = not trump)
// In Skat: ♣J (11) > ♠J (10) > ♥J (9) > ♦J (8) > trump suit cards (by rank)
func (gs *GameState) trumpValue(card Card) int {
	// Jacks are always trump (except in Null games)
	if gs.Mode != ModeNull && card.Rank == Jack {
		// Jack trump hierarchy: Clubs > Spades > Hearts > Diamonds
		switch card.Suit {
		case Clubs:
			return 11
		case Spades:
			return 10
		case Hearts:
			return 9
		case Diamonds:
			return 8
		}
	}

	// In Suit games, trump suit cards (non-Jacks) are trump
	if gs.Mode == ModeSuit && card.Suit == gs.TrumpSuit && card.Rank != Jack {
		// Trump suit hierarchy by rank: Ace (7) > Ten (6) > King (5) > Queen (4) > Nine (3) > Eight (2) > Seven (1)
		switch card.Rank {
		case Ace:
			return 7
		case Ten:
			return 6
		case King:
			return 5
		case Queen:
			return 4
		case Nine:
			return 3
		case Eight:
			return 2
		case Seven:
			return 1
		}
	}

	return 0 // Not trump
}

// compareNonTrumpRank compares ranks for non-trump cards
// Skat rank order: Ace > Ten > King > Queen > Nine > Eight > Seven
func (gs *GameState) compareNonTrumpRank(a, b Rank) bool {
	order := map[Rank]int{
		Ace:   7,
		Ten:   6,
		King:  5,
		Queen: 4,
		Nine:  3,
		Eight: 2,
		Seven: 1,
	}
	return order[a] > order[b]
}

// Clone creates a deep copy of the game state
func (gs *GameState) Clone() *GameState {
	clone := &GameState{
		CurrentPlayer: gs.CurrentPlayer,
		Declarer:      gs.Declarer,
		Mode:          gs.Mode,
		TrumpSuit:     gs.TrumpSuit,
		TrickWinner:   gs.TrickWinner,
		Phase:         gs.Phase,
		GameValue:     gs.GameValue,
		DeclarerScore: gs.DeclarerScore,
	}

	for i := 0; i < 3; i++ {
		clone.Players[i] = &PlayerState{
			Hand:     append([]Card{}, gs.Players[i].Hand...),
			IsActive: gs.Players[i].IsActive,
		}
	}

	clone.Skat = gs.Skat
	clone.Trick = append([]Card{}, gs.Trick...)

	return clone
}

// Bidding methods

// CanBid returns true if the current player can make a bid
func (gs *GameState) CanBid() bool {
	return gs.Phase == PhaseBidding
}

// GetValidBids returns the valid bidding actions for the current player
// Returns: "pass", "hold" (match current bid), or bid values >= BidValue+1
func (gs *GameState) GetValidBids() []string {
	if gs.Phase != PhaseBidding {
		return nil
	}

	actions := []string{"pass"}

	// In Skat bidding:
	// - Speaker announces bid values
	// - Listener/Dealer say "yes" (hold) to match announced bid, or pass

	if gs.CurrentPlayer == Speaker && !gs.SpeakerPassed {
		// Speaker can announce the next bid value
		nextBid := gs.getNextBidValue()
		if nextBid <= 264 { // Max reasonable bid
			actions = append(actions, fmt.Sprintf("%d", nextBid))
		}
	} else if gs.CurrentPlayer == Listener && !gs.ListenerPassed {
		// If Speaker has passed, Listener takes over the announcing role
		if gs.SpeakerPassed && gs.BidValue == 0 {
			// Listener can now announce bid values
			nextBid := gs.getNextBidValue()
			if nextBid <= 264 { // Max reasonable bid
				actions = append(actions, fmt.Sprintf("%d", nextBid))
			}
		} else if gs.BidValue > 0 {
			// Normal case: Listener says "yes" to match Speaker's bid or passes
			actions = append(actions, "hold")
		}
	} else if gs.CurrentPlayer == Dealer && !gs.DealerPassed {
		// If both Speaker and Listener passed without bidding, Dealer can announce
		if gs.SpeakerPassed && gs.ListenerPassed && gs.BidValue == 0 {
			// Dealer can now announce bid values
			nextBid := gs.getNextBidValue()
			if nextBid <= 264 { // Max reasonable bid
				actions = append(actions, fmt.Sprintf("%d", nextBid))
			}
		} else if gs.BidValue > 0 {
			// Normal case: Dealer says "yes" to match bid or passes
			actions = append(actions, "hold")
		}
	}

	return actions
}

// getNextBidValue returns the next valid bid value
// Valid bids in Skat: 18, 20, 22, 23, 24, 27, 30, 33, 35, 36, 40, 44, 45, 46, 48, 50, 54, 55, ...
func (gs *GameState) getNextBidValue() int {
	validBids := []int{18, 20, 22, 23, 24, 27, 30, 33, 35, 36, 40, 44, 45, 46, 48, 50, 54, 55, 59, 60, 63, 66, 70, 72, 77, 80, 81, 84, 88, 90, 96, 99, 100, 108, 110, 117, 120, 121, 126, 130, 132, 135, 140, 143, 144, 150, 153, 154, 156, 160, 162, 165, 168, 170, 176, 180, 187, 192, 198, 204, 216, 240, 264}

	// If no bid yet (BidValue == 0), start with 18
	if gs.BidValue == 0 {
		return 18
	}

	for _, bid := range validBids {
		if bid > gs.BidValue {
			return bid
		}
	}
	return gs.BidValue + 1 // Fallback
}

// MakeBid processes a bidding action ("pass", "hold", or a bid value)
func (gs *GameState) MakeBid(action string) error {
	if gs.Phase != PhaseBidding {
		return fmt.Errorf("not in bidding phase")
	}

	validActions := gs.GetValidBids()
	isValid := false
	for _, a := range validActions {
		if a == action {
			isValid = true
			break
		}
	}
	if !isValid {
		return fmt.Errorf("invalid bid action: %s", action)
	}

	if action == "pass" {
		// Player passes
		if gs.CurrentPlayer == Listener {
			gs.ListenerPassed = true
		} else if gs.CurrentPlayer == Speaker {
			gs.SpeakerPassed = true
		} else if gs.CurrentPlayer == Dealer {
			gs.DealerPassed = true
		}
	} else if action == "hold" {
		// Player matches the current bid
		// Bid value stays the same, other player must raise or pass
	} else {
		// Player raises the bid
		var newBid int
		fmt.Sscanf(action, "%d", &newBid)
		gs.BidValue = newBid
	}

	// Determine next player and check if bidding is complete
	gs.advanceBidding()

	return nil
}

// advanceBidding determines the next bidder or ends bidding
func (gs *GameState) advanceBidding() {
	// Check if bidding is over
	passCount := 0
	if gs.ListenerPassed {
		passCount++
	}
	if gs.SpeakerPassed {
		passCount++
	}
	if gs.DealerPassed {
		passCount++
	}

	if passCount >= 2 {
		// Bidding is over, determine declarer
		if !gs.ListenerPassed {
			gs.Declarer = Listener
		} else if !gs.SpeakerPassed {
			gs.Declarer = Speaker
		} else if !gs.DealerPassed {
			gs.Declarer = Dealer
		} else {
			// All passed - dealer becomes declarer by default with minimum bid
			gs.Declarer = Dealer
			gs.BidValue = 18
		}

		// Move to skat exchange phase
		gs.Phase = PhaseSkatExchange
		gs.CurrentPlayer = gs.Declarer
		return
	}

	// Bidding continues - determine next bidder
	// Bidding alternates: Speaker announces, Listener/Dealer respond
	// Round 1: Speaker vs Listener
	// Round 2: Winner vs Dealer

	if !gs.ListenerPassed && !gs.SpeakerPassed {
		// Round 1: Speaker announces, Listener responds
		if gs.CurrentPlayer == Speaker {
			// After Speaker announces, Listener responds
			gs.CurrentPlayer = Listener
		} else {
			// After Listener responds (hold), Speaker can raise
			gs.CurrentPlayer = Speaker
		}
	} else if gs.ListenerPassed && !gs.SpeakerPassed && !gs.DealerPassed {
		// Round 2: Speaker vs Dealer
		if gs.CurrentPlayer == Speaker {
			gs.CurrentPlayer = Dealer
		} else {
			gs.CurrentPlayer = Speaker
		}
	} else if gs.SpeakerPassed && !gs.ListenerPassed && !gs.DealerPassed {
		// Round 2: Listener vs Dealer
		// Listener now takes Speaker role (announces)
		if gs.CurrentPlayer == Listener {
			gs.CurrentPlayer = Dealer
		} else {
			gs.CurrentPlayer = Listener
		}
	}
}

// DecideSkatPickup handles the declarer's decision to pick up skat or play hand
func (gs *GameState) DecideSkatPickup(pickup bool) error {
	if gs.Phase != PhaseSkatExchange {
		return fmt.Errorf("not in skat exchange phase")
	}

	if pickup {
		// Add skat cards to declarer's hand
		gs.Players[gs.Declarer].Hand = append(gs.Players[gs.Declarer].Hand, gs.Skat[0], gs.Skat[1])
		// Stay in skat exchange phase until discard
		return nil
	} else {
		// Play hand - skip to game declaration
		gs.Phase = PhaseDeclarerChoice
		return nil
	}
}

// PickUpSkat allows the declarer to pick up the skat cards (legacy method)
func (gs *GameState) PickUpSkat() error {
	if gs.Phase != PhaseSkatExchange && gs.Phase != PhaseDeclarerChoice {
		return fmt.Errorf("not in skat exchange or declarer choice phase")
	}

	// Add skat cards to declarer's hand
	gs.Players[gs.Declarer].Hand = append(gs.Players[gs.Declarer].Hand, gs.Skat[0], gs.Skat[1])

	return nil
}

// DiscardToSkat allows declarer to discard 2 cards to the skat
func (gs *GameState) DiscardToSkat(card1, card2 Card) error {
	if gs.Phase != PhaseSkatExchange {
		return fmt.Errorf("not in skat exchange phase")
	}

	if len(gs.Players[gs.Declarer].Hand) != 12 {
		return fmt.Errorf("must pick up skat before discarding")
	}

	// Remove cards from hand
	hand := gs.Players[gs.Declarer].Hand
	found1, found2 := false, false
	newHand := []Card{}

	for _, c := range hand {
		if !found1 && c == card1 {
			found1 = true
			continue
		}
		if !found2 && c == card2 {
			found2 = true
			continue
		}
		newHand = append(newHand, c)
	}

	if !found1 || !found2 {
		return fmt.Errorf("cards not in hand")
	}

	gs.Players[gs.Declarer].Hand = newHand
	gs.Skat[0] = card1
	gs.Skat[1] = card2

	// Move to game declaration phase
	gs.Phase = PhaseDeclarerChoice

	return nil
}

// DeclareGame sets the game mode and trump suit, starting the playing phase
func (gs *GameState) DeclareGame(mode GameMode, trumpSuit Suit) error {
	if gs.Phase != PhaseDeclarerChoice {
		return fmt.Errorf("not in declarer choice phase")
	}

	gs.Mode = mode
	gs.TrumpSuit = trumpSuit

	// Calculate game value
	// Simplified for now
	gs.GameValue = gs.BidValue

	// Start playing phase
	gs.Phase = PhasePlaying
	gs.CurrentPlayer = Dealer // Player to left of dealer leads
	if Dealer == 0 {
		gs.CurrentPlayer = 1
	}

	return nil
}

// IsSchneider returns true if the game ended with Schneider
// (one side took 90+ points, null games don't have schneider)
func (gs *GameState) IsSchneider() bool {
	if gs.Mode == ModeNull {
		// Null games don't have schneider - it's binary win/lose
		return false
	}
	// In normal games, Schneider means one side got 90+ points
	return gs.DeclarerScore >= 90 || gs.OpponentScore >= 90
}

// IsSchwarz returns true if the game ended with Schwarz
// (one side took all tricks/120 points, null games don't have schwarz)
func (gs *GameState) IsSchwarz() bool {
	if gs.Mode == ModeNull {
		// Null games don't have schwarz - it's binary win/lose
		return false
	}
	// In normal games, Schwarz means one side took all tricks (120 points)
	return gs.DeclarerScore == 120 || gs.OpponentScore == 120
}

// GetGameResult returns the result of the game including schneider/schwarz
func (gs *GameState) GetGameResult() (declarerWon bool, schneider bool, schwarz bool) {
	if gs.Mode == ModeNull {
		// Null game: declarer wins if they take no tricks
		declarerWon = gs.DeclarerTricks == 0
		// Null games don't have schneider/schwarz
		schneider = false
		schwarz = false
		return
	}

	// Normal games: declarer needs 61+ points to win
	declarerWon = gs.DeclarerScore >= 61

	// Check for schneider (90+ points by one side)
	if declarerWon {
		schneider = gs.OpponentScore < 30 // Opponents got less than 30
		schwarz = gs.OpponentScore == 0   // Opponents got no points
	} else {
		schneider = gs.DeclarerScore < 30 // Declarer got less than 30
		schwarz = gs.DeclarerScore == 0   // Declarer got no points
	}

	return
}
