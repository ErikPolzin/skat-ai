package game

import (
	"fmt"
	"log"
	"math/rand"

	"github.com/google/uuid"
)

// AddPlayer adds a player to the game
func (gs *GameState) AddPlayer(player *PlayerState) (string, error) {
	if gs.PlayerCount() >= 3 {
		return "", fmt.Errorf("game is full")
	}
	position := gs.GetRandomPosition()
	gs.Players[position] = player
	log.Printf("Player %s joined game %s at position %d", player.Name, gs.ID, position)
	if gs.PlayerCount() != 3 {
		return fmt.Sprintf("%s joined the game", player.Name), nil
	}
	gs.Phase = PhaseDealing
	fmt.Printf("Game %s ready with 3 players", gs.ID)
	return "Game started", nil
}

// HandleMove processes a move from a human player
func (gs *GameState) PlayCard(playerID string, card Card) (string, error) {
	currentPlayer := gs.GetCurrentPlayer()
	if playerID != "" && currentPlayer.ID != playerID {
		return "", fmt.Errorf("not your turn")
	}

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

	if len(gs.Trick) != 3 {
		// Next player
		gs.CurrentPlayer = (gs.CurrentPlayer + 1) % 3
	}
	log.Printf("Player %s played %v", currentPlayer.Name, card)
	return fmt.Sprintf("%v", card), nil
}

func (gs *GameState) ResolveTrick() (string, error) {
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

	// Calculate points based on game mode
	if gs.Mode == ModeNull {
		// In null games, if declarer takes any trick, game ends immediately
		if actualWinner == gs.Declarer {
			gs.Phase = PhaseComplete // Game ends immediately - declarer loses
			gs.CardsPlayed = append(gs.CardsPlayed, gs.Trick)
			gs.Trick = nil
			gs.TrickWinner = actualWinner
			declarer := gs.GetPlayerByPosition(gs.Declarer)
			return fmt.Sprintf("%s lost the null game", declarer.Name), nil // Exit early - game is over
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
	}

	trickWinner := gs.GetPlayerByPosition(gs.TrickWinner)
	log.Printf("Player %s won the trick", trickWinner.Name)
	// Trick was completed, broadcast using state diff
	return "Won the trick", nil
}

// HandleBid processes a bidding action
func (gs *GameState) Bid(playerID string, action string) (string, error) {
	currentPlayer := gs.GetCurrentPlayer()
	if currentPlayer.ID != playerID {
		return "", fmt.Errorf("not your turn to bid")
	}

	if gs.Phase != PhaseBidding {
		return "", fmt.Errorf("not in bidding phase")
	}

	if gs.Phase != PhaseBidding {
		return "", fmt.Errorf("not in bidding phase")
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
		return "", fmt.Errorf("invalid bid action: %s", action)
	}

	switch action {
	case "pass":
		// Player passes
		switch gs.CurrentPlayer {
		case Listener:
			gs.ListenerPassed = true
		case Speaker:
			gs.SpeakerPassed = true
		case Dealer:
			gs.DealerPassed = true
		}
	case "hold":
		// Player matches the current bid
		// Bid value stays the same, other player must raise or pass
	default:
		// Player raises the bid
		var newBid int
		fmt.Sscanf(action, "%d", &newBid)
		gs.BidValue = newBid
	}

	// Determine next player and check if bidding is complete
	gs.advanceBidding()

	log.Printf("Player %s bid: %s", currentPlayer.Name, action)
	return action, nil
}

// HandleDeal processes the deal action from the dealer before bidding
func (gs *GameState) Deal(playerID string) (string, error) {

	// Check if the player is the dealer
	dealer := gs.GetPlayerByPosition(0)
	if dealer == nil || dealer.ID != playerID {
		return "", fmt.Errorf("only the dealer can deal")
	}

	// Check if we're in dealing phase
	if gs.Phase != PhaseDealing {
		return "", fmt.Errorf("not in dealing phase")
	}

	log.Printf("Dealer %s dealing cards", dealer.Name)

	// Actually deal the cards
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

	// Move to bidding phase
	gs.Phase = PhaseBidding
	gs.CurrentPlayer = 2 // Speaker starts bidding
	return "Dealt cards", nil
}

// HandleGameDeclaration processes the declarer's game mode choice
func (gs *GameState) DeclareGame(playerID string, mode GameMode, trumpSuit Suit) (string, error) {
	declarer := gs.GetCurrentPlayer()
	if declarer.ID != playerID {
		return "", fmt.Errorf("only declarer can declare game")
	}

	if gs.Phase != PhaseDeclarerChoice {
		return "", fmt.Errorf("not in declarer choice phase")
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

	log.Printf("Declarer %s declared %s %s", declarer.Name, mode, trumpSuit)
	return fmt.Sprintf("%s %s", mode, trumpSuit), nil
}

// HandleSkatDecision processes the declarer's decision to pick up skat or play hand
func (gs *GameState) SkatDecision(playerID string, pickup bool) (string, error) {

	declarer := gs.GetCurrentPlayer()
	if declarer.ID != playerID {
		return "", fmt.Errorf("only declarer can make skat decision")
	}

	if gs.Phase != PhaseSkatExchange {
		return "", fmt.Errorf("not in skat exchange phase")
	}

	if gs.Phase != PhaseSkatExchange {
		return "", fmt.Errorf("not in skat exchange phase")
	}

	if pickup {
		// Add skat cards to declarer's hand
		gs.Players[gs.Declarer].Hand = append(gs.Players[gs.Declarer].Hand, gs.Skat[0], gs.Skat[1])
		// Stay in PhaseSkatExchange so player can discard
		log.Printf("Declarer %s picked up skat", declarer.Name)
		return "Pick up skat", nil
	} else {
		// Play hand - skip to game declaration
		gs.Phase = PhaseDeclarerChoice
		log.Printf("Declarer %s playing the hand", declarer.Name)
		return "Playing the hand", nil
	}
}

// HandleDiscard processes the declarer's card discard after picking up skat
func (gs *GameState) Discard(playerID string, card1, card2 Card) (string, error) {
	declarer := gs.GetCurrentPlayer()
	if declarer.ID != playerID {
		return "", fmt.Errorf("only declarer can discard cards")
	}

	if gs.Phase != PhaseSkatExchange {
		return "", fmt.Errorf("not in skat exchange phase")
	}

	if gs.Phase != PhaseSkatExchange {
		return "", fmt.Errorf("not in skat exchange phase")
	}

	if len(gs.Players[gs.Declarer].Hand) != 12 {
		return "", fmt.Errorf("must pick up skat before discarding")
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
		return "", fmt.Errorf("cards not in hand")
	}

	gs.Players[gs.Declarer].Hand = newHand
	gs.Skat[0] = card1
	gs.Skat[1] = card2

	// Move to game declaration phase
	gs.Phase = PhaseDeclarerChoice

	log.Printf("Declarer %s discarded cards to skat", declarer.Name)
	return "Discarded cards to skat", nil
}

func (gs *GameState) NextGame() (string, error) {
	gs.ID = uuid.New().String()
	gs.Phase = PhaseDealing
	gs.Declarer = -1
	gs.BidValue = 0
	gs.CurrentPlayer = 0
	gs.ListenerPassed = false
	gs.SpeakerPassed = false
	gs.DealerPassed = false
	for _, player := range gs.Players {
		player.Hand = []Card{}
	}
	log.Printf("Started a new game with ID %s", gs.ID)
	return "Started new game", nil
}
