package server

import (
	"skat/agent"
	"skat/game"
	"skat/logger"
	"time"
)

// Message represents a WebSocket message
type Message struct {
	Type     string         `json:"type"`
	Data     map[string]any `json:"data"`
	ActionID string         `json:"action_id,omitempty"`
}

// handleMessage processes incoming WebSocket messages
func (s *Server) handleMessage(client *Client, msg *Message) {
	logger.Debug("Received message", "type", msg.Type, "profile_id", client.profileID)

	// Add 2 second delay for local development to test loading states
	if !s.IsCloudRun() {
		time.Sleep(2 * time.Second)
	}

	// Validate game and current player for action messages
	if msg.Type != "start_next_game" {
		gameID, ok := msg.Data["game_id"].(string)
		if !ok {
			client.SendMessage(&Message{
				Type:     "error",
				Data:     map[string]any{"message": "game_id required"},
				ActionID: msg.ActionID,
			})
			return
		}

		gs, err := s.db.GetGameByID(gameID)
		if err != nil {
			client.SendMessage(&Message{
				Type:     "error",
				Data:     map[string]any{"message": err.Error()},
				ActionID: msg.ActionID,
			})
			return
		}

		currentPlayer := gs.GetCurrentPlayer()
		if currentPlayer.ID != client.profileID {
			client.SendMessage(&Message{
				Type:     "error",
				Data:     map[string]any{"message": "not your turn"},
				ActionID: msg.ActionID,
			})
			return
		}
	}

	switch msg.Type {
	case "deal":
		s.handleDealMessage(client, msg)
	case "play_card":
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

func (cm *ClientManager) BroadcastStateChange(gs *game.GameState, msg string, fromPlayer game.GamePosition, actionID string) {
	for _, player := range gs.Players {
		if player != nil && !player.IsAgent { // Only send to human players
			stateMsg := &Message{
				Type: "state_update",
				Data: map[string]any{
					"diff":        gs.SerializeForPlayer(player.ID),
					"description": msg,
					"from_player": fromPlayer,
				},
			}
			// Include action_id if present (for acknowledgment)
			if actionID != "" {
				stateMsg.ActionID = actionID
			}
			cm.SendToClient(player.ID, stateMsg)
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

		// Wait longer if this is resolving a trick (3 cards on table)
		time.Sleep(1 * time.Second)

		response, err := action()
		if err != nil {
			logger.Error("Agent encountered an error", err)
			s.clients.BroadcastToPlayers(gs, &Message{
				Type: "error",
				Data: map[string]any{"message": err.Error()},
			})
			return
		}
		s.db.SaveGame(*gs)
		s.maybeSaveGameResults(gs)
		s.clients.BroadcastStateChange(gs, response, currentPlayer, "") // No action_id for AI actions
	}
}

// handleDealMessage processes game deal events
func (s *Server) handleDealMessage(client *Client, msg *Message) {
	gameID := msg.Data["game_id"].(string)
	gs, _ := s.db.GetGameByID(gameID)
	currentPlayer := gs.CurrentPlayer
	response, err := gs.Deal()
	if err == nil {
		s.db.SaveGame(*gs)
		s.clients.BroadcastStateChange(gs, response, currentPlayer, msg.ActionID)
		go s.BroadcastAIActions(gs)
	} else {
		client.SendMessage(&Message{
			Type:     "error",
			Data:     map[string]any{"message": err.Error()},
			ActionID: msg.ActionID,
		})
	}
}

// handlePlayCardMessage processes card play requests
func (s *Server) handlePlayCardMessage(client *Client, msg *Message) {
	gameID := msg.Data["game_id"].(string)
	gs, _ := s.db.GetGameByID(gameID)

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
	response, err := gs.PlayCard(card)
	if err == nil {
		s.db.SaveGame(*gs)
		s.maybeSaveGameResults(gs) // Save player results if game is complete
		s.clients.BroadcastStateChange(gs, response, currentPlayer, msg.ActionID)
		go s.BroadcastAIActions(gs)
	} else {
		client.SendMessage(&Message{
			Type:     "error",
			Data:     map[string]any{"message": err.Error()},
			ActionID: msg.ActionID,
		})
	}
}

// handleBidMessage processes bidding
func (s *Server) handleBidMessage(client *Client, msg *Message) {
	gameID := msg.Data["game_id"].(string)
	gs, _ := s.db.GetGameByID(gameID)

	// Parse bid action (frontend sends "accept" field as boolean)
	accept, ok := msg.Data["accept"].(bool)
	if !ok {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]any{"message": "invalid bid action"},
		})
		return
	}

	currentPlayer := gs.CurrentPlayer
	response, err := gs.Bid(accept)
	if err == nil {
		s.db.SaveGame(*gs)
		s.clients.BroadcastStateChange(gs, response, currentPlayer, msg.ActionID)
		go s.BroadcastAIActions(gs)
	} else {
		client.SendMessage(&Message{
			Type:     "error",
			Data:     map[string]any{"message": err.Error()},
			ActionID: msg.ActionID,
		})
	}
}

// handleChooseGameMessage processes game mode selection
func (s *Server) handleChooseGameMessage(client *Client, msg *Message) {
	gameID := msg.Data["game_id"].(string)
	gs, _ := s.db.GetGameByID(gameID)

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
	response, err := gs.DeclareGame(mode, trump)
	if err == nil {
		s.db.SaveGame(*gs)
		s.clients.BroadcastStateChange(gs, response, currentPlayer, msg.ActionID)
		go s.BroadcastAIActions(gs)
	} else {
		client.SendMessage(&Message{
			Type:     "error",
			Data:     map[string]any{"message": err.Error()},
			ActionID: msg.ActionID,
		})
	}
}

// handleSkatDecisionMessage processes the declarer's decision to pick up skat or play hand
func (s *Server) handleSkatDecisionMessage(client *Client, msg *Message) {
	gameID := msg.Data["game_id"].(string)
	gs, _ := s.db.GetGameByID(gameID)

	pickup, ok := msg.Data["pickup"].(bool)
	if !ok {
		client.SendMessage(&Message{
			Type: "error",
			Data: map[string]any{"message": "invalid pickup decision"},
		})
		return
	}

	currentPlayer := gs.CurrentPlayer
	response, err := gs.SkatDecision(pickup)
	if err == nil {
		s.db.SaveGame(*gs)
		s.clients.BroadcastStateChange(gs, response, currentPlayer, msg.ActionID)
		go s.BroadcastAIActions(gs)
	} else {
		client.SendMessage(&Message{
			Type:     "error",
			Data:     map[string]any{"message": err.Error()},
			ActionID: msg.ActionID,
		})
	}
}

// handleDiscardCardsMessage processes the declarer's card discard after picking up skat
func (s *Server) handleDiscardCardsMessage(client *Client, msg *Message) {
	gameID := msg.Data["game_id"].(string)
	gs, _ := s.db.GetGameByID(gameID)

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
	response, err := gs.Discard(cards[0], cards[1])
	if err == nil {
		s.db.SaveGame(*gs)
		s.clients.BroadcastStateChange(gs, response, currentPlayer, msg.ActionID)
		go s.BroadcastAIActions(gs)
	} else {
		client.SendMessage(&Message{
			Type:     "error",
			Data:     map[string]any{"message": err.Error()},
			ActionID: msg.ActionID,
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
			Type:     "start_next_game",
			Data:     map[string]any{"game_id": newGameID},
			ActionID: msg.ActionID,
		})

		// Also broadcast the state change
		s.clients.BroadcastStateChange(gs, response, currentPlayer, msg.ActionID)
		go s.BroadcastAIActions(gs)
	} else {
		client.SendMessage(&Message{
			Type:     "error",
			Data:     map[string]any{"message": err.Error()},
			ActionID: msg.ActionID,
		})
	}
}
