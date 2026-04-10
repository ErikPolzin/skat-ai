package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
)

// Database wraps the sql.DB connection
type Database struct {
	DB *sql.DB
}

// NewDatabase creates a new database connection
func NewDatabase() (*Database, error) {
	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "localhost"
	}

	dbPort := os.Getenv("DB_PORT")
	if dbPort == "" {
		dbPort = "5432"
	}

	dbUser := os.Getenv("DB_USER")
	if dbUser == "" {
		dbUser = "postgres"
	}

	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "skat"
	}

	dbSSLMode := os.Getenv("DB_SSLMODE")
	if dbSSLMode == "" {
		dbSSLMode = "disable"
	}

	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		dbHost, dbPort, dbUser, dbPassword, dbName, dbSSLMode,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	log.Printf("Connected to database: %s@%s:%s/%s", dbUser, dbHost, dbPort, dbName)

	return &Database{DB: db}, nil
}

// Close closes the database connection
func (d *Database) Close() error {
	return d.DB.Close()
}

// InitSchema initializes the database schema
func (d *Database) InitSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS player_profiles (
		id VARCHAR(255) PRIMARY KEY,
		name VARCHAR(255) NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		last_seen TIMESTAMP NOT NULL DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS games (
		id VARCHAR(255) PRIMARY KEY,
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP NOT NULL DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS players (
		id VARCHAR(255) PRIMARY KEY,
		game_id VARCHAR(255) NOT NULL,
		name VARCHAR(255) NOT NULL,
		position INT NOT NULL,
		is_agent BOOLEAN NOT NULL DEFAULT FALSE,
		agent_type VARCHAR(50),
		online BOOLEAN NOT NULL DEFAULT TRUE,
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		FOREIGN KEY (game_id) REFERENCES games(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS game_states (
		id SERIAL PRIMARY KEY,
		game_id VARCHAR(255) NOT NULL,
		game_data JSONB NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		FOREIGN KEY (game_id) REFERENCES games(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_players_game_id ON players(game_id);
	CREATE INDEX IF NOT EXISTS idx_game_states_game_id ON game_states(game_id);

	-- Game history tracking
	CREATE TABLE IF NOT EXISTS game_history (
		id SERIAL PRIMARY KEY,
		game_id VARCHAR(255) NOT NULL,
		game_code VARCHAR(10),
		player_id VARCHAR(255) NOT NULL,
		player_name VARCHAR(255) NOT NULL,
		is_winner BOOLEAN,
		is_declarer BOOLEAN,
		final_score INT,
		game_mode VARCHAR(50),
		opponent_names TEXT,  -- JSON array of opponent names
		vs_ai BOOLEAN DEFAULT FALSE,  -- True if game included AI players
		finished_at TIMESTAMP NOT NULL DEFAULT NOW(),
		FOREIGN KEY (player_id) REFERENCES player_profiles(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_game_history_player_id ON game_history(player_id);
	CREATE INDEX IF NOT EXISTS idx_game_history_finished_at ON game_history(finished_at DESC);
	`

	_, err := d.DB.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to initialize schema: %w", err)
	}

	log.Println("Database schema initialized")
	return nil
}

// SaveGame persists a game to the database
func (d *Database) SaveGame(game *GameSession) error {
	game.mutex.RLock()
	defer game.mutex.RUnlock()

	_, err := d.DB.Exec(
		`INSERT INTO games (id, updated_at)
		 VALUES ($1, $2, NOW())
		 ON CONFLICT (id) DO UPDATE SET
		 updated_at = NOW()`,
		game.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to save game: %w", err)
	}

	return nil
}

// SavePlayer persists a player to the database
func (d *Database) SavePlayer(gameID string, player *Player) error {
	var agentType *string
	if player.Agent != nil {
		t := "unknown"
		agentType = &t
	}

	// For now, just set online to false - this is just for database persistence
	// The actual online status is tracked by ClientManager
	online := false

	_, err := d.DB.Exec(
		`INSERT INTO players (id, game_id, name, position, is_agent, agent_type, online)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (id) DO UPDATE SET
		 game_id = $2, name = $3, position = $4, is_agent = $5, agent_type = $6, online = $7`,
		player.ID, gameID, player.Name, player.Position, player.Agent != nil, agentType, online,
	)
	if err != nil {
		return fmt.Errorf("failed to save player: %w", err)
	}

	return nil
}

// LoadGame loads a game from the database
func (d *Database) LoadGame(gameID string) (*GameSession, error) {
	var game GameSession
	err := d.DB.QueryRow(
		`SELECT id FROM games WHERE id = $1`,
		gameID,
	).Scan(&game.ID)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("game not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load game: %w", err)
	}

	game.Players = make(map[string]*Player)

	return &game, nil
}

// DeleteGame removes a game from the database
func (d *Database) DeleteGame(gameID string) error {
	_, err := d.DB.Exec(`DELETE FROM games WHERE id = $1`, gameID)
	if err != nil {
		return fmt.Errorf("failed to delete game: %w", err)
	}

	return nil
}

// ListGamesFromDB returns all games from the database
func (d *Database) ListGamesFromDB() ([]*GameInfo, error) {
	rows, err := d.DB.Query(`
		SELECT g.id
		FROM games g
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list games: %w", err)
	}
	defer rows.Close()

	var games []*GameInfo
	for rows.Next() {
		var game GameInfo
		if err := rows.Scan(&game.ID); err != nil {
			return nil, fmt.Errorf("failed to scan game: %w", err)
		}
		// Initialize empty slices
		game.Players = []*PlayerInfo{}
		game.Trick = []CardInfo{}
		game.CurrentPlayer = -1
		game.Phase = "lobby"
		games = append(games, &game)
	}

	return games, nil
}

// SavePlayerProfile saves or updates a player profile
func (d *Database) SavePlayerProfile(playerID, playerName string) error {
	_, err := d.DB.Exec(`
		INSERT INTO player_profiles (id, name, last_seen)
		VALUES ($1, $2, NOW())
		ON CONFLICT (id) DO UPDATE
		SET name = $2, last_seen = NOW()
	`, playerID, playerName)
	return err
}

// GetPlayerProfile retrieves a player's name by ID
func (d *Database) GetPlayerProfile(playerID string) (string, error) {
	var name string
	err := d.DB.QueryRow(`
		SELECT name FROM player_profiles WHERE id = $1
	`, playerID).Scan(&name)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("player profile not found")
	}
	return name, err
}

// SetPlayerProfile stores or updates a player's profile
func (d *Database) SetPlayerProfile(playerID, name string) error {
	_, err := d.DB.Exec(`
		INSERT INTO player_profiles (id, name)
		VALUES ($1, $2)
		ON CONFLICT (id) DO UPDATE SET
		name = EXCLUDED.name
	`, playerID, name)
	return err
}

// GameHistoryEntry represents a completed game for a player
type GameHistoryEntry struct {
	GameID        string   `json:"game_id"`
	GameCode      string   `json:"game_code"`
	PlayerID      string   `json:"player_id"`
	PlayerName    string   `json:"player_name"`
	IsWinner      bool     `json:"is_winner"`
	IsDeclarer    bool     `json:"is_declarer"`
	FinalScore    int      `json:"final_score"`
	GameMode      string   `json:"game_mode"`
	OpponentNames []string `json:"opponent_names"`
	VsAI          bool     `json:"vs_ai"`
	FinishedAt    string   `json:"finished_at"`
}

// SaveGameHistory saves the result of a completed game
func (d *Database) SaveGameHistory(gameID, gameCode string, players []GameHistoryEntry) error {
	tx, err := d.DB.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	for _, player := range players {
		// Convert opponent names to JSON string
		opponentNamesJSON := "[]"
		if len(player.OpponentNames) > 0 {
			jsonBytes, _ := json.Marshal(player.OpponentNames)
			opponentNamesJSON = string(jsonBytes)
		}

		_, err := tx.Exec(`
			INSERT INTO game_history (game_id, game_code, player_id, player_name, is_winner, is_declarer, final_score, game_mode, opponent_names, vs_ai)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		`, gameID, gameCode, player.PlayerID, player.PlayerName, player.IsWinner, player.IsDeclarer, player.FinalScore, player.GameMode, opponentNamesJSON, player.VsAI)
		if err != nil {
			return fmt.Errorf("failed to save game history: %w", err)
		}
	}

	return tx.Commit()
}

// GetPlayerGameHistory retrieves the last N games for a player
func (d *Database) GetPlayerGameHistory(playerID string, limit int) ([]GameHistoryEntry, error) {
	rows, err := d.DB.Query(`
		SELECT game_id, game_code, player_id, player_name, is_winner, is_declarer, final_score, game_mode,
		       opponent_names, vs_ai, to_char(finished_at, 'YYYY-MM-DD HH24:MI:SS') as finished_at
		FROM game_history
		WHERE player_id = $1
		ORDER BY finished_at DESC
		LIMIT $2
	`, playerID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get game history: %w", err)
	}
	defer rows.Close()

	var history []GameHistoryEntry
	for rows.Next() {
		var entry GameHistoryEntry
		var isWinner, isDeclarer, vsAI sql.NullBool
		var finalScore sql.NullInt32
		var gameMode, opponentNamesJSON sql.NullString

		if err := rows.Scan(
			&entry.GameID, &entry.GameCode, &entry.PlayerID, &entry.PlayerName,
			&isWinner, &isDeclarer, &finalScore, &gameMode,
			&opponentNamesJSON, &vsAI, &entry.FinishedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan game history: %w", err)
		}

		entry.IsWinner = isWinner.Bool
		entry.IsDeclarer = isDeclarer.Bool
		entry.FinalScore = int(finalScore.Int32)
		entry.GameMode = gameMode.String
		entry.VsAI = vsAI.Bool

		// Parse opponent names from JSON
		if opponentNamesJSON.Valid && opponentNamesJSON.String != "" {
			json.Unmarshal([]byte(opponentNamesJSON.String), &entry.OpponentNames)
		}

		history = append(history, entry)
	}

	return history, nil
}
