package server

import (
	"fmt"
	"log"
	"math/rand"
	"skat/agent"
	"skat/game"
	"slices"
	"sync"
	"time"
)

// GameSession represents a single Skat game session
type GameSession struct {
	ID           string
	Code         string // Short memorable code for joining
	Players      map[string]*Player
	GameState    *game.GameState
	mutex        sync.RWMutex
	autoStarting bool    // Prevents double auto-start
	server       *Server // Reference to server for database access
}

// Player represents a player in the game
type Player struct {
	ID       string            // Can be profile ID for humans or agent_X for AI
	Name     string
	Agent    agent.Agent       // Can be nil for human players
	Position game.GamePosition // 0, 1, or 2
}

// GameInfo contains public game information
type GameInfo struct {
	ID                string            `json:"id"`
	Code              string            `json:"code"`
	Players           []*PlayerInfo     `json:"players"`
	CurrentPlayer     game.GamePosition `json:"current_player"`
	Phase             string            `json:"phase"`
	Trick             []CardInfo        `json:"trick"`
	DeclarerScore     int               `json:"declarer_score"`
	OpponentScore     int               `json:"opponent_score"`
	Declarer          game.GamePosition `json:"declarer"`
	GameOver          bool              `json:"game_over"`
	GameMode          string            `json:"game_mode,omitempty"`
	TrumpSuit         string            `json:"trump_suit,omitempty"`
	ValidBids         []string          `json:"valid_bids,omitempty"`
	PlayerID          string            `json:"player_id,omitempty"`
	Hand              []CardInfo        `json:"hand,omitempty"`
	DeclarerPileCount int               `json:"declarer_pile_count,omitempty"`
	OpponentPileCount int               `json:"opponent_pile_count,omitempty"`
	TrickStarter      game.GamePosition `json:"trick_starter,omitempty"`
	TrickWinner       game.GamePosition `json:"trick_winner,omitempty"`
	DeclarerTricks    int               `json:"declarer_tricks,omitempty"` // For null games
}

// CardInfo contains card information
type CardInfo struct {
	Suit string `json:"suit"`
	Rank string `json:"rank"`
}

// PlayerInfo contains public player information
type PlayerInfo struct {
	PlayerID  string            `json:"player_id"`
	Name      string            `json:"name"`
	Position  game.GamePosition `json:"position"`
	IsAgent   bool              `json:"is_agent,omitempty"`
	CardCount int               `json:"card_count,omitempty"`
}

func NewGame(id string, code string, server *Server) *GameSession {
	return &GameSession{
		ID:      id,
		Code:    code,
		Players: make(map[string]*Player),
		server:  server,
	}
}

// AddPlayer adds a player to the game
func (r *GameSession) AddPlayer(playerID, playerName string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Check if player already exists (reconnection)
	if _, exists := r.Players[playerID]; exists {
		return nil
	}

	if len(r.Players) >= 3 {
		return fmt.Errorf("game is full")
	}

	if r.GameState != nil {
		return fmt.Errorf("game already started")
	}

	player := &Player{
		ID:       playerID,
		Name:     playerName,
		Position: r.getRandomPosition(),
	}

	r.Players[playerID] = player

	log.Printf("Player %s joined game %s at position %d", playerName, r.ID, player.Position)

	// Broadcast player joined using state diff
	r.broadcastStateChange("player_joined", playerID, nil)

	// Try to auto-start if we have 3 players
	r.tryAutoStart()

	return nil
}

// ConnectPlayer is no longer needed since we don't track current game per client
// Each message includes the game_id it's for

// AddAgent adds an AI agent to the game
func (r *GameSession) AddAgent(agentType string, agentName string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if len(r.Players) >= 3 {
		return fmt.Errorf("game is full")
	}

	playerID := fmt.Sprintf("agent_%d", len(r.Players))

	var ag agent.Agent
	switch agentType {
	case "random":
		ag = agent.NewRandomAgent(agentName)
	case "mcts":
		ag = agent.NewMCTSAgent(agentName, 100)
	default:
		return fmt.Errorf("unknown agent type: %s", agentType)
	}

	player := &Player{
		ID:       playerID,
		Name:     agentName,
		Agent:    ag,
		Position: r.getRandomPosition(),
	}

	r.Players[playerID] = player

	log.Printf("Agent %s (%s) joined game %s at position %d", agentName, agentType, r.ID, player.Position)

	// Broadcast agent joined using state diff
	r.broadcastStateChange("player_joined", playerID, nil)

	// Try to auto-start if we have 3 players
	r.tryAutoStart()

	return nil
}

// tryAutoStart checks if we have 3 players and auto-starts the game
func (r *GameSession) tryAutoStart() {
	// Only auto-start if we don't have a game yet and have exactly 3 players
	if r.GameState == nil && len(r.Players) == 3 && !r.autoStarting {
		r.autoStarting = true
		log.Printf("Auto-starting game %s with 3 players", r.ID)
		go func() {
			// Small delay to ensure all players are ready
			time.Sleep(100 * time.Millisecond)
			if err := r.StartGame(); err != nil {
				log.Printf("Failed to auto-start game %s: %v", r.ID, err)
			}
		}()
	}
}

// DisconnectPlayer marks a player as offline (disconnected)
func (r *GameSession) DisconnectPlayer(profileID string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	player, exists := r.Players[profileID]
	if !exists {
		return
	}

	log.Printf("Player %s disconnected from game %s", player.Name, r.ID)

	// Broadcast player disconnected using state diff
	r.broadcastStateChange("player_disconnected", profileID, map[string]interface{}{
		"name": player.Name,
	})
}

// StartGame initializes the game
func (r *GameSession) StartGame() error {

	if r.GameState != nil {
		return fmt.Errorf("game already started")
	}

	if len(r.Players) != 3 {
		return fmt.Errorf("need exactly 3 players to start")
	}

	r.GameState = game.NewGame()

	log.Printf("Game started with ID %s", r.ID)

	// Broadcast game started to all players
	r.broadcastStateChange("game_started", "", nil)

	// Send player-specific data (hand) to each player
	for _, player := range r.Players {
		r.sendPlayerSpecificUpdate(player, "game_started", map[string]interface{}{
			"position": player.Position,
		})
	}

	r.gameLoop()

	return nil
}

// gameLoop manages the game flow
func (r *GameSession) gameLoop() {
	if r.GameState.Phase == game.PhaseComplete {
		r.handleGameComplete()
		return
	}

	r.mutex.RLock()
	phase := r.GameState.Phase
	currentPlayer := r.getCurrentPlayer()
	r.mutex.RUnlock()

	if currentPlayer != nil {
		log.Printf("Game loop: phase=%v, currentPlayer=%s (position=%d), isAgent=%v",
			phase, currentPlayer.Name, currentPlayer.Position, currentPlayer.Agent != nil)
	} else {
		log.Printf("Game loop: phase=%v, currentPlayer=nil", phase)
		return
	}
	if currentPlayer.Agent == nil {
		log.Printf("Skipping game loop: waiting for current player for input")
		return
	}

	switch phase {
	case game.PhaseDealing:
		// Dealing phase
		log.Printf("Processing AI deal")
		r.processAgentDeal(currentPlayer)
	case game.PhaseBidding:
		// Bidding phase
		log.Printf("Processing AI bid for %s", currentPlayer.Name)
		r.processAgentBid(currentPlayer)
	case game.PhaseDeclarerChoice:
		// AI agent declares game automatically
		r.processAgentDeclaration(currentPlayer)
	case game.PhasePlaying:
		// Agent turn
		r.processAgentTurn(currentPlayer)
	}
}

func (r *GameSession) processAgentDeal(player *Player) {
	// Add delay before AI deals (2 seconds)
	time.Sleep(2 * time.Second)

	// Call HandleMove without holding the lock
	err := r.HandleDeal(player.ID)
	if err != nil {
		log.Printf("Error handling AI deal: %v", err)
	}
}

// processAgentDeclaration handles an AI agent's declaration
func (r *GameSession) processAgentDeclaration(player *Player) {
	// Add delay before AI declares game (2 seconds)
	time.Sleep(2 * time.Second)

	r.mutex.Lock()
	// AI always picks suit game with trump based on strongest suit
	// Count cards per suit in agent's hand
	suitCounts := make(map[game.Suit]int)
	position := player.Position
	for _, card := range r.GameState.Players[position].Hand {
		if card.Rank != game.Jack { // Jacks are trump in all games
			suitCounts[card.Suit]++
		}
	}
	r.mutex.Unlock()

	// Find suit with most cards
	maxCount := 0
	var trump game.Suit
	for suit, count := range suitCounts {
		if count > maxCount {
			maxCount = count
			trump = suit
		}
	}

	// Default to clubs if no clear winner
	if maxCount == 0 {
		trump = game.Clubs
	}

	// Call HandleGameDeclaration without holding the lock
	err := r.HandleGameDeclaration(player.ID, "suit", trump.String())
	if err != nil {
		log.Printf("Error handling AI declaration: %v", err)
	}
}

func (r *GameSession) processAgentTurn(player *Player) {
	// Add delay before AI makes move (2 seconds)
	time.Sleep(2 * time.Second)

	r.mutex.Lock()
	validMoves := r.GameState.GetValidMoves()
	r.mutex.Unlock()

	if len(validMoves) == 0 {
		log.Printf("No valid moves for AI %s", player.Name)
		return
	}

	move := player.Agent.SelectMove(r.GameState, validMoves)

	// Call HandleMove without holding the lock
	err := r.HandleMove(player.ID, move)
	if err != nil {
		log.Printf("Error handling AI move: %v", err)
	}
}

// processAgentBid handles an AI agent's bidding turn
func (r *GameSession) processAgentBid(player *Player) {
	// Add delay before AI makes bid (1 second)
	time.Sleep(1 * time.Second)

	// Get valid bids with lock
	r.mutex.Lock()
	validBids := r.GameState.GetValidBids()
	r.mutex.Unlock()

	if len(validBids) == 0 {
		log.Printf("No valid bids for AI %s", player.Name)
		return
	}

	// Use the agent's Bid method for intelligent bidding
	var action string

	if player.Agent != nil {
		// Get a copy of the game state for the agent
		r.mutex.Lock()
		currentBid := r.GameState.BidValue
		stateCopy := *r.GameState // Make a copy
		r.mutex.Unlock()

		// Call the agent's Bid method
		agentBid := player.Agent.Bid(&stateCopy, currentBid)

		if agentBid == 0 {
			// Agent wants to pass
			action = "pass"
		} else {
			// Agent wants to bid or hold
			// Check if the agent's bid matches any valid action
			bidStr := fmt.Sprintf("%d", agentBid)

			// Check if this bid value is in valid bids
			for _, validBid := range validBids {
				if validBid == bidStr {
					action = bidStr
					break
				} else if validBid == "hold" && agentBid == currentBid {
					// Agent wants to hold at current bid
					action = "hold"
					break
				}
			}

			// If agent's bid isn't valid, default to pass
			if action == "" {
				action = "pass"
			}
		}
	} else {
		// Fallback if no agent (shouldn't happen)
		action = "pass"
	}

	log.Printf("AI %s choosing to %s", player.Name, action)

	// Call HandleBid without holding the lock
	err := r.HandleBid(player.ID, action)
	if err != nil {
		log.Printf("Error handling AI bid: %v", err)
	}
}

// HandleMove processes a move from a human player
func (r *GameSession) HandleMove(playerID string, card game.Card) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	currentPlayer := r.getCurrentPlayer()
	if currentPlayer.ID != playerID {
		return fmt.Errorf("not your turn")
	}

	err := r.GameState.PlayCard(card)
	if err != nil {
		return err
	}

	log.Printf("Player %s played %v", currentPlayer.Name, card)

	// Broadcast card played using state diff
	r.broadcastStateChange("card_played", playerID, map[string]interface{}{
		"card": cardToJSON(card),
	})

	// Check if trick is complete
	if len(r.GameState.Trick) == 3 {
		// Add delay after trick completion before next trick
		time.Sleep(2 * time.Second)
		trickWinner := r.getPlayerByPosition(r.GameState.TrickWinner)
		log.Printf("Player %s won the trick", trickWinner.Name)
		r.GameState.ResolveTrick()
		// Trick was completed, broadcast using state diff
		r.broadcastStateChange("trick_complete", "", map[string]interface{}{
			"winner": r.GameState.TrickWinner,
		})
	}

	// Continue game loop
	go r.gameLoop()

	return nil
}

// HandleBid processes a bidding action
func (r *GameSession) HandleBid(playerID string, action string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	currentPlayer := r.getCurrentPlayer()
	if currentPlayer.ID != playerID {
		return fmt.Errorf("not your turn to bid")
	}

	if r.GameState.Phase != game.PhaseBidding {
		return fmt.Errorf("not in bidding phase")
	}

	err := r.GameState.MakeBid(action)
	if err != nil {
		return err
	}

	log.Printf("Player %s bid: %s", currentPlayer.Name, action)

	// Broadcast bid action using state diff
	r.broadcastStateChange("bid_made", playerID, map[string]interface{}{
		"action": action,
	})

	// Check if bidding is complete
	if r.GameState.Phase == game.PhaseSkatExchange {
		// Bidding complete, notify declarer
		declarer := r.getCurrentPlayer()
		r.broadcastStateChange("bidding_complete", "", map[string]interface{}{
			"declarer": declarer.ID,
		})

		// Show skat cards to declarer for pickup decision
		r.sendPlayerSpecificUpdate(declarer, "skat_available", map[string]interface{}{
			"skat": cardsToJSON(r.GameState.Skat[:]),
		})

		// Continue to skat exchange phase
		go r.gameLoop()
	} else {
		// Continue bidding, notify next bidder
		go r.gameLoop()
	}

	return nil
}

// HandleDeal processes the deal action from the dealer before bidding
func (r *GameSession) HandleDeal(playerID string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Check if the player is the dealer
	dealer := r.getPlayerByPosition(0)
	if dealer == nil || dealer.ID != playerID {
		return fmt.Errorf("only the dealer can deal")
	}

	// Check if we're in dealing phase
	if r.GameState.Phase != game.PhaseDealing {
		return fmt.Errorf("not in dealing phase")
	}

	log.Printf("Dealer %s dealing cards", dealer.Name)

	// Actually deal the cards
	r.GameState.Deal()

	// Move to bidding phase
	r.GameState.Phase = game.PhaseBidding
	r.GameState.CurrentPlayer = 2 // Speaker starts bidding

	// Broadcast deal event
	r.broadcastStateChange("cards_dealt", playerID, nil)

	// Send hands to all human players (not AI agents)
	for _, player := range r.Players {
		if player.Agent == nil {
			r.sendPlayerSpecificUpdate(player, "cards_dealt", nil)
		}
	}

	// Start bidding phase
	go r.gameLoop()

	return nil
}

// HandleGameDeclaration processes the declarer's game mode choice
func (r *GameSession) HandleGameDeclaration(playerID string, modeStr string, trumpStr string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	declarer := r.getCurrentPlayer()
	if declarer.ID != playerID {
		return fmt.Errorf("only declarer can declare game")
	}

	if r.GameState.Phase != game.PhaseDeclarerChoice {
		return fmt.Errorf("not in declarer choice phase")
	}

	// Parse mode
	var mode game.GameMode
	switch modeStr {
	case "grand":
		mode = game.ModeGrand
	case "suit":
		mode = game.ModeSuit
	case "null":
		mode = game.ModeNull
	default:
		return fmt.Errorf("invalid game mode: %s", modeStr)
	}

	// Parse trump suit (only needed for suit games)
	var trumpSuit game.Suit
	if mode == game.ModeSuit {
		var err error
		trumpSuit, err = parseSuit(trumpStr)
		if err != nil {
			return err
		}
	}

	err := r.GameState.DeclareGame(mode, trumpSuit)
	if err != nil {
		return err
	}

	log.Printf("Declarer %s declared %s %s", declarer.Name, modeStr, trumpStr)

	// Broadcast game declaration using state diff
	r.broadcastStateChange("game_declared", playerID, map[string]interface{}{
		"mode":  modeStr,
		"trump": trumpStr,
	})

	// Continue game loop for playing phase
	go r.gameLoop()

	return nil
}

// getCurrentPlayer gets the player object for the current player
func (r *GameSession) getCurrentPlayer() *Player {
	for _, player := range r.Players {
		if player.Position == r.GameState.CurrentPlayer {
			return player
		}
	}
	return nil
}

// get a position that hasn't been assigned yet
func (r *GameSession) getRandomPosition() game.GamePosition {
	availablePositions := []game.GamePosition{game.Dealer, game.Listener, game.Speaker}
	for _, player := range r.Players {
		i := slices.Index(availablePositions, player.Position)
		if i != -1 {
			availablePositions = slices.Delete(availablePositions, i, i+1)
		}
	}
	if len(availablePositions) == 0 {
		return -1
	}
	return availablePositions[rand.Int()%len(availablePositions)]
}

// get a position that hasn't been assigned yet
func (r *GameSession) getRandomAgentName() string {
	availableNames := []string{
		"Bill",
		"Dave",
		"Lisa",
	}
	for _, player := range r.Players {
		if player.Agent != nil {
			i := slices.Index(availableNames, player.Name)
			if i != -1 {
				availableNames = slices.Delete(availableNames, i, i+1)
			}
		}

	}
	if len(availableNames) == 0 {
		return "John"
	}
	return availableNames[rand.Int()%len(availableNames)]
}

// HandleSkatDecision processes the declarer's decision to pick up skat or play hand
func (r *GameSession) HandleSkatDecision(playerID string, pickup bool) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	declarer := r.getCurrentPlayer()
	if declarer.ID != playerID {
		return fmt.Errorf("only declarer can make skat decision")
	}

	if r.GameState.Phase != game.PhaseSkatExchange {
		return fmt.Errorf("not in skat exchange phase")
	}

	err := r.GameState.DecideSkatPickup(pickup)
	if err != nil {
		return err
	}

	if pickup {
		// Broadcast that declarer picked up skat
		r.broadcastStateChange("skat_picked_up", playerID, nil)

		// Send updated hand to declarer (now with 12 cards)
		r.sendPlayerSpecificUpdate(declarer, "hand_updated", nil)
	} else {
		// Playing hand - move to game declaration
		r.broadcastStateChange("playing_hand", playerID, nil)
	}

	// Continue game loop if moved to declaration phase
	if r.GameState.Phase == game.PhaseDeclarerChoice {
		go r.gameLoop()
	}

	return nil
}

// HandleDiscard processes the declarer's card discard after picking up skat
func (r *GameSession) HandleDiscard(playerID string, card1, card2 game.Card) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	declarer := r.getCurrentPlayer()
	if declarer.ID != playerID {
		return fmt.Errorf("only declarer can discard cards")
	}

	if r.GameState.Phase != game.PhaseSkatExchange {
		return fmt.Errorf("not in skat exchange phase")
	}

	err := r.GameState.DiscardToSkat(card1, card2)
	if err != nil {
		return err
	}

	log.Printf("Declarer %s discarded cards to skat", declarer.Name)

	// Broadcast that discard is complete
	r.broadcastStateChange("cards_discarded", playerID, nil)

	// Send updated hand to declarer (back to 10 cards)
	r.sendPlayerSpecificUpdate(declarer, "hand_updated", nil)

	// Continue to game declaration phase
	go r.gameLoop()

	return nil
}

// handleGameComplete processes game end
func (r *GameSession) handleGameComplete() {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Get game result including schneider/schwarz
	declarerWon, schneider, schwarz := r.GameState.GetGameResult()

	// Get game mode as string for broadcast
	var gameModeStr string
	switch r.GameState.Mode {
	case game.ModeGrand:
		gameModeStr = "Grand"
	case game.ModeSuit:
		gameModeStr = "Suit"
	case game.ModeNull:
		gameModeStr = "Null"
	default:
		gameModeStr = "Unknown"
	}

	// Broadcast game complete using state diff
	r.broadcastStateChange("game_complete", "", map[string]interface{}{
		"declarer_won":   declarerWon,
		"schneider":      schneider,
		"schwarz":        schwarz,
		"declarer_score": r.GameState.DeclarerScore,
		"opponent_score": r.GameState.OpponentScore,
		"game_mode":      gameModeStr,
		"trump_suit":     r.GameState.TrumpSuit,
	})

	log.Printf("Game complete with ID %s - Declarer won: %v, Schneider: %v, Schwarz: %v",
		r.ID, declarerWon, schneider, schwarz)

	// Save game history to database
	if r.server != nil && r.server.db != nil {
		var gameMode string
		switch r.GameState.Mode {
		case game.ModeGrand:
			gameMode = "Grand"
		case game.ModeSuit:
			gameMode = fmt.Sprintf("Suit (%s)", r.GameState.TrumpSuit)
		case game.ModeNull:
			gameMode = "Null"
		}

		// Check if game has any AI players
		hasAI := false
		for _, player := range r.Players {
			if player.Agent != nil {
				hasAI = true
				break
			}
		}

		// Collect all player names for opponent tracking
		allPlayerNames := make(map[game.GamePosition]string)
		for _, player := range r.Players {
			allPlayerNames[player.Position] = player.Name
		}

		var historyEntries []GameHistoryEntry
		for _, player := range r.Players {
			// Save history for human players only
			if player.Agent != nil {
				continue
			}

			isDeclarer := player.Position == r.GameState.Declarer
			var isWinner bool
			var finalScore int

			if isDeclarer {
				isWinner = declarerWon
				finalScore = r.GameState.DeclarerScore
			} else {
				isWinner = !declarerWon
				finalScore = r.GameState.OpponentScore
			}

			// Get opponent names (everyone except this player)
			var opponentNames []string
			for pos, name := range allPlayerNames {
				if pos != player.Position {
					opponentNames = append(opponentNames, name)
				}
			}

			entry := GameHistoryEntry{
				GameID:        r.ID,
				GameCode:      r.Code,
				PlayerID:      player.ID,
				PlayerName:    player.Name,
				IsWinner:      isWinner,
				IsDeclarer:    isDeclarer,
				FinalScore:    finalScore,
				GameMode:      gameMode,
				OpponentNames: opponentNames,
				VsAI:          hasAI,
			}
			historyEntries = append(historyEntries, entry)
		}

		if len(historyEntries) > 0 {
			if err := r.server.db.SaveGameHistory(r.ID, r.Code, historyEntries); err != nil {
				log.Printf("Failed to save game history: %v", err)
			} else {
				log.Printf("Saved game history for %d players", len(historyEntries))
			}
		}
	}
}

// broadcast sends a message to all players in the game
func (r *GameSession) broadcast(msg *Message) {
	// Collect profile IDs of human players
	var profileIDs []string
	for _, player := range r.Players {
		if player.Agent == nil { // Only send to human players
			profileIDs = append(profileIDs, player.ID)
		}
	}

	// Use ClientManager to broadcast
	if r.server != nil && r.server.clients != nil {
		r.server.clients.BroadcastToClients(profileIDs, msg)
	}
}

// getPlayersInfo returns the current players information
func (r *GameSession) GetPlayersInfo() []*PlayerInfo {
	players := make([]*PlayerInfo, 3)
	for _, player := range r.Players {
		info := PlayerInfo{
			PlayerID: player.ID,
			Name:     player.Name,
			Position: player.Position,
			IsAgent:  player.Agent != nil,
		}
		// Add card count if game is active
		if r.GameState != nil && player.Position < 3 && r.GameState.Players[player.Position] != nil {
			info.CardCount = len(r.GameState.Players[player.Position].Hand)
		}
		if player.Position < 3 {
			players[player.Position] = &info
		}
	}
	return players
}

// getPlayerByPosition returns the player at the given position
func (r *GameSession) getPlayerByPosition(position game.GamePosition) *Player {
	for _, player := range r.Players {
		if player.Position == position {
			return player
		}
	}
	return nil
}

// getPlayerByPosition returns the player at the given ID
func (r *GameSession) getPlayerById(playerId string) *Player {
	for _, player := range r.Players {
		if player.ID == playerId {
			return player
		}
	}
	return nil
}

// GetInfo returns public game information, optionally including player-specific data
func (r *GameSession) GetInfo(playerID string) *GameInfo {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	info := &GameInfo{
		ID:            r.ID,
		Code:          r.Code,
		Players:       r.GetPlayersInfo(),
		CurrentPlayer: -1,
		Phase:         "lobby",
		Trick:         []CardInfo{},
		DeclarerScore: 0,
		Declarer:      -1,
		GameOver:      false,
	}

	// Include player ID if provided
	if playerID != "" {
		info.PlayerID = playerID
	}

	// If game has started, include game state
	if r.GameState != nil {
		info.CurrentPlayer = r.GameState.CurrentPlayer

		// Convert phase to string
		switch r.GameState.Phase {
		case game.PhaseDealing:
			info.Phase = "dealing"
		case game.PhaseBidding:
			info.Phase = "bidding"
		case game.PhaseSkatExchange:
			info.Phase = "skat_exchange"
		case game.PhaseDeclarerChoice:
			info.Phase = "declarer_choice"
		case game.PhasePlaying:
			info.Phase = "playing"
		case game.PhaseComplete:
			info.Phase = "complete"
		}

		// During bidding phase, use BidValue as the score to display
		if r.GameState.Phase == game.PhaseBidding {
			info.DeclarerScore = r.GameState.BidValue
		} else {
			info.DeclarerScore = r.GameState.DeclarerScore
			info.OpponentScore = r.GameState.OpponentScore
		}
		info.Declarer = r.GameState.Declarer
		info.GameOver = r.GameState.Phase == game.PhaseComplete

		// Include declarer tricks for null games
		if r.GameState.Mode == game.ModeNull && r.GameState.Phase == game.PhaseComplete {
			info.DeclarerTricks = r.GameState.DeclarerTricks
		}

		// Include game mode if it's been declared
		if r.GameState.Phase != game.PhaseBidding {
			switch r.GameState.Mode {
			case game.ModeGrand:
				info.GameMode = "Grand"
			case game.ModeSuit:
				info.GameMode = "Suit"
			case game.ModeNull:
				info.GameMode = "Null"
			}

			// Include trump suit for suit games
			if r.GameState.Mode == game.ModeSuit {
				info.TrumpSuit = r.GameState.TrumpSuit.String()
			}
		}

		// Convert trick cards
		trick := make([]CardInfo, len(r.GameState.Trick))
		for i, card := range r.GameState.Trick {
			trick[i] = CardInfo{
				Suit: card.Suit.String(),
				Rank: card.Rank.String(),
			}
		}
		info.Trick = trick

		// Include trick starter and winner info
		info.TrickStarter = r.GameState.TrickStarter
		info.TrickWinner = r.GameState.TrickWinner

		// Include valid bids if in bidding phase
		if r.GameState.Phase == game.PhaseBidding {
			info.ValidBids = r.GameState.GetValidBids()
		}

		requestingPlayer := r.getPlayerById(playerID)

		// Include player's hand if they're in the game
		if requestingPlayer != nil {
			hand := make([]CardInfo, len(r.GameState.Players[requestingPlayer.Position].Hand))
			for i, card := range r.GameState.Players[requestingPlayer.Position].Hand {
				hand[i] = CardInfo{
					Suit: card.Suit.String(),
					Rank: card.Rank.String(),
				}
			}
			info.Hand = hand
		}

		// Calculate score pile counts
		if r.GameState.Declarer >= 0 && r.GameState.Phase == game.PhasePlaying {
			declarerPileCount := 0
			opponentPileCount := 0

			// Count cards in each player's tricks taken
			for i, player := range r.GameState.Players {
				for _, trick := range player.TricksTaken {
					if i == int(r.GameState.Declarer) {
						declarerPileCount += len(trick)
					} else {
						opponentPileCount += len(trick)
					}
				}
			}

			// Add the skat cards to declarer's pile if game has started playing
			if r.GameState.Phase == game.PhasePlaying {
				declarerPileCount += 2 // The two skat cards
			}

			info.DeclarerPileCount = declarerPileCount
			info.OpponentPileCount = opponentPileCount
		}
	}

	return info
}

// getGameStateData returns game state as JSON-friendly data
func (r *GameSession) getGameStateData() map[string]interface{} {
	return map[string]interface{}{
		"current_player": r.GameState.CurrentPlayer,
		"phase":          phaseToString(r.GameState.Phase),
		"trick":          cardsToJSON(r.GameState.Trick),
	}
}

// Helper functions for JSON serialization
func phaseToString(phase game.GamePhase) string {
	switch phase {
	case game.PhaseDealing:
		return "dealing"
	case game.PhaseBidding:
		return "bidding"
	case game.PhaseSkatExchange:
		return "skat_exchange"
	case game.PhaseDeclarerChoice:
		return "declarer_choice"
	case game.PhasePlaying:
		return "playing"
	case game.PhaseComplete:
		return "complete"
	default:
		return "unknown"
	}
}

func cardToJSON(card game.Card) map[string]interface{} {
	return map[string]interface{}{
		"suit": card.Suit.String(),
		"rank": card.Rank.String(),
	}
}

func cardsToJSON(cards []game.Card) []map[string]interface{} {
	result := make([]map[string]interface{}, len(cards))
	for i, card := range cards {
		result[i] = cardToJSON(card)
	}
	return result
}
