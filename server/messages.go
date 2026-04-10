package server

import (
	"fmt"
	"log"
	"skat/game"
)

// Message represents a WebSocket message
type Message struct {
	Type string                 `json:"type"`
	Data map[string]interface{} `json:"data"`
}

// handleMessage processes incoming WebSocket messages
func (s *Server) handleMessage(client *Client, msg *Message) {
	log.Printf("Received message type: %s from profile: %s", msg.Type, client.profileID)

	switch msg.Type {
	case "deal":
		s.handleDealMessage(client, msg)
	case "play_card":
		log.Printf("Handling play_card message with data: %+v", msg.Data)
		s.handlePlayCardMessage(client, msg)
	case "bid":
		s.handleBidMessage(client, msg)
	case "choose_game":
		s.handleChooseGameMessage(client, msg)
	case "skat_decision":
		s.handleSkatDecisionMessage(client, msg)
	case "discard_cards":
		s.handleDiscardCardsMessage(client, msg)
	default:
		log.Printf("Unknown message type: %s", msg.Type)
	}
}

// handleDealMessage processes game deal events
func (s *Server) handleDealMessage(client *Client, msg *Message) {
	gameID, ok := msg.Data["game_id"].(string)
	if !ok {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]interface{}{"message": "game_id required"},
		})
		return
	}

	game, err := s.GetGame(gameID)
	if err != nil {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]interface{}{"message": err.Error()},
		})
		return
	}

	if err := game.HandleDeal(client.profileID); err != nil {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]interface{}{"message": err.Error()},
		})
		return
	}
}

// handlePlayCardMessage processes card play requests
func (s *Server) handlePlayCardMessage(client *Client, msg *Message) {
	gameID, ok := msg.Data["game_id"].(string)
	if !ok {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]interface{}{"message": "game_id required"},
		})
		return
	}

	game, err := s.GetGame(gameID)
	if err != nil {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]interface{}{"message": err.Error()},
		})
		return
	}

	// Parse card
	cardData, ok := msg.Data["card"].(map[string]interface{})
	if !ok {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]interface{}{"message": "invalid card data"},
		})
		return
	}

	card, err := parseCard(cardData)
	if err != nil {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]interface{}{"message": err.Error()},
		})
		return
	}

	if err := game.HandleMove(client.profileID, card); err != nil {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]interface{}{"message": err.Error()},
		})
		return
	}
}

// handleBidMessage processes bidding
func (s *Server) handleBidMessage(client *Client, msg *Message) {
	gameID, ok := msg.Data["game_id"].(string)
	if !ok {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]interface{}{"message": "game_id required"},
		})
		return
	}

	game, err := s.GetGame(gameID)
	if err != nil {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]interface{}{"message": err.Error()},
		})
		return
	}

	// Parse bid action (frontend sends "bid" field)
	action, ok := msg.Data["bid"].(string)
	if !ok {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]interface{}{"message": "invalid bid action"},
		})
		return
	}

	if err := game.HandleBid(client.profileID, action); err != nil {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]interface{}{"message": err.Error()},
		})
		return
	}
}

// handleChooseGameMessage processes game mode selection
func (s *Server) handleChooseGameMessage(client *Client, msg *Message) {
	gameID, ok := msg.Data["game_id"].(string)
	if !ok {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]interface{}{"message": "game_id required"},
		})
		return
	}

	game, err := s.GetGame(gameID)
	if err != nil {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]interface{}{"message": err.Error()},
		})
		return
	}

	// Parse game mode and trump suit
	modeStr, ok := msg.Data["mode"].(string)
	if !ok {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]interface{}{"message": "invalid game mode"},
		})
		return
	}

	trumpStr, _ := msg.Data["trump"].(string)

	if err := game.HandleGameDeclaration(client.profileID, modeStr, trumpStr); err != nil {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]interface{}{"message": err.Error()},
		})
		return
	}
}

// parseCard converts JSON data to a Card
func parseCard(data map[string]interface{}) (game.Card, error) {
	suitStr, ok := data["suit"].(string)
	if !ok {
		return game.Card{}, fmt.Errorf("invalid suit")
	}

	rankStr, ok := data["rank"].(string)
	if !ok {
		return game.Card{}, fmt.Errorf("invalid rank")
	}

	suit, err := parseSuit(suitStr)
	if err != nil {
		return game.Card{}, err
	}

	rank, err := parseRank(rankStr)
	if err != nil {
		return game.Card{}, err
	}

	return game.Card{Suit: suit, Rank: rank}, nil
}

// parseSuit converts string to Suit
func parseSuit(s string) (game.Suit, error) {
	switch s {
	case "Clubs", "clubs", "♣":
		return game.Clubs, nil
	case "Spades", "spades", "♠":
		return game.Spades, nil
	case "Hearts", "hearts", "♥":
		return game.Hearts, nil
	case "Diamonds", "diamonds", "♦":
		return game.Diamonds, nil
	default:
		return 0, fmt.Errorf("invalid suit: %s", s)
	}
}

// parseRank converts string to Rank
func parseRank(s string) (game.Rank, error) {
	switch s {
	case "Seven", "7":
		return game.Seven, nil
	case "Eight", "8":
		return game.Eight, nil
	case "Nine", "9":
		return game.Nine, nil
	case "Ten", "10":
		return game.Ten, nil
	case "Jack", "J":
		return game.Jack, nil
	case "Queen", "Q":
		return game.Queen, nil
	case "King", "K":
		return game.King, nil
	case "Ace", "A":
		return game.Ace, nil
	default:
		return 0, fmt.Errorf("invalid rank: %s", s)
	}
}

// handleSkatDecisionMessage processes the declarer's decision to pick up skat or play hand
func (s *Server) handleSkatDecisionMessage(client *Client, msg *Message) {
	gameID, ok := msg.Data["game_id"].(string)
	if !ok {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]interface{}{"message": "game_id required"},
		})
		return
	}

	gameSession, err := s.GetGame(gameID)
	if err != nil {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]interface{}{"message": err.Error()},
		})
		return
	}

	pickup, ok := msg.Data["pickup"].(bool)
	if !ok {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]interface{}{"message": "invalid pickup decision"},
		})
		return
	}

	if err := gameSession.HandleSkatDecision(client.profileID, pickup); err != nil {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]interface{}{"message": err.Error()},
		})
		return
	}
}

// handleDiscardCardsMessage processes the declarer's card discard after picking up skat
func (s *Server) handleDiscardCardsMessage(client *Client, msg *Message) {
	gameID, ok := msg.Data["game_id"].(string)
	if !ok {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]interface{}{"message": "game_id required"},
		})
		return
	}

	gameSession, err := s.GetGame(gameID)
	if err != nil {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]interface{}{"message": err.Error()},
		})
		return
	}

	// Parse the two cards to discard
	cardsData, ok := msg.Data["cards"].([]interface{})
	if !ok || len(cardsData) != 2 {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]interface{}{"message": "must discard exactly 2 cards"},
		})
		return
	}

	var card1, card2 game.Card
	for i, cardData := range cardsData {
		cardMap, ok := cardData.(map[string]interface{})
		if !ok {
			client.SendMessage(&Message{
				Type: "error",
				Data: map[string]interface{}{"message": "invalid card data"},
			})
			return
		}

		parsedCard, err := parseCard(cardMap)
		if err != nil {
			client.SendMessage(&Message{
				Type: "error",
				Data: map[string]interface{}{"message": err.Error()},
			})
			return
		}
		if i == 0 {
			card1 = parsedCard
		} else {
			card2 = parsedCard
		}
	}

	if err := gameSession.HandleDiscard(client.profileID, card1, card2); err != nil {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]interface{}{"message": err.Error()},
		})
		return
	}
}

// Helper to serialize game state for client
func serializeGameState(gs *game.GameState, playerID string) map[string]interface{} {
	// Find player position
	playerPos := -1
	// You'd need to track this mapping in the room

	var hand []map[string]interface{}
	if playerPos >= 0 && playerPos < 3 {
		hand = cardsToJSON(gs.Players[playerPos].Hand)
	}

	return map[string]interface{}{
		"current_player": gs.CurrentPlayer,
		"phase":          phaseToString(gs.Phase),
		"trick":          cardsToJSON(gs.Trick),
		"hand":           hand,
		"declarer":       gs.Declarer,
		"declarer_score": gs.DeclarerScore,
	}
}
