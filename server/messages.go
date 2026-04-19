package server

import (
	"skat/agent"
	"skat/game"
	"skat/logger"
	"time"
)

// Message represents a WebSocket message
type Message struct {
	Type string         `json:"type"`
	Data map[string]any `json:"data"`
}

// handleMessage processes incoming WebSocket messages
func (s *Server) handleMessage(client *Client, msg *Message) {
	logger.Debug("Received message", "type", msg.Type, "profile_id", client.profileID)

	switch msg.Type {
	case "deal":
		s.handleDealMessage(client, msg)
	case "play_card":
		logger.Debug("Handling play_card message", "data", msg.Data)
		s.handlePlayCardMessage(client, msg)
	case "bid":
		s.handleBidMessage(client, msg)
	case "choose_game":
		s.handleChooseGameMessage(client, msg)
	case "skat_decision":
		s.handleSkatDecisionMessage(client, msg)
	case "discard_cards":
		s.handleDiscardCardsMessage(client, msg)
	case "start_next_game":
		s.handleStartNextGameMessage(client, msg)
	default:
		logger.Warning("Unknown message type", "type", msg.Type)
	}
}

func (cm *ClientManager) BroadcastToPlayers(gs *game.GameState, msg *Message) {
	// Collect profile IDs of human players
	var profileIDs []string
	for _, player := range gs.Players {
		if player != nil && !player.IsAgent { // Only send to human players
			profileIDs = append(profileIDs, player.ID)
		}
	}
	cm.BroadcastToClients(profileIDs, msg)
}

func (cm *ClientManager) BroadcastStateChange(gs *game.GameState, msg string, fromPlayer game.GamePosition) {
	for _, player := range gs.Players {
		if player != nil && !player.IsAgent { // Only send to human players
			cm.SendToClient(player.ID, &Message{
				Type: "state_update",
				Data: map[string]any{
					"diff":        gs.SerializeForPlayer(player.ID),
					"description": msg,
					"from_player": fromPlayer,
				},
			})
		}
	}
}

// saveGameResults saves player results when a game completes
func (s *Server) maybeSaveGameResults(gs *game.GameState) {
	if gs.Phase == game.PhaseComplete {
		results := game.Results(gs)
		if err := s.db.SavePlayerResults(results); err != nil {
			logger.Warning("Failed to save player results", "error", err)
		}
	}
}

func (s *Server) BroadcastAIActions(gs *game.GameState) {
	for {
		action := agent.NextAction(gs)
		if action == nil {
			break
		}
		currentPlayer := gs.CurrentPlayer
		time.Sleep(1 * time.Second)
		response, err := action()
		if err != nil {
			s.clients.BroadcastToPlayers(gs, &Message{
				Type: "error",
				Data: map[string]any{"message": err.Error()},
			})
			return
		}
		s.db.SaveGame(*gs)
		s.maybeSaveGameResults(gs)
		s.clients.BroadcastStateChange(gs, response, currentPlayer)
	}
}

// handleDealMessage processes game deal events
func (s *Server) handleDealMessage(client *Client, msg *Message) {
	gameID, ok := msg.Data["game_id"].(string)
	if !ok {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]any{"message": "game_id required"},
		})
		return
	}

	gs, err := s.db.GetGameByID(gameID)
	if err != nil {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]any{"message": err.Error()},
		})
		return
	}
	currentPlayer := gs.CurrentPlayer
	response, err := gs.Deal(client.profileID)
	if err == nil {
		s.db.SaveGame(*gs)
		s.clients.BroadcastStateChange(gs, response, currentPlayer)
		go s.BroadcastAIActions(gs)
	} else {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]any{"message": err.Error()},
		})
	}
}

// handlePlayCardMessage processes card play requests
func (s *Server) handlePlayCardMessage(client *Client, msg *Message) {
	gameID, ok := msg.Data["game_id"].(string)
	if !ok {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]any{"message": "game_id required"},
		})
		return
	}

	gs, err := s.db.GetGameByID(gameID)
	if err != nil {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]any{"message": err.Error()},
		})
		return
	}

	// Parse card (sent as string "rank.suit")
	cardStr, ok := msg.Data["card"].(string)
	if !ok {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]any{"message": "card must be a string"},
		})
		return
	}

	card, err := game.ParseCard(cardStr)
	if err != nil {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]any{"message": "invalid card: " + err.Error()},
		})
		return
	}

	currentPlayer := gs.CurrentPlayer
	response, err := gs.PlayCard(client.profileID, card)
	if err == nil {
		s.db.SaveGame(*gs)
		s.maybeSaveGameResults(gs) // Save player results if game is complete
		s.clients.BroadcastStateChange(gs, response, currentPlayer)
		go s.BroadcastAIActions(gs)
	} else {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]any{"message": err.Error()},
		})
	}
}

// handleBidMessage processes bidding
func (s *Server) handleBidMessage(client *Client, msg *Message) {
	gameID, ok := msg.Data["game_id"].(string)
	if !ok {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]any{"message": "game_id required"},
		})
		return
	}

	gs, err := s.db.GetGameByID(gameID)
	if err != nil {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]any{"message": err.Error()},
		})
		return
	}

	// Parse bid action (frontend sends "bid" field)
	action, ok := msg.Data["bid"].(string)
	if !ok {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]any{"message": "invalid bid action"},
		})
		return
	}

	currentPlayer := gs.CurrentPlayer
	response, err := gs.Bid(client.profileID, action)
	if err == nil {
		s.db.SaveGame(*gs)
		s.clients.BroadcastStateChange(gs, response, currentPlayer)
		go s.BroadcastAIActions(gs)
	} else {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]any{"message": err.Error()},
		})
	}
}

// handleChooseGameMessage processes game mode selection
func (s *Server) handleChooseGameMessage(client *Client, msg *Message) {
	gameID, ok := msg.Data["game_id"].(string)
	if !ok {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]any{"message": "game_id required"},
		})
		return
	}

	gs, err := s.db.GetGameByID(gameID)
	if err != nil {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]any{"message": err.Error()},
		})
		return
	}

	// Parse game mode and trump suit (sent as strings)
	modeStr, ok := msg.Data["mode"].(string)
	if !ok {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]any{"message": "mode must be a string"},
		})
		return
	}
	mode := game.GameMode(modeStr)

	trumpStr, ok := msg.Data["trump"].(string)
	if !ok {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]any{"message": "trump must be a string"},
		})
		return
	}

	trump, err := game.ParseSuit(trumpStr)
	if err != nil {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]any{"message": "invalid trump suit: " + err.Error()},
		})
		return
	}

	currentPlayer := gs.CurrentPlayer
	response, err := gs.DeclareGame(client.profileID, mode, trump)
	if err == nil {
		s.db.SaveGame(*gs)
		s.clients.BroadcastStateChange(gs, response, currentPlayer)
		go s.BroadcastAIActions(gs)
	} else {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]any{"message": err.Error()},
		})
	}
}

// handleSkatDecisionMessage processes the declarer's decision to pick up skat or play hand
func (s *Server) handleSkatDecisionMessage(client *Client, msg *Message) {
	gameID, ok := msg.Data["game_id"].(string)
	if !ok {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]any{"message": "game_id required"},
		})
		return
	}

	gs, err := s.db.GetGameByID(gameID)
	if err != nil {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]any{"message": err.Error()},
		})
		return
	}

	pickup, ok := msg.Data["pickup"].(bool)
	if !ok {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]any{"message": "invalid pickup decision"},
		})
		return
	}

	currentPlayer := gs.CurrentPlayer
	response, err := gs.SkatDecision(client.profileID, pickup)
	if err == nil {
		s.db.SaveGame(*gs)
		s.clients.BroadcastStateChange(gs, response, currentPlayer)
		go s.BroadcastAIActions(gs)
	} else {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]any{"message": err.Error()},
		})
	}
}

// handleDiscardCardsMessage processes the declarer's card discard after picking up skat
func (s *Server) handleDiscardCardsMessage(client *Client, msg *Message) {
	gameID, ok := msg.Data["game_id"].(string)
	if !ok {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]any{"message": "game_id required"},
		})
		return
	}

	gs, err := s.db.GetGameByID(gameID)
	if err != nil {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]any{"message": err.Error()},
		})
		return
	}

	// Parse the two cards to discard (sent as string "rank.suit-rank.suit")
	cardsStr, ok := msg.Data["cards"].(string)
	if !ok {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]any{"message": "cards must be a string"},
		})
		return
	}

	cards, err := game.ParseSkatCards(cardsStr)
	if err != nil {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]any{"message": "invalid cards: " + err.Error()},
		})
		return
	}

	currentPlayer := gs.CurrentPlayer
	response, err := gs.Discard(client.profileID, cards[0], cards[1])
	if err == nil {
		s.db.SaveGame(*gs)
		s.clients.BroadcastStateChange(gs, response, currentPlayer)
		go s.BroadcastAIActions(gs)
	} else {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]any{"message": err.Error()},
		})
	}
}

// handleBidMessage processes bidding
func (s *Server) handleStartNextGameMessage(client *Client, msg *Message) {
	gameID, ok := msg.Data["game_id"].(string)
	if !ok {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]any{"message": "game_id required"},
		})
		return
	}

	gs, err := s.db.GetGameByID(gameID)
	if err != nil {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]any{"message": err.Error()},
		})
		return
	}

	currentPlayer := gs.CurrentPlayer
	response, err := gs.NextGame()
	if err == nil {
		newGameID := gs.ID // NextGame() creates a new game ID
		s.db.SaveGame(*gs)

		// Send start_next_game message to trigger navigation
		s.clients.BroadcastToPlayers(gs, &Message{
			Type: "start_next_game",
			Data: map[string]any{"game_id": newGameID},
		})

		// Also broadcast the state change
		s.clients.BroadcastStateChange(gs, response, currentPlayer)
		go s.BroadcastAIActions(gs)
	} else {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]any{"message": err.Error()},
		})
	}
}
