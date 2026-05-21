package db

import (
	"fmt"
	"skat/game"
	"sync"
)

// MemoryDatabase is an in-memory implementation of the Database interface
// Useful for testing and running without database persistence
type MemoryDatabase struct {
	profiles       map[string]*ProfileEntry
	games          map[string]*game.GameState
	sessions       map[string]*game.GameSessionState
	playerResults  map[string][]game.PlayerResultState
	sessionResults map[string][]game.PlayerSessionResultState
	ratings        map[string]*PlayerRating
	agentConfigs   map[string]*AgentConfig
	mu             sync.RWMutex
}

// NewMemoryDatabase creates a new in-memory database
func NewMemoryDatabase() *MemoryDatabase {
	return &MemoryDatabase{
		profiles:       make(map[string]*ProfileEntry),
		games:          make(map[string]*game.GameState),
		sessions:       make(map[string]*game.GameSessionState),
		playerResults:  make(map[string][]game.PlayerResultState),
		sessionResults: make(map[string][]game.PlayerSessionResultState),
		ratings:        make(map[string]*PlayerRating),
		agentConfigs:   make(map[string]*AgentConfig),
	}
}

func (d *MemoryDatabase) Close() error {
	// Nothing to close for in-memory database
	return nil
}

func (d *MemoryDatabase) InitSchema() error {
	// No schema needed for in-memory database
	return nil
}

func (d *MemoryDatabase) GetProfile(profileID string) (*ProfileEntry, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	profile, ok := d.profiles[profileID]
	if !ok {
		return nil, fmt.Errorf("profile not found")
	}
	return profile, nil
}

func (d *MemoryDatabase) GetProfileByName(name string) (*ProfileEntry, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	for _, profile := range d.profiles {
		if profile.Name == name {
			return profile, nil
		}
	}
	return nil, fmt.Errorf("profile not found")
}

func (d *MemoryDatabase) SaveProfile(profile ProfileEntry) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.profiles[profile.ID] = &profile
	return nil
}

func (d *MemoryDatabase) SaveGameSession(session game.GameSessionState) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if session.MaxGames <= 0 {
		session.MaxGames = game.DefaultMaxGames
	}
	if session.PassPolicy == "" {
		session.PassPolicy = string(game.DefaultPassPolicy)
	}
	d.sessions[session.ID] = &session
	return nil
}

func (d *MemoryDatabase) GetGameSession(sessionID string) (*game.GameSessionState, error) {
	// For memory database, sessions aren't explicitly tracked, so generate a basic one
	d.mu.RLock()
	defer d.mu.RUnlock()

	if session, ok := d.sessions[sessionID]; ok {
		return session, nil
	}
	_, ok := d.games[sessionID]
	if !ok {
		for _, gameState := range d.games {
			if gameState.SessionID == sessionID {
				return &game.GameSessionState{
					ID:           sessionID,
					Code:         string(gameState.Code),
					GameID:       gameState.ID,
					PlayerCount:  gameState.PlayerCount(),
					MaxGames:     gameState.MaxGames,
					PassPolicy:   string(gameState.PassPolicy),
					TimerEnabled: gameState.TimerEnabled,
				}, nil
			}
		}
		return nil, fmt.Errorf("game session not found")
	}
	return &game.GameSessionState{
		ID:           sessionID,
		Code:         sessionID[:8],
		MaxGames:     game.DefaultMaxGames,
		PassPolicy:   string(game.DefaultPassPolicy),
		TimerEnabled: game.DefaultTimerEnabled,
	}, nil
}

func (d *MemoryDatabase) GetGameByID(sessionID string) (*game.GameState, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	gameState, ok := d.games[sessionID]
	if !ok {
		return nil, fmt.Errorf("game not found")
	}
	return gameState, nil
}

func (d *MemoryDatabase) GetGameBySessionCode(sessionCode string) (*game.GameState, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// Search for game by its Code field
	for _, gameState := range d.games {
		if string(gameState.Code) == sessionCode {
			return gameState, nil
		}
	}
	return nil, fmt.Errorf("game not found")
}

func (d *MemoryDatabase) SaveGame(gameState game.GameState) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Store by session ID since that's what we look up by
	d.games[gameState.SessionID] = &gameState
	if session, ok := d.sessions[gameState.SessionID]; ok && session.EndedAt == nil {
		session.GameID = gameState.ID
	}
	return nil
}

func (d *MemoryDatabase) DeleteGame(gameID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	delete(d.games, gameID)
	return nil
}

func (d *MemoryDatabase) RemovePlayer(gameID, playerID string) error {
	// No-op for memory database since players are stored in the game state
	// which is updated directly
	return nil
}

func (d *MemoryDatabase) ListOpenSessions() ([]game.GameSessionState, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var sessions []game.GameSessionState
	for sessionID, gameState := range d.games {
		if gameState.PlayerCount() < 3 && gameState.Phase == game.PhaseWaitingForPlayers {
			sessions = append(sessions, game.GameSessionState{
				ID:           sessionID,
				Code:         string(gameState.Code),
				GameID:       gameState.ID,
				PlayerCount:  gameState.PlayerCount(),
				MaxGames:     gameState.MaxGames,
				PassPolicy:   string(gameState.PassPolicy),
				TimerEnabled: gameState.TimerEnabled,
			})
		}
	}
	return sessions, nil
}

func (d *MemoryDatabase) ListPlayers(gameID string) ([3]*game.PlayerState, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	gameState, ok := d.games[gameID]
	if !ok {
		return [3]*game.PlayerState{}, fmt.Errorf("game not found")
	}

	return gameState.Players, nil
}

func (d *MemoryDatabase) SavePlayerResults(results []game.PlayerResultState) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, result := range results {
		existing := d.playerResults[result.PlayerID]
		replaced := false
		for i := range existing {
			if existing[i].GameID == result.GameID {
				existing[i] = result
				replaced = true
				break
			}
		}
		if !replaced {
			existing = append(existing, result)
		}
		d.playerResults[result.PlayerID] = existing
	}
	return nil
}

func (d *MemoryDatabase) SavePlayerSessionResults(results []game.PlayerSessionResultState) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, result := range results {
		existing := d.sessionResults[result.PlayerID]
		replaced := false
		for i := range existing {
			if existing[i].SessionID == result.SessionID {
				existing[i] = result
				replaced = true
				break
			}
		}
		if !replaced {
			existing = append(existing, result)
		}
		d.sessionResults[result.PlayerID] = existing
	}
	return nil
}

func (d *MemoryDatabase) GetPlayerSessionResults(playerID string, limit int) ([]game.PlayerSessionResultState, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	results, ok := d.sessionResults[playerID]
	if !ok {
		return []game.PlayerSessionResultState{}, nil
	}

	reversed := make([]game.PlayerSessionResultState, 0, len(results))
	for i := len(results) - 1; i >= 0; i-- {
		reversed = append(reversed, results[i])
		if limit > 0 && len(reversed) >= limit {
			break
		}
	}
	return reversed, nil
}

func (d *MemoryDatabase) GetSessionPlayerResults(sessionID string) ([]game.PlayerSessionResultState, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var results []game.PlayerSessionResultState
	for _, playerResults := range d.sessionResults {
		for _, result := range playerResults {
			if result.SessionID == sessionID {
				results = append(results, result)
				break
			}
		}
	}
	return results, nil
}

func (d *MemoryDatabase) CountGamesInSession(sessionID string) (int, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	count := 0
	for _, gameState := range d.games {
		if gameState.SessionID == sessionID && gameState.Phase == game.PhaseComplete {
			count++
		}
	}
	return count, nil
}

func (d *MemoryDatabase) GetPlayerResultsForSession(sessionID string) ([]game.PlayerResultState, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var results []game.PlayerResultState

	// Find all completed games for this session
	for _, gameState := range d.games {
		if gameState.SessionID == sessionID && gameState.Phase == game.PhaseComplete {
			playerResults := gameState.PlayerResults()
			if playerResults != nil {
				for _, r := range playerResults {
					results = append(results, r)
				}
			}
		}
	}

	return results, nil
}

func (d *MemoryDatabase) GetFormattedSessionResults(sessionID string) ([]game.SessionGameResult, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var results []game.SessionGameResult

	// Find all completed games for this session
	for _, gs := range d.games {
		if gs.SessionID != sessionID || gs.Phase != game.PhaseComplete {
			continue
		}

		var declarer *game.PlayerState
		if gs.Declarer != nil {
			declarer = gs.Players[*gs.Declarer]
		}
		declarerWon, _, _ := gs.GetGameResult()

		declarerName := ""
		if declarer != nil {
			declarerName = declarer.Name
		}

		result := game.SessionGameResult{
			GameID:          gs.ID,
			GameNumber:      gs.GameNumber,
			DeclarerName:    declarerName,
			DeclarerWon:     declarerWon,
			GameMode:        string(gs.Mode),
			TrumpSuit:       gs.TrumpSuit.String(),
			PlayerResults:   make(map[string]int),
			PlayerNames:     make(map[string]string),
			PlayerWinners:   make(map[string]bool),
			ForfeitedPlayer: gs.ForfeitedPlayer,
		}

		// Add player names and results
		playerResults := gs.PlayerResults()
		if playerResults != nil {
			for _, pr := range playerResults {
				result.PlayerResults[pr.PlayerID] = pr.PlayerPoints
				result.PlayerWinners[pr.PlayerID] = pr.IsWinner
			}
		}

		for _, player := range gs.Players {
			if player != nil {
				result.PlayerNames[player.ID] = player.Name
			}
		}

		results = append(results, result)
	}

	return results, nil
}

func (d *MemoryDatabase) ListAgentProfiles() ([]ProfileEntry, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var profiles []ProfileEntry
	for _, profile := range d.profiles {
		if profile.IsAgent {
			profiles = append(profiles, *profile)
		}
	}
	return profiles, nil
}

func (d *MemoryDatabase) CleanupStaleGames(inactiveMinutes int, onlinePlayerIDs []string) (int, error) {
	// Memory database doesn't track timestamps, so this is a no-op
	// In a real scenario, you'd need to track game timestamps
	return 0, nil
}

func (d *MemoryDatabase) GetActiveGamesByPlayer(playerID string) ([]game.GameState, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var games []game.GameState
	for _, gameState := range d.games {
		// Check if player is in this game and game is not complete
		if gameState.Phase != game.PhaseComplete {
			for _, player := range gameState.Players {
				if player != nil && player.ID == playerID {
					games = append(games, *gameState)
					break
				}
			}
		}
	}
	return games, nil
}

func (d *MemoryDatabase) GetAllExpiredGames() ([]game.GameState, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var games []game.GameState
	for _, gameState := range d.games {
		if gameState.Phase != game.PhaseComplete && gameState.Phase != game.PhaseWaitingForPlayers && gameState.IsDeadlinePassed() {
			games = append(games, *gameState)
		}
	}
	return games, nil
}

// Rating methods

func (d *MemoryDatabase) GetPlayerRating(profileID string) (*PlayerRating, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rating, ok := d.ratings[profileID]
	if !ok {
		// Return default rating for new player
		return &PlayerRating{
			ProfileID:   profileID,
			Rating:      1500,
			GamesPlayed: 0,
			Wins:        0,
			Losses:      0,
			PeakRating:  1500,
		}, nil
	}
	return rating, nil
}

func (d *MemoryDatabase) SavePlayerRating(rating PlayerRating) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.ratings[rating.ProfileID] = &rating
	return nil
}

func (d *MemoryDatabase) GetLeaderboard(limit int) ([]PlayerRating, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var ratings []PlayerRating
	for _, rating := range d.ratings {
		ratings = append(ratings, *rating)
	}

	// Sort by rating descending
	for i := 0; i < len(ratings)-1; i++ {
		for j := i + 1; j < len(ratings); j++ {
			if ratings[j].Rating > ratings[i].Rating {
				ratings[i], ratings[j] = ratings[j], ratings[i]
			}
		}
	}

	// Apply limit if specified
	if limit > 0 && len(ratings) > limit {
		ratings = ratings[:limit]
	}

	return ratings, nil
}

func (d *MemoryDatabase) GetAgentConfig(profileID string) (*AgentConfig, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	config, ok := d.agentConfigs[profileID]
	if !ok {
		return nil, fmt.Errorf("agent config not found for profile %s", profileID)
	}
	return config, nil
}

func (d *MemoryDatabase) SaveAgentConfig(config AgentConfig) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.agentConfigs[config.ProfileID] = &config
	return nil
}

func (d *MemoryDatabase) DeleteAgentConfig(profileID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	delete(d.agentConfigs, profileID)
	return nil
}
