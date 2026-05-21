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
	ModeGrand  GameMode = "grand"  // Only Jacks are trump
	ModeSuit   GameMode = "suit"   // One suit is trump (plus Jacks)
	ModeNull   GameMode = "null"   // No trumps, declarer tries to lose
	ModeRamsch GameMode = "ramsch" // No declarer, each player tries to take few points
)

type PassPolicy string

const (
	PassPolicyReshuffle     PassPolicy = "reshuffle"
	PassPolicyForceListener PassPolicy = "force_listener"
	PassPolicyRamsch        PassPolicy = "ramsch"
)

const (
	DefaultMaxGames     = 10
	DefaultPassPolicy   = PassPolicyReshuffle
	DefaultTimerEnabled = true
)

// ValidBidValues are the legal bid values in Skat (based on game values)
var ValidBidValues = []int{
	0, 18, 20, 22, 23, 24, 27, 30, 33, 35, 36, 40, 44, 45, 46, 48, 50,
	54, 55, 59, 60, 63, 66, 70, 72, 77, 80, 81, 84, 88, 90, 96, 99,
	100, 108, 110, 117, 120, 121, 126, 130, 132, 135, 140, 143, 144,
	150, 153, 154, 156, 160, 162, 165, 168, 170, 176, 180, 187, 192,
	198, 204, 216, 240, 264,
}

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
	MaxGames      int             `json:"max_games"`
	PassPolicy    PassPolicy      `json:"pass_policy"`
	TimerEnabled  bool            `json:"timer_enabled"`
	Players       [3]*PlayerState `json:"players"`        // Players in the game
	Skat          SkatCards       `json:"-"`              // Ommitted in JSON, not public knowledge
	CurrentPlayer GamePosition    `json:"current_player"` // Current player position
	Declarer      *GamePosition   `json:"declarer"`       // Declarer position (nil if not determined yet)
	Mode          GameMode        `json:"mode"`           // Game mode (Suit, Null, Grand)
	TrumpSuit     Suit            `json:"trump_suit"`     // Trump suite (D, H, A, C)
	Trick         Cards           `json:"trick"`          // Current trick being played
	TrickWinner   *GamePosition   `json:"trick_winner"`   // Who won the last trick
	TrickStarter  GamePosition    `json:"trick_starter"`  // Who started the current trick
	CardsPlayed   [][]Card        `json:"-"`              // History of all tricks
	Phase         GamePhase       `json:"phase"`          // Current phase of the game
	PlayerScores  [3]int          `json:"player_scores"`  // Card points taken by each player
	Matadors      int             `json:"matadors"`       // Matadors count (positive=with, negative=without)

	// Hand and announcements
	PlayedHand         bool `json:"played_hand"`         // Declarer played without picking up skat
	AnnouncedSchneider bool `json:"announced_schneider"` // Declarer announced schneider
	AnnouncedSchwarz   bool `json:"announced_schwarz"`   // Declarer announced schwarz

	// Bidding state
	BidValue       int  `json:"bid_value"`       // Current bid value
	ListenerPassed bool `json:"listener_passed"` // Has listener passed?
	SpeakerPassed  bool `json:"speaker_passed"`  // Has speaker passed?
	DealerPassed   bool `json:"dealer_passed"`   // Has dealer passed?
	Overbid        bool `json:"overbid"`         // True if declarer's game value < bid value (automatic loss)

	// Inactivity timeout tracking
	CurrentPlayerDeadline string        `json:"current_player_deadline"` // RFC3339 timestamp when current player times out
	ForfeitedPlayer       *GamePosition `json:"forfeited_player"`        // Position of player who forfeited (nil if no forfeit)
}

type GameResult struct {
	BaseValue          int  `json:"base_value"`          // Base value (9-12 for suits, 24 for grand, 23 for null)
	Matadors           int  `json:"matadors"`            // Matadors (positive=with, negative=without)
	Multiplier         int  `json:"multiplier"`          // Total multiplier (1 + |matadors| + schneider + schwarz + hand + announced)
	DeclarerWon        bool `json:"declarer_won"`        // Did declarer win
	IsSchneider        bool `json:"is_schneider"`        // Schneider achieved
	IsSchwarz          bool `json:"is_schwarz"`          // Schwarz achieved
	PlayedHand         bool `json:"played_hand"`         // Played without skat
	AnnouncedSchneider bool `json:"announced_schneider"` // Announced schneider
	AnnouncedSchwarz   bool `json:"announced_schwarz"`   // Announced schwarz
	Value              int  `json:"value"`               // Final game value (negative if lost, doubled if lost)
	IsForfeit          bool `json:"is_forfeit"`          // Game ended due to forfeit
}

type GameSessionState struct {
	ID           string  `json:"id"`
	Code         string  `json:"code"`
	GameID       string  `json:"game_id"`
	PlayerCount  int     `json:"player_count"`
	MaxGames     int     `json:"max_games"`
	PassPolicy   string  `json:"pass_policy"`
	TimerEnabled bool    `json:"timer_enabled"`
	CreatedAt    string  `json:"created_at"`
	EndedAt      *string `json:"ended_at"`
}

type PlayerResultState struct {
	GameID         string       `json:"game_id"`
	SessionID      string       `json:"session_id"`
	PlayerID       string       `json:"player_id"`
	PlayerPosition GamePosition `json:"player_position"`
	PlayerPoints   int          `json:"player_points"`
	IsWinner       bool         `json:"is_winner"`
	IsDeclarer     bool         `json:"is_declarer"`
	IsOverbid      bool         `json:"is_overbid"`
	OtherPlayers   []string     `json:"other_players,omitempty"`
}

type PlayerSessionResultState struct {
	SessionID    string   `json:"session_id"`
	PlayerID     string   `json:"player_id"`
	PlayerPoints int      `json:"player_points"`
	IsWinner     bool     `json:"is_winner"`
	IsForfeit    bool     `json:"is_forfeit"`
	RatingBefore int      `json:"rating_before"`
	RatingAfter  int      `json:"rating_after"`
	RatingChange int      `json:"rating_change"`
	OtherPlayers []string `json:"other_players,omitempty"`
}

// SessionGameResult represents the results of a single game within a session
type SessionGameResult struct {
	GameID          string            `json:"-"`
	GameNumber      int               `json:"game_number"`
	DeclarerName    string            `json:"declarer_name"`
	DeclarerWon     bool              `json:"declarer_won"`
	GameMode        string            `json:"game_mode"`
	TrumpSuit       string            `json:"trump_suit"`
	PlayerResults   map[string]int    `json:"player_results"` // PlayerID -> Points
	PlayerNames     map[string]string `json:"player_names"`   // PlayerID -> Name
	PlayerWinners   map[string]bool   `json:"player_winners"` // PlayerID -> IsWinner
	ForfeitedPlayer *GamePosition     `json:"forfeited_player"`
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
	// Generate a 4-character hexadecimal code (1000-FFFF)
	code := rand.Intn(0xF000) + 0x1000
	return GameCode(fmt.Sprintf("%04X", code))
}

// PlayerState represents one player's state
type PlayerState struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Hand         Cards  `json:"-"`
	CardCount    int    `json:"card_count"`
	IsAgent      bool   `json:"is_agent"`
	ProfileIcon  string `json:"profile_icon"`
	IsOnline     bool   `json:"is_online"`
	ReadyForNext bool   `json:"ready_for_next"` // Player has consented to play next game
}

// NewGame creates a new Skat game
func NewGame() *GameState {
	gs := &GameState{
		ID:              uuid.New().String(),
		SessionID:       uuid.New().String(),
		Code:            NewGameCode(),
		Players:         [3]*PlayerState{},
		MaxGames:        DefaultMaxGames,
		PassPolicy:      DefaultPassPolicy,
		TimerEnabled:    DefaultTimerEnabled,
		Phase:           PhaseWaitingForPlayers, // Start waiting for players
		Declarer:        nil,                    // Not determined yet
		CurrentPlayer:   0,                      // Dealer starts as the current player
		BidValue:        0,                      // Bidding starts at 0
		ListenerPassed:  false,
		SpeakerPassed:   false,
		DealerPassed:    false,
		ForfeitedPlayer: nil, // No forfeit
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
func (gs *GameState) GetValidMoves() Cards {
	if gs.Phase != PhasePlaying {
		return nil
	}

	player := gs.Players[gs.CurrentPlayer]
	if len(gs.Trick) == 0 {
		// Lead player can play any card
		return append(Cards{}, player.Hand...)
	}

	// Must follow suit if possible
	leadCard := gs.Trick[0]
	leadSuit := gs.effectiveSuit(leadCard)

	var valid Cards
	for _, card := range player.Hand {
		if gs.effectiveSuit(card) == leadSuit {
			valid = append(valid, card)
		}
	}

	// If can't follow suit, can play anything
	if len(valid) == 0 {
		return append(Cards{}, player.Hand...)
	}
	return valid
}

// effectiveSuit returns the effective suit for following rules
func (gs *GameState) effectiveSuit(card Card) Suit {
	// Jacks are their own "suit" (trump) in Grand and Suit games
	if gs.Mode != ModeNull && card.Rank == Jack {
		// In Grand mode, use a special marker for all Jacks
		// In Suit mode, use the trump suit
		if gs.Mode == ModeGrand {
			return NoSuit // All Jacks have same effective suit in Grand
		}
		return gs.TrumpSuit
	}

	// In Suit games, trump suit cards are considered trump suit
	if gs.Mode == ModeSuit && card.Suit == gs.TrumpSuit {
		return gs.TrumpSuit
	}

	// Otherwise return actual suit
	return card.Suit
}

// CardBeats returns true if card a beats card b in the current game context
func (gs *GameState) CardBeats(a, b Card) bool {
	// Null games: no trumps, must follow suit, A > K > Q > J > 10 > 9 > 8 > 7
	if gs.Mode == ModeNull {
		// Must follow suit - if different suits, can't beat
		if a.Suit != b.Suit {
			return false
		}
		// Use the Card.NullRank() method for consistent ranking
		return a.NullRank() > b.NullRank()
	}

	// Get trump values (higher = stronger)
	aValue := gs.TrumpValue(a)
	bValue := gs.TrumpValue(b)

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

	// Same suit: compare by rank order (Ace > Ten > King > Queen > Nine > Eight > Seven)
	if aSuit == bSuit {
		return a.Rank.SkatRank() > b.Rank.SkatRank()
	}

	// Different non-trump suits: first card wins
	return false
}

// TrumpValue returns the trump hierarchy value (0 = not trump)
// In Skat: ♣J (11) > ♠J (10) > ♥J (9) > ♦J (8) > trump suit cards (by rank)
func (gs *GameState) TrumpValue(card Card) int {
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

// Clone creates a deep copy of the game state
func (gs *GameState) Clone() *GameState {
	var declarer *GamePosition
	if gs.Declarer != nil {
		d := *gs.Declarer
		declarer = &d
	}

	var trickWinner *GamePosition
	if gs.TrickWinner != nil {
		tw := *gs.TrickWinner
		trickWinner = &tw
	}

	var forfeitedPlayer *GamePosition
	if gs.ForfeitedPlayer != nil {
		fp := *gs.ForfeitedPlayer
		forfeitedPlayer = &fp
	}

	clone := &GameState{
		ID:                    gs.ID,
		Code:                  gs.Code,
		SessionID:             gs.SessionID,
		GameNumber:            gs.GameNumber,
		MaxGames:              gs.MaxGames,
		PassPolicy:            gs.PassPolicy,
		TimerEnabled:          gs.TimerEnabled,
		CurrentPlayer:         gs.CurrentPlayer,
		Declarer:              declarer,
		Mode:                  gs.Mode,
		TrumpSuit:             gs.TrumpSuit,
		Phase:                 gs.Phase,
		TrickWinner:           trickWinner,
		TrickStarter:          gs.TrickStarter,
		PlayerScores:          gs.PlayerScores,
		Matadors:              gs.Matadors,
		PlayedHand:            gs.PlayedHand,
		AnnouncedSchneider:    gs.AnnouncedSchneider,
		AnnouncedSchwarz:      gs.AnnouncedSchwarz,
		BidValue:              gs.BidValue,
		ListenerPassed:        gs.ListenerPassed,
		SpeakerPassed:         gs.SpeakerPassed,
		DealerPassed:          gs.DealerPassed,
		Overbid:               gs.Overbid,
		CurrentPlayerDeadline: gs.CurrentPlayerDeadline,
		ForfeitedPlayer:       forfeitedPlayer,
	}

	// Deep copy players
	for i := 0; i < 3; i++ {
		clone.Players[i] = &PlayerState{
			Hand:    append([]Card{}, gs.Players[i].Hand...),
			ID:      gs.Players[i].ID,
			Name:    gs.Players[i].Name,
			IsAgent: gs.Players[i].IsAgent,
		}
	}

	// Copy skat (it's a fixed-size array, so this is a value copy)
	clone.Skat = gs.Skat

	// Deep copy trick
	clone.Trick = append([]Card{}, gs.Trick...)

	// Deep copy completed trick history
	clone.CardsPlayed = make([][]Card, len(gs.CardsPlayed))
	for i, trick := range gs.CardsPlayed {
		clone.CardsPlayed[i] = append([]Card{}, trick...)
	}

	return clone
}

// Bidding methods

// CanBid returns true if the current player can make a bid
func (gs *GameState) CanBid() bool {
	return gs.Phase == PhaseBidding
}

// getNextBidValue returns the next valid bid value (uses the global function)
func (gs *GameState) getNextBidValue() int {
	for _, bid := range ValidBidValues {
		if bid > gs.BidValue {
			return bid
		}
	}
	return 0 // No higher bid available
}

// GetNextBidValue is the exported version for agent access
func (gs *GameState) GetNextBidValue() int {
	return gs.getNextBidValue()
}

// IsSchneider returns true if the game ended with Schneider
// (one side took 90+ points, null games don't have schneider)
func (gs *GameState) IsSchneider() bool {
	if gs.Mode == ModeNull {
		// Null games don't have schneider - it's binary win/lose
		return false
	}
	// In normal games, Schneider means one side got 90+ points
	return gs.DeclarerCardScore() >= 90 || gs.OpponentCardScore() >= 90
}

// IsSchwarz returns true if the game ended with Schwarz
// (one side took all tricks/120 points, null games don't have schwarz)
func (gs *GameState) IsSchwarz() bool {
	if gs.Mode == ModeNull {
		// Null games don't have schwarz - it's binary win/lose
		return false
	}
	// In normal games, Schwarz means one side took all tricks (120 points)
	return gs.DeclarerCardScore() == 120 || gs.OpponentCardScore() == 120
}

// GetGameResult returns the result of the game including schneider/schwarz
func (gs *GameState) GetGameResult() (declarerWon bool, schneider bool, schwarz bool) {
	// Handle forfeit games - winner is determined by who forfeited
	if gs.ForfeitedPlayer != nil {
		declarerWon = gs.Declarer != nil && *gs.ForfeitedPlayer != *gs.Declarer // Declarer wins if they didn't forfeit
		schneider = false
		schwarz = false
		return
	}

	if gs.Mode == ModeNull {
		// Null game: declarer wins if they take no tricks
		declarerWon = gs.DeclarerCardScore() == 0
		// Null games don't have schneider/schwarz
		schneider = false
		schwarz = false
		return
	}

	// Normal games: declarer needs 61+ points to win
	declarerWon = gs.DeclarerCardScore() >= 61

	// Check for schneider (90+ points by one side)
	if declarerWon {
		schneider = gs.OpponentCardScore() < 30 // Opponents got less than 30
		schwarz = gs.OpponentCardScore() == 0   // Opponents got no points
	} else {
		schneider = gs.DeclarerCardScore() < 30 // Declarer got less than 30
		schwarz = gs.DeclarerCardScore() == 0   // Declarer got no points
	}

	return
}

func (gs *GameState) DeclarerCardScore() int {
	if gs.Declarer == nil {
		return 0
	}
	return gs.PlayerScores[*gs.Declarer]
}

func (gs *GameState) OpponentCardScore() int {
	if gs.Declarer == nil {
		return 0
	}
	score := 0
	for _, pos := range AllPositions {
		if pos != *gs.Declarer {
			score += gs.PlayerScores[pos]
		}
	}
	return score
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
