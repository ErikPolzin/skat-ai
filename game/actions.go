package game

import (
	"fmt"
	"math/rand"
	"skat/logger"

	"github.com/google/uuid"
)

// AddPlayer adds a player to the game
func (gs *GameState) AddPlayer(player *PlayerState) (string, error) {
	if gs.PlayerCount() >= 3 {
		return "", fmt.Errorf("game is full")
	}
	position := gs.GetRandomPosition()
	gs.Players[position] = player
	logger.Info("Player joined game", "player_name", player.Name, "game_id", gs.ID, "position", position)
	if gs.PlayerCount() != 3 {
		return fmt.Sprintf("%s joined the game", player.Name), nil
	}
	gs.Phase = PhaseDealing
	logger.Info("Game ready with 3 players", "game_id", gs.ID)
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

	// If trick is complete (3 cards), calculate the winner now
	// This ensures the winner is available before ResolveTrick clears the trick
	if len(gs.Trick) == 3 {
		winner := Dealer
		winCard := gs.Trick[0]
		for i := Listener; i < 3; i++ {
			if gs.cardBeats(gs.Trick[i], winCard) {
				winner = i
				winCard = gs.Trick[i]
			}
		}
		actualWinner := (gs.TrickStarter + winner) % 3
		gs.TrickWinner = actualWinner
		gs.CurrentPlayer = actualWinner
	} else {
		// Next player
		gs.CurrentPlayer = (gs.CurrentPlayer + 1) % 3
	}
	return fmt.Sprintf("%v", card), nil
}

func (gs *GameState) ResolveTrick() (string, error) {

	// Calculate points based on game mode
	if gs.Mode == ModeNull {
		// In null games, if declarer takes any trick, game ends immediately
		if gs.TrickWinner == gs.Declarer {
			gs.Phase = PhaseComplete // Game ends immediately - declarer loses
			gs.CardsPlayed = append(gs.CardsPlayed, gs.Trick)
			gs.Trick = nil
			declarer := gs.GetPlayerByPosition(gs.Declarer)
			return fmt.Sprintf("%s lost the null game", declarer.Name), nil // Exit early - game is over
		}
	} else {
		// In normal games, calculate card points
		for _, card := range gs.Trick {
			if gs.TrickWinner == gs.Declarer {
				gs.DeclarerScore += card.Value()
			} else {
				gs.OpponentScore += card.Value()
			}
		}
	}

	gs.CardsPlayed = append(gs.CardsPlayed, gs.Trick)
	gs.Trick = nil
	gs.TrickStarter = gs.TrickWinner

	// Check if game is over
	if len(gs.Players[0].Hand) == 0 {
		gs.Phase = PhaseComplete
		// Add skat cards to declarer's score in normal games
		if gs.Mode != ModeNull && gs.Declarer >= 0 {
			gs.DeclarerScore += gs.Skat[0].Value() + gs.Skat[1].Value()
		}
	}

	// Trick was completed, broadcast using state diff
	return "Won the trick", nil
}

// HandleBid processes a bidding action
func (gs *GameState) Bid(playerID string, accept bool) (string, error) {
	currentPlayer := gs.GetCurrentPlayer()
	if currentPlayer.ID != playerID {
		return "", fmt.Errorf("not your turn to bid")
	}

	if gs.Phase != PhaseBidding {
		return "", fmt.Errorf("not in bidding phase")
	}

	bidValue := gs.BidValue

	if !accept {
		// Player passes
		switch gs.CurrentPlayer {
		case Speaker:
			gs.SpeakerPassed = true
			gs.CurrentPlayer = Listener
		case Listener:
			gs.ListenerPassed = true
			gs.CurrentPlayer = Dealer
		case Dealer:
			gs.DealerPassed = true
		}
	} else {
		switch gs.CurrentPlayer {
		case Speaker:
			if !gs.ListenerPassed {
				gs.CurrentPlayer = Listener
				// Speaker raises the bid when bidding speaker-listener
				gs.BidValue = gs.getNextBidValue()
			} else {
				gs.CurrentPlayer = Dealer
			}
		case Listener:
			if !gs.SpeakerPassed {
				gs.CurrentPlayer = Speaker
			} else {
				gs.CurrentPlayer = Dealer
				// Listener raises the bid when bidding listener-dealer
				gs.BidValue = gs.getNextBidValue()
			}
		case Dealer:
			if !gs.ListenerPassed {
				gs.CurrentPlayer = Listener
			} else {
				gs.CurrentPlayer = Speaker
				// Dealer raises the bid when bidding dealer-speaker
				gs.BidValue = gs.getNextBidValue()
			}
		}
	}

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
		}
		// Move to skat exchange phase
		gs.Phase = PhaseSkatExchange
		gs.CurrentPlayer = gs.Declarer
	}
	action := "pass"
	if accept {
		if gs.BidValue > bidValue {
			action = fmt.Sprintf("I raise the bid to %d", gs.BidValue)
		} else {
			action = fmt.Sprintf("I accept %d", bidValue)
		}
	}

	logger.Debug("Player bid", "player_name", currentPlayer.Name, "accept", accept, "bid value", gs.BidValue)
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

	logger.Debug("Dealer dealing cards", "dealer_name", dealer.Name)

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

	// Calculate the actual game value based on cards and game type
	// Note: This is calculated before playing to validate against bid
	// The final game value will be recalculated after the game ends
	gameValue := gs.calculatePotentialGameValue()

	// Validate that the declared game can meet the bid value
	if gameValue < int(gs.BidValue) {
		return "", fmt.Errorf("game value %d is less than bid value %d", gameValue, gs.BidValue)
	}

	gs.GameValue = gameValue

	// Start playing phase
	gs.Phase = PhasePlaying
	gs.CurrentPlayer = Dealer // Player to left of dealer leads
	if Dealer == 0 {
		gs.CurrentPlayer = 1
	}

	logger.Info("Declarer declared game", "declarer_name", declarer.Name, "mode", string(mode), "trump_suit", trumpSuit.String(), "value", gameValue)
	return fmt.Sprintf("%s %s", mode, trumpSuit), nil
}

// calculatePotentialGameValue calculates the game value assuming the declarer wins normally
// (without schneider or schwarz). Used for validating the game declaration against the bid.
func (gs *GameState) calculatePotentialGameValue() int {
	// Use the Cards.GameValue method for consistency
	hand := Cards(gs.Players[gs.Declarer].Hand)
	return hand.GameValue(gs.Mode, gs.TrumpSuit)
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
		logger.Debug("Declarer picked up skat", "declarer_name", declarer.Name)
		return "Pick up skat", nil
	} else {
		// Play hand - skip to game declaration
		gs.Phase = PhaseDeclarerChoice
		logger.Debug("Declarer playing the hand", "declarer_name", declarer.Name)
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

	logger.Debug("Declarer discarded cards to skat", "declarer_name", declarer.Name)
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
	gs.DeclarerScore = 0
	gs.OpponentScore = 0
	for _, player := range gs.Players {
		player.Hand = []Card{}
	}
	logger.Info("Started a new game", "game_id", gs.ID)
	return "Started new game", nil
}
