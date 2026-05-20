package game

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// AddPlayer adds a player to the game
func (gs *GameState) AddPlayer(player *PlayerState) (string, error) {
	if gs.PlayerCount() >= 3 {
		return "", fmt.Errorf("game is full")
	}
	position := gs.GetRandomPosition()
	gs.Players[position] = player
	if gs.PlayerCount() != 3 {
		return fmt.Sprintf("%s joined the game", player.Name), nil
	}
	gs.Phase = PhaseDealing
	return "Game started", nil
}

// HandleMove processes a move from a human player
func (gs *GameState) PlayCard(card Card) (string, error) {
	valid := gs.GetValidMoves()
	found := false
	for _, c := range valid {
		if c == card {
			found = true
			break
		}
	}
	if !found {
		return "", fmt.Errorf("invalid move")
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

	// If trick is complete (3 cards), calculate the winner now
	// This ensures the winner is available before ResolveTrick clears the trick
	if len(gs.Trick) == 3 {
		winner := Dealer
		winCard := gs.Trick[0]
		for i := Listener; i < 3; i++ {
			if gs.CardBeats(gs.Trick[i], winCard) {
				winner = i
				winCard = gs.Trick[i]
			}
		}
		actualWinner := (gs.TrickStarter + winner) % 3
		gs.TrickWinner = &actualWinner
		gs.CurrentPlayer = actualWinner
	} else {
		// Next player
		gs.CurrentPlayer = (gs.CurrentPlayer + 1) % 3
	}
	gs.UpdateCurrentPlayerDeadline()
	return fmt.Sprintf("%v", card), nil
}

func (gs *GameState) ResolveTrick() (string, error) {
	if gs.Mode != ModeRamsch && gs.Declarer == nil {
		return "", fmt.Errorf("declarer not set")
	}
	if gs.TrickWinner == nil {
		return "", fmt.Errorf("trick winner not set")
	}

	// Calculate points based on game mode
	if gs.Mode == ModeRamsch {
		for _, card := range gs.Trick {
			gs.PlayerScores[*gs.TrickWinner] += card.Value()
		}
	} else if gs.Mode == ModeNull {
		// In null games, if declarer takes any trick, game ends immediately
		if *gs.TrickWinner == *gs.Declarer {
			trickPoints := 0
			for _, card := range gs.Trick {
				trickPoints += card.Value()
			}
			if trickPoints == 0 {
				trickPoints = 1
			}
			gs.PlayerScores[*gs.Declarer] += trickPoints
			gs.Phase = PhaseComplete      // Game ends immediately - declarer loses
			gs.CurrentPlayerDeadline = "" // Clear deadline when game ends
			gs.CardsPlayed = append(gs.CardsPlayed, gs.Trick)
			gs.Trick = nil
			declarer := gs.GetPlayerByPosition(*gs.Declarer)
			return fmt.Sprintf("%s lost the null game", declarer.Name), nil // Exit early - game is over
		}
	} else {
		// In normal games, calculate card points
		for _, card := range gs.Trick {
			gs.PlayerScores[*gs.TrickWinner] += card.Value()
		}
	}

	gs.CardsPlayed = append(gs.CardsPlayed, gs.Trick)
	gs.Trick = nil
	if gs.TrickWinner != nil {
		gs.TrickStarter = *gs.TrickWinner
	}

	// Check if game is over
	if len(gs.Players[0].Hand) == 0 {
		gs.Phase = PhaseComplete
		gs.CurrentPlayerDeadline = "" // Clear deadline when game ends
		// Add skat cards to declarer's score in normal games
		if (gs.Mode == ModeSuit || gs.Mode == ModeGrand) && gs.Declarer != nil {
			gs.PlayerScores[*gs.Declarer] += gs.Skat[0].Value() + gs.Skat[1].Value()
		}
	}

	// Trick was completed, broadcast using state diff
	return "Won the trick", nil
}

// HandleBid processes a bidding action
// Bidding in Skat has two phases:
// Phase 1: Speaker (middlehand) bids, Listener (forehand) responds
// Phase 2: Winner of Phase 1 responds, Dealer (rearhand) bids
func (gs *GameState) Bid(accept bool) (string, error) {
	if gs.Phase != PhaseBidding {
		return "", fmt.Errorf("not in bidding phase")
	}

	bidValue := gs.BidValue
	var declarer *GamePosition

	if !accept {
		// Player passes
		switch gs.CurrentPlayer {
		case Speaker:
			// Speaker passes in Phase 1 - Listener wins Phase 1
			gs.SpeakerPassed = true
			if !gs.ListenerPassed {
				// Move to Phase 2: Dealer now sets the bid value
				gs.CurrentPlayer = Dealer
			} else {
				// Speaker passed in Phase 2 against the dealer, dealer wins the bid
				d := Dealer
				declarer = &d
			}
		case Listener:
			// Listener passes
			gs.ListenerPassed = true
			if !gs.SpeakerPassed {
				// Phase 1: Listener passed, Speaker wins Phase 1
				// Move to Phase 2: Dealer now sets the bid value against the Speaker
				gs.CurrentPlayer = Dealer
			} else if !gs.DealerPassed {
				// Phase 2: Listener passed, if the dealer set the bid (i.e. didn't pass)
				// then the dealer becomes declarer.
				d := Dealer
				declarer = &d
			} else {
				// We have Zwangsspiel
			}
			// Both Speaker and Listener passed - bidding will end after pass count check
		case Dealer:
			// Dealer passes in Phase 2
			gs.DealerPassed = true
			// The Phase 1 winner wins overall - bidding will end after pass count check
			if !gs.SpeakerPassed {
				d := Speaker
				declarer = &d
			} else if !gs.ListenerPassed {
				if gs.BidValue == 0 {
					// Both other players passed without bidding. Forehand/listener now
					// chooses whether to play at 0 or pass into the session all-pass policy.
					gs.CurrentPlayer = Listener
				} else {
					d := Listener
					declarer = &d
				}
			} else {
				// We have Zwangsspiel
			}
		}
	} else {
		nextBidValue := gs.getNextBidValue()
		// This is a sanity check if the bid goes too high
		if nextBidValue == 0 {
			gs.SpeakerPassed = true
			gs.ListenerPassed = true
			gs.DealerPassed = true
		}
		// Player accepts or raises
		switch gs.CurrentPlayer {
		case Speaker:
			if !gs.ListenerPassed {
				// Phase 1: Speaker is bidding against Listener
				// Speaker names a value (either first bid or raise after Listener held)
				gs.BidValue = nextBidValue
				gs.CurrentPlayer = Listener
			} else if !gs.DealerPassed {
				// Phase 2: Speaker responds to Dealer by holding
				// Speaker holds, turn back to Dealer who must raise
				gs.CurrentPlayer = Dealer
			} else {
				d := gs.CurrentPlayer
				declarer = &d
			}
		case Listener:
			if !gs.SpeakerPassed {
				// Phase 1: Listener responds to Speaker by holding
				// Listener holds, turn back to Speaker who must raise
				gs.CurrentPlayer = Speaker
			} else if !gs.DealerPassed {
				// Phase 2: Listener responds to Dealer by holding
				// Listener holds, turn back to Dealer who must raise
				gs.CurrentPlayer = Dealer
			} else {
				d := gs.CurrentPlayer
				declarer = &d
			}
		case Dealer:
			// Phase 2: Dealer bids (names a value) against the Phase 1 winner
			gs.BidValue = nextBidValue
			// Turn passes to Phase 1 winner to respond (hold or pass)
			if !gs.ListenerPassed {
				gs.CurrentPlayer = Listener
			} else if !gs.SpeakerPassed {
				gs.CurrentPlayer = Speaker
			} else {
				d := gs.CurrentPlayer
				declarer = &d
			}
		}
	}

	if gs.AllPlayersPassed() {
		switch gs.PassPolicy {
		case PassPolicyReshuffle:
			gs.ResetForRedeal()
			gs.UpdateCurrentPlayerDeadline()
			return "All players passed; cards will be reshuffled", nil
		case PassPolicyRamsch:
			gs.Declarer = nil
			gs.Mode = ModeRamsch
			gs.TrumpSuit = NoSuit
			gs.Phase = PhasePlaying
			gs.CurrentPlayer = Listener
		default:
			// All three passed explicitly - Listener must play by forehand privilege (Zwangsspiel)
			forcedDeclarer := Listener
			declarer = &forcedDeclarer
		}
	}
	if declarer != nil {
		gs.Declarer = declarer
		// Move to skat exchange phase
		gs.Phase = PhaseSkatExchange
		gs.CurrentPlayer = *gs.Declarer
	}
	action := "pass"
	if accept {
		if gs.BidValue > bidValue {
			action = fmt.Sprintf("I raise the bid to %d", gs.BidValue)
		} else {
			action = fmt.Sprintf("I accept %d", bidValue)
		}
	} else if gs.Mode == ModeRamsch && gs.Phase == PhasePlaying && gs.AllPlayersPassed() {
		action = "All players passed; Ramsch begins"
	}

	gs.UpdateCurrentPlayerDeadline()
	return action, nil
}

func (gs *GameState) AllPlayersPassed() bool {
	return gs.BidValue == 0 && gs.ListenerPassed && gs.SpeakerPassed && gs.DealerPassed
}

func (gs *GameState) ResetForRedeal() {
	gs.Phase = PhaseDealing
	gs.Mode = ModeSuit
	gs.TrumpSuit = NoSuit
	gs.Declarer = nil
	gs.BidValue = 0
	gs.CurrentPlayer = Dealer
	gs.ListenerPassed = false
	gs.SpeakerPassed = false
	gs.DealerPassed = false
	gs.Overbid = false
	gs.PlayerScores = [3]int{}
	gs.Trick = nil
	gs.TrickWinner = nil
	gs.CardsPlayed = nil
	gs.ForfeitedPlayer = nil
	gs.PlayedHand = false
	gs.AnnouncedSchneider = false
	gs.AnnouncedSchwarz = false
	for _, player := range gs.Players {
		if player != nil {
			player.Hand = []Card{}
		}
	}
}

// HandleDeal processes the deal action from the dealer before bidding
func (gs *GameState) Deal() (string, error) {
	// Check if we're in dealing phase
	if gs.Phase != PhaseDealing {
		return "", fmt.Errorf("not in dealing phase")
	}
	// Actually deal the cards
	deck := NewDeck()
	// Shuffle
	deck.Shuffle()
	gs.Players[0].Hand = make(Cards, 10)
	gs.Players[1].Hand = make(Cards, 10)
	gs.Players[2].Hand = make(Cards, 10)
	copy(gs.Players[0].Hand, deck[0:10])
	copy(gs.Players[1].Hand, deck[10:20])
	copy(gs.Players[2].Hand, deck[20:30])
	// Skat (2 cards)
	gs.Skat[0] = deck[30]
	gs.Skat[1] = deck[31]
	// Move to bidding phase
	gs.Phase = PhaseBidding
	gs.CurrentPlayer = Speaker // Speaker starts bidding
	gs.UpdateCurrentPlayerDeadline()
	return "Dealt cards", nil
}

// HandleGameDeclaration processes the declarer's game mode choice
func (gs *GameState) DeclareGame(mode GameMode, trumpSuit Suit, announceSchneider bool, announceSchwarz bool) (string, error) {
	if gs.Phase != PhaseDeclarerChoice {
		return "", fmt.Errorf("not in declarer choice phase")
	}

	// Validate announcements
	if announceSchneider && !gs.PlayedHand {
		return "", fmt.Errorf("can only announce schneider when playing hand")
	}
	if announceSchwarz && !announceSchneider {
		return "", fmt.Errorf("can only announce schwarz when announcing schneider")
	}
	if mode == ModeNull && (announceSchneider || announceSchwarz) {
		return "", fmt.Errorf("cannot make announcements in null games")
	}

	gs.Mode = mode
	gs.TrumpSuit = trumpSuit
	gs.AnnouncedSchneider = announceSchneider
	gs.AnnouncedSchwarz = announceSchwarz

	// Calculate and store matadors (will be used throughout the game)
	// countMatadors returns the count; we negate if without Club Jack
	gs.Matadors = gs.countMatadorsWithSign()

	// Calculate the actual game value based on cards and game type
	// Note: This is calculated before playing to validate against bid
	potentialValue := gs.calculatePotentialGameValue()

	// Validate that the declared game can meet the bid value
	if potentialValue < int(gs.BidValue) {
		gs.Overbid = true
		gs.Phase = PhaseComplete
		gs.CurrentPlayerDeadline = "" // Clear deadline when game ends
	} else {
		// Start playing phase
		gs.Phase = PhasePlaying
		gs.CurrentPlayer = Listener // Player to left of dealer (Listener) leads
	}
	gs.UpdateCurrentPlayerDeadline()

	announcement := ""
	if gs.AnnouncedSchwarz {
		announcement = " (announced schwarz)"
	} else if gs.AnnouncedSchneider {
		announcement = " (announced schneider)"
	}
	return fmt.Sprintf("%s %s%s", mode, trumpSuit, announcement), nil
}

// calculatePotentialGameValue calculates the declared game value before play.
// It includes fixed declaration multipliers, but not schneider/schwarz outcomes
// that are only known after the game.
func (gs *GameState) calculatePotentialGameValue() int {
	if gs.Declarer == nil {
		return 0
	}

	if gs.Mode == ModeNull {
		return gs.nullGameValue()
	}

	baseValue := 0
	switch gs.Mode {
	case ModeGrand:
		baseValue = 24
	case ModeSuit:
		switch gs.TrumpSuit {
		case Diamonds:
			baseValue = 9
		case Hearts:
			baseValue = 10
		case Spades:
			baseValue = 11
		case Clubs:
			baseValue = 12
		}
	}

	if baseValue == 0 {
		return 0
	}

	matadorCount := gs.Matadors
	if matadorCount < 0 {
		matadorCount = -matadorCount
	}

	multiplier := 1 + matadorCount
	if gs.PlayedHand {
		multiplier++
	}
	if gs.AnnouncedSchneider {
		multiplier++
	}
	if gs.AnnouncedSchwarz {
		multiplier++
	}

	return baseValue * multiplier
}

// HandleSkatDecision processes the declarer's decision to pick up skat or play hand
func (gs *GameState) SkatDecision(pickup bool) (string, error) {
	if gs.Phase != PhaseSkatExchange {
		return "", fmt.Errorf("not in skat exchange phase")
	}
	if gs.Declarer == nil {
		return "", fmt.Errorf("declarer not set")
	}

	if pickup {
		// Add skat cards to declarer's hand
		gs.Players[*gs.Declarer].Hand = append(gs.Players[*gs.Declarer].Hand, gs.Skat[0], gs.Skat[1])
		// Stay in PhaseSkatExchange so player can discard
		gs.PlayedHand = false
		return "Pick up skat", nil
	} else {
		// Play hand - skip to game declaration
		gs.Phase = PhaseDeclarerChoice
		gs.PlayedHand = true
		return "Playing the hand", nil
	}
}

// HandleDiscard processes the declarer's card discard after picking up skat
func (gs *GameState) Discard(card1, card2 Card) (string, error) {
	if gs.Phase != PhaseSkatExchange {
		return "", fmt.Errorf("not in skat exchange phase")
	}

	if gs.Declarer == nil {
		return "", fmt.Errorf("no declarer set")
	}

	if len(gs.Players[*gs.Declarer].Hand) != 12 {
		return "", fmt.Errorf("must pick up skat before discarding")
	}

	// Remove cards from hand
	hand := gs.Players[*gs.Declarer].Hand
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
		return "", fmt.Errorf("cards not in hand")
	}

	gs.Players[*gs.Declarer].Hand = newHand
	gs.Skat[0] = card1
	gs.Skat[1] = card2

	// Move to game declaration phase
	gs.Phase = PhaseDeclarerChoice

	return "Discarded cards to skat", nil
}

func (gs *GameState) NextGame() (string, error) {
	// Rotate players: Dealer -> Listener, Listener -> Speaker, Speaker -> Dealer
	rotatedPlayers := [3]*PlayerState{
		gs.Players[2], // Speaker becomes Dealer
		gs.Players[0], // Dealer becomes Listener
		gs.Players[1], // Listener becomes Speaker
	}
	gs.Players = rotatedPlayers

	gs.ID = uuid.New().String()
	gs.GameNumber++
	gs.Phase = PhaseDealing
	gs.Mode = ModeSuit
	gs.TrumpSuit = NoSuit
	gs.Declarer = nil
	gs.BidValue = 0
	gs.CurrentPlayer = 0
	gs.ListenerPassed = false
	gs.SpeakerPassed = false
	gs.DealerPassed = false
	gs.Overbid = false
	gs.PlayerScores = [3]int{}
	gs.Trick = nil
	gs.TrickWinner = nil
	gs.CardsPlayed = nil
	gs.ForfeitedPlayer = nil
	gs.PlayedHand = false
	gs.AnnouncedSchneider = false
	gs.AnnouncedSchwarz = false
	for _, player := range gs.Players {
		player.Hand = []Card{}
		player.ReadyForNext = false // Reset ready state for new game
	}
	gs.UpdateCurrentPlayerDeadline()
	return "Started new game", nil
}

// UpdateCurrentPlayerDeadline sets the deadline for the current player (2 minutes from now)
func (gs *GameState) UpdateCurrentPlayerDeadline() {
	if !gs.TimerEnabled {
		gs.CurrentPlayerDeadline = ""
		return
	}

	// Only set deadline during active gameplay phases
	if gs.Phase == PhaseWaitingForPlayers || gs.Phase == PhaseComplete {
		gs.CurrentPlayerDeadline = ""
		return
	}

	// Don't set deadlines for AI players - they move instantly
	currentPlayer := gs.GetCurrentPlayer()
	if currentPlayer != nil && currentPlayer.IsAgent {
		gs.CurrentPlayerDeadline = ""
		return
	}

	deadline := time.Now().UTC().Add(2 * time.Minute)
	gs.CurrentPlayerDeadline = deadline.Format(time.RFC3339)
}

// IsDeadlinePassed checks if the current player's deadline has passed
func (gs *GameState) IsDeadlinePassed() bool {
	if gs.CurrentPlayerDeadline == "" {
		return false
	}

	deadline, err := time.Parse(time.RFC3339, gs.CurrentPlayerDeadline)
	if err != nil {
		return false
	}

	return time.Now().UTC().After(deadline)
}

// ForfeitDueToInactivity forfeits the game for the current player due to inactivity
func (gs *GameState) ForfeitDueToInactivity() []PlayerResultState {
	if gs.Phase == PhaseComplete {
		return nil
	}

	// Mark game as complete and record who forfeited
	gs.Phase = PhaseComplete
	gs.CurrentPlayerDeadline = "" // Clear the deadline
	currentPos := gs.CurrentPlayer
	gs.ForfeitedPlayer = &currentPos

	return nil
}
