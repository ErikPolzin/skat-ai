package db

import (
	"fmt"
	"skat/game"
	"sync"
)

// MemoryDatabase is an in-memory implementation of the Database interface
// Useful for testing and running without database persistence
type MemoryDatabase struct {
	profiles      map[string]*ProfileEntry
	games         map[string]*game.GameState
	playerResults map[string][]game.PlayerResultState
	ratings       map[string]*PlayerRating
	mu            sync.RWMutex
}

// NewMemoryDatabase creates a new in-memory database
func NewMemoryDatabase() *MemoryDatabase {
	return &MemoryDatabase{
		profiles:      make(map[string]*ProfileEntry),
		games:         make(map[string]*game.GameState),
		playerResults: make(map[string][]game.PlayerResultState),
		ratings:       make(map[string]*PlayerRating),
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
	// No-op for memory database - sessions are implicit
	return nil
}

func (d *MemoryDatabase) GetGameSession(sessionID string) (*game.GameSessionState, error) {
	// For memory database, sessions aren't explicitly tracked, so generate a basic one
	d.mu.RLock()
	defer d.mu.RUnlock()

	_, ok := d.games[sessionID]
	if !ok {
		return nil, fmt.Errorf("game session not found")
	}

	// Memory DB uses sessionID as the code since we don't persist sessions
	return &game.GameSessionState{
		ID:   sessionID,
		Code: sessionID[:8], // Use first 8 chars of session ID as code
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
	return nil
}

func (d *MemoryDatabase) DeleteGame(gameID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	delete(d.games, gameID)
	return nil
}

func (d *MemoryDatabase) ListOpenSessions() ([]game.GameSessionState, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var sessions []game.GameSessionState
	for sessionID, gameState := range d.games {
		if gameState.PlayerCount() < 3 && gameState.Phase == game.PhaseWaitingForPlayers {
			sessions = append(sessions, game.GameSessionState{
				ID:          sessionID,
				Code:        string(gameState.Code),
				GameID:      gameState.ID,
				PlayerCount: gameState.PlayerCount(),
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
		d.playerResults[result.PlayerID] = append(d.playerResults[result.PlayerID], result)
	}
	return nil
}

func (d *MemoryDatabase) GetPlayerResults(playerID string, limit int) ([]game.PlayerResultState, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	results, ok := d.playerResults[playerID]
	if !ok {
		return []game.PlayerResultState{}, nil
	}

	// Return most recent results first
	start := 0

	// Reverse order to get most recent first
	reversed := make([]game.PlayerResultState, 0, len(results)-start)
	for i := len(results) - 1; i >= start; i-- {
		reversed = append(reversed, results[i])
	}

	return reversed, nil
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

func (d *MemoryDatabase) GetSessionResults(sessionID string) ([]game.PlayerResultState, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var results []game.PlayerResultState

	// Find all completed games for this session
	for _, gameState := range d.games {
		if gameState.SessionID != sessionID || gameState.Phase != game.PhaseComplete {
			continue
		}

		// Get player results for this game
		points := gameState.CalculatePlayerPoints()

		for pos := game.Dealer; pos <= game.Speaker; pos++ {
			player := gameState.Players[pos]
			if player == nil {
				continue
			}

			isDeclarer := pos == gameState.Declarer
			var isWinner bool
			if isDeclarer {
				declarerWon, _, _ := gameState.GetGameResult()
				isWinner = declarerWon
			} else {
				declarerWon, _, _ := gameState.GetGameResult()
				isWinner = !declarerWon
			}

			results = append(results, game.PlayerResultState{
				GameID:         gameState.ID,
				SessionID:      sessionID,
				PlayerID:       player.ID,
				PlayerPosition: pos,
				PlayerPoints:   points[pos],
				IsWinner:       isWinner,
			})
		}
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
