package game

import (
	"fmt"
	"math/rand"
	"slices"

	"github.com/google/uuid"
)

// GameMode represents the type of Skat game being played
type GameMode string

const (
	ModeGrand GameMode = "grand" // Only Jacks are trump
	ModeSuit  GameMode = "suit"  // One suit is trump (plus Jacks)
	ModeNull  GameMode = "null"  // No trumps, declarer tries to lose
)

type GamePosition int

const (
	Dealer GamePosition = iota
	Listener
	Speaker
)

var AllPositions = [3]GamePosition{Dealer, Listener, Speaker}

// GameState represents the current state of a Skat game
type GameState struct {
	ID            string          `json:"id"`
	Code          GameCode        `json:"code"`
	SessionID     string          `json:"session_id"` // Session ID for mutiple games
	GameNumber    int             `json:"game_number"`
	Players       [3]*PlayerState `json:"players"`        // Players in the game
	Skat          SkatCards       `json:"-"`              // Ommitted in JSON, not public knowledge
	CurrentPlayer GamePosition    `json:"current_player"` // Current player position
	Declarer      GamePosition    `json:"declarer"`       // Declarer position
	Mode          GameMode        `json:"mode"`           // Game mode (Suit, Null, Grand)
	TrumpSuit     Suit            `json:"trump_suit"`     // Trump suite (D, H, A, C)
	Trick         Cards           `json:"trick"`          // Current trick being played
	TrickWinner   GamePosition    `json:"trick_winner"`   // Who won the last trick
	TrickStarter  GamePosition    `json:"trick_starter"`  // Who started the current trick
	CardsPlayed   [][]Card        `json:"-"`              // History of all tricks
	Phase         GamePhase       `json:"phase"`          // Current phase of the game
	GameValue     int             `json:"game_value"`     // Value of the current game
	DeclarerScore int             `json:"declarer_score"` // Current score for declarer
	OpponentScore int             `json:"opponent_score"` // Current score for opponents

	// Bidding state
	BidValue       int  `json:"bid_value"`       // Current bid value
	ListenerPassed bool `json:"listener_passed"` // Has listener passed?
	SpeakerPassed  bool `json:"speaker_passed"`  // Has speaker passed?
	DealerPassed   bool `json:"dealer_passed"`   // Has dealer passed?
}

type GameSessionState struct {
	ID          string  `json:"id"`
	Code        string  `json:"code"`
	GameID      string  `json:"game_id"`
	PlayerCount int     `json:"player_count"`
	CreatedAt   string  `json:"created_at"`
	EndedAt     *string `json:"ended_at"`
}

type PlayerResultState struct {
	GameID         string       `json:"game_id"`
	SessionID      string       `json:"session_id"`
	PlayerID       string       `json:"player_id"`
	PlayerPosition GamePosition `json:"player_position"`
	PlayerPoints   int          `json:"player_points"`
	IsWinner       bool         `json:"is_winner"`
}

type GamePhase string

const (
	PhaseWaitingForPlayers GamePhase = "waiting_for_players" // Waiting for players to join
	PhaseDealing           GamePhase = "dealing"             // Waiting for dealer to deal cards
	PhaseBidding           GamePhase = "bidding"             // Bidding phase
	PhaseSkatExchange      GamePhase = "skat_exchange"       // Declarer decides whether to pick up skat
	PhaseDeclarerChoice    GamePhase = "declarer_choice"     // Declarer chooses game mode after skat exchange
	PhasePlaying           GamePhase = "playing"
	PhaseComplete          GamePhase = "complete"
)

type GameCode string

func NewGameCode() GameCode {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	code := ""
	for i := 0; i < 4; i++ {
		code += string(chars[rand.Intn(len(chars))])
	}

	// Fallback to longer code if we can't find a unique 4-char code
	return GameCode(fmt.Sprintf("%08X", rand.Uint32()))
}

// PlayerState represents one player's state
type PlayerState struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Hand      Cards  `json:"-"`
	CardCount int    `json:"card_count"`
	IsAgent   bool   `json:"is_agent"`
}

// NewGame creates a new Skat game
func NewGame() *GameState {
	gs := &GameState{
		ID:             uuid.New().String(),
		SessionID:      uuid.New().String(),
		Code:           NewGameCode(),
		Players:        [3]*PlayerState{},
		Phase:          PhaseWaitingForPlayers, // Start waiting for players
		Declarer:       -1,                     // Not determined yet
		CurrentPlayer:  0,                      // Dealer starts as the current player
		BidValue:       0,                      // No bid yet, Speaker will bid 18 first
		ListenerPassed: false,
		SpeakerPassed:  false,
		DealerPassed:   false,
	}
	return gs
}

// PlayerCount returns the number of non-nil players
func (gs *GameState) PlayerCount() int {
	count := 0
	for _, player := range gs.Players {
		if player != nil {
			count++
		}
	}
	return count
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
			Hand:    append([]Card{}, gs.Players[i].Hand...),
			ID:      gs.Players[i].ID,
			Name:    gs.Players[i].Name,
			IsAgent: gs.Players[i].IsAgent,
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
		declarerWon = gs.DeclarerScore == 0
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

func (gs *GameState) GetPlayerByPosition(position GamePosition) *PlayerState {
	return gs.Players[position]
}

func (gs *GameState) GetPlayerById(playerId string) *PlayerState {
	for _, player := range gs.Players {
		if player != nil && player.ID == playerId {
			return player
		}
	}
	return nil
}

// getCurrentPlayer gets the player object for the current player
func (gs *GameState) GetCurrentPlayer() *PlayerState {
	for _, pos := range AllPositions {
		if pos == gs.CurrentPlayer {
			return gs.Players[pos]
		}
	}
	return nil
}

func (gs *GameState) GetCurrentPlayerID() string {
	player := gs.GetCurrentPlayer()
	if player != nil {
		return player.ID
	}
	return ""
}

func (gs *GameState) GetPositionForPlayer(playerID string) GamePosition {
	for _, pos := range AllPositions {
		if gs.Players[pos] != nil && gs.Players[pos].ID == playerID {
			return pos
		}
	}
	return -1
}

// get a position that hasn't been assigned yet
func (gs *GameState) GetRandomPosition() GamePosition {
	availablePositions := []int{0, 1, 2}
	for i, player := range gs.Players {
		if player != nil {
			idx := slices.Index(availablePositions, i)
			if idx != -1 {
				availablePositions = slices.Delete(availablePositions, idx, idx+1)
			}
		}
	}
	if len(availablePositions) == 0 {
		return -1
	}
	return GamePosition(availablePositions[rand.Int()%len(availablePositions)])
}

func Results(gs *GameState) []PlayerResultState {
	if gs.Phase != PhaseComplete {
		return nil
	}

	results := make([]PlayerResultState, 0, 3)
	declarerWon, _, _ := gs.GetGameResult()
	points := gs.CalculatePlayerPoints()

	for pos := Dealer; pos <= Speaker; pos++ {
		player := gs.Players[pos]
		if player == nil {
			continue
		}

		result := PlayerResultState{
			GameID:         gs.ID,
			SessionID:      gs.SessionID,
			PlayerID:       player.ID,
			IsWinner:       (pos == gs.Declarer && declarerWon) || (pos != gs.Declarer && !declarerWon),
			PlayerPosition: pos,
			PlayerPoints:   points[pos],
		}

		results = append(results, result)
	}

	return results
}
