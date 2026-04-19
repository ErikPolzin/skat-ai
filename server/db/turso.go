package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"skat/game"
	"strings"

	_ "github.com/tursodatabase/libsql-client-go/libsql"
)

// Database wraps the sql.DB connection
type TursoDatabase struct {
	DB *sql.DB
}

// NewDatabase creates a new database connection
func NewTursoDatabase(connStr string) (*TursoDatabase, error) {
	db, err := sql.Open("libsql", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open Turso database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	log.Printf("Connected to Turso database")

	return &TursoDatabase{DB: db}, nil
}

// Close closes the database connection
func (d *TursoDatabase) Close() error {
	return d.DB.Close()
}

// InitSchema initializes the database schema
func (d *TursoDatabase) InitSchema() error {
	schema, err := os.ReadFile("server/db/schema/schema.turso.sql")
	if err != nil {
		return fmt.Errorf("failed to read schema file: %w", err)
	}
	// Split by semicolon and execute each statement
	statements := strings.Split(string(schema), ";")
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt != "" {
			_, err := d.DB.Exec(stmt)
			if err != nil {
				return fmt.Errorf("failed to execute statement: %s: %w", stmt[:50], err)
			}
		}
	}
	log.Println("Database schema initialized")
	return nil
}

func (d *TursoDatabase) GetProfile(profileID string) (*ProfileEntry, error) {
	var profile ProfileEntry
	var isAgent int
	err := d.DB.QueryRow(`
		SELECT id, name, is_agent FROM profiles WHERE id = $1
	`, profileID).Scan(&profile.ID, &profile.Name, &isAgent)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("player profile not found")
	}
	return &profile, err
}

func (d *TursoDatabase) SaveProfile(profile ProfileEntry) error {
	_, err := d.DB.Exec(
		`INSERT INTO profiles (id, name, is_agent)
		 	VALUES ($1, $2, $3)
		 	ON CONFLICT (id) DO UPDATE SET
		 	id = $1, name = $2, is_agent = $3`,
		profile.ID, profile.Name, profile.IsAgent,
	)
	if err != nil {
		return fmt.Errorf("failed to save profile: %w", err)
	}
	return nil
}

func (d *TursoDatabase) SaveGameSession(session game.GameSessionState) error {
	_, err := d.DB.Exec(
		`INSERT INTO game_sessions (id, code, game_id, player_count, created_at, ended_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO NOTHING`,
		session.ID, session.Code, session.GameID, session.PlayerCount, session.CreatedAt, session.EndedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to save game session: %w", err)
	}
	return nil
}

func (d *TursoDatabase) GetGameSession(sessionID string) (*game.GameSessionState, error) {
	var session game.GameSessionState
	err := d.DB.QueryRow(`
		SELECT id, code, created_at, ended_at
		FROM game_sessions
		WHERE id = $1
	`, sessionID).Scan(&session.ID, &session.Code, &session.CreatedAt, &session.EndedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("game session not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get game session: %w", err)
	}
	return &session, nil
}

func (d *TursoDatabase) GetGameByID(gameID string) (*game.GameState, error) {
	var gs game.GameState
	var skatString, trickString string
	err := d.DB.QueryRow(
		`SELECT g.id, g.session_id, gs.code, g.game_number, g.phase, g.skat, g.trick,
			g.trick_starter, g.trick_winner, g.current_player, g.declarer,
			g.declarer_score, g.opponent_score, g.game_mode, g.trump_suit,
			g.bid_value, g.listener_passed, g.speaker_passed, g.dealer_passed
		FROM games g
		JOIN game_sessions gss ON g.session_id = gss.id
		WHERE g.id = $1`,
		gameID,
	).Scan(
		&gs.ID, &gs.SessionID, &gs.Code, &gs.GameNumber, &gs.Phase, &skatString, &trickString,
		&gs.TrickStarter, &gs.TrickWinner, &gs.CurrentPlayer, &gs.Declarer,
		&gs.DeclarerScore, &gs.OpponentScore, &gs.Mode, &gs.TrumpSuit,
		&gs.BidValue, &gs.ListenerPassed, &gs.SpeakerPassed, &gs.DealerPassed)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("game not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load game: %w", err)
	}

	// Parse skat and trick
	gs.Skat, err = game.ParseSkatCards(skatString)
	if err != nil {
		return nil, fmt.Errorf("cannot parse skat cards: %w", err)
	}
	gs.Trick, err = game.ParseCards(trickString)
	if err != nil {
		return nil, fmt.Errorf("cannot parse trick: %w", err)
	}

	// Load players
	gs.Players, err = d.ListPlayers(gs.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to load players: %w", err)
	}

	return &gs, nil
}

func (d *TursoDatabase) GetGameBySessionCode(sessionCode string) (*game.GameState, error) {
	var gs game.GameState
	var skatString, trickString string
	err := d.DB.QueryRow(
		`SELECT g.id, g.session_id, gs.code, g.game_number, g.phase, g.skat, g.trick,
			g.trick_starter, g.trick_winner, g.current_player, g.declarer,
			g.declarer_score, g.opponent_score, g.game_mode, g.trump_suit,
			g.bid_value, g.listener_passed, g.speaker_passed, g.dealer_passed
		FROM games g
		JOIN game_sessions gs ON g.session_id = gs.id
		WHERE gs.code = $1
		ORDER BY g.created_at DESC
		LIMIT 1`,
		sessionCode,
	).Scan(
		&gs.ID, &gs.SessionID, &gs.Code, &gs.GameNumber, &gs.Phase, &skatString, &trickString,
		&gs.TrickStarter, &gs.TrickWinner, &gs.CurrentPlayer, &gs.Declarer,
		&gs.DeclarerScore, &gs.OpponentScore, &gs.Mode, &gs.TrumpSuit,
		&gs.BidValue, &gs.ListenerPassed, &gs.SpeakerPassed, &gs.DealerPassed)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("game not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load game: %w", err)
	}

	// Parse skat and trick
	gs.Skat, err = game.ParseSkatCards(skatString)
	if err != nil {
		return nil, fmt.Errorf("cannot parse skat cards: %w", err)
	}
	gs.Trick, err = game.ParseCards(trickString)
	if err != nil {
		return nil, fmt.Errorf("cannot parse trick: %w", err)
	}

	// Load players
	gs.Players, err = d.ListPlayers(gs.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to load players: %w", err)
	}

	return &gs, nil
}

func (d *TursoDatabase) SaveGame(gs game.GameState) error {
	skatCards := game.SkatCards(gs.Skat)
	skatString := skatCards.String()
	trickString := gs.Trick.String()

	// SQLite/Turso syntax
	_, err := d.DB.Exec(
		`INSERT INTO games (
			id, session_id, game_number, phase, skat, trick,
			trick_starter, trick_winner, current_player,
			declarer, declarer_score, opponent_score,
			game_mode, trump_suit, bid_value,
			listener_passed, speaker_passed, dealer_passed,
			created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18,
			CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT (id) DO UPDATE SET
			session_id = $2, game_number = $3, phase = $4, skat = $5, trick = $6,
			trick_starter = $7, trick_winner = $8, current_player = $9,
			declarer = $10, declarer_score = $11, opponent_score = $12,
			game_mode = $13, trump_suit = $14, bid_value = $15,
			listener_passed = $16, speaker_passed = $17, dealer_passed = $18,
			updated_at = CURRENT_TIMESTAMP`,
		gs.ID, gs.SessionID, gs.GameNumber, gs.Phase, skatString, trickString,
		gs.TrickStarter, gs.TrickWinner, gs.CurrentPlayer,
		gs.Declarer, gs.DeclarerScore, gs.OpponentScore,
		gs.Mode, gs.TrumpSuit, gs.BidValue,
		gs.ListenerPassed, gs.SpeakerPassed, gs.DealerPassed,
	)
	if err != nil {
		return fmt.Errorf("failed to save game: %w", err)
	}

	// Save players
	for pos, player := range gs.Players {
		if player != nil {
			handString := player.Hand.String()
			_, err := d.DB.Exec(
				`INSERT INTO players (game_id, profile_id, hand, position)
				VALUES ($1, $2, $3, $4)
				ON CONFLICT (game_id, profile_id) DO UPDATE SET
					hand = $3, position = $4`,
				gs.ID, player.ID, handString, pos,
			)
			if err != nil {
				return fmt.Errorf("failed to save player: %w", err)
			}
		}
	}

	return nil
}

func (d *TursoDatabase) ListOpenSessions() ([]game.GameSessionState, error) {
	// Query for games where not all 3 player positions are filled
	rows, err := d.DB.Query(`
		SELECT gs.id, gs.code, gs.game_id, gs.player_count, gs.created_at, gs.ended_at
		FROM game_sessions gs
		JOIN games g ON g.id = gs.game_id
		WHERE gs.player_count < 3 AND g.phase = "waiting_for_players"
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list open games: %w", err)
	}
	defer rows.Close()

	var sessions []game.GameSessionState
	for rows.Next() {
		var se game.GameSessionState
		if err := rows.Scan(
			&se.ID, &se.Code, &se.GameID, &se.PlayerCount, &se.CreatedAt, &se.EndedAt); err != nil {
			return nil, fmt.Errorf("failed to scan game: %w", err)
		}

		sessions = append(sessions, se)
	}
	return sessions, nil
}

func (d *TursoDatabase) DeleteGame(gameID string) error {
	_, err := d.DB.Exec(`DELETE FROM games WHERE id = $1`, gameID)
	if err != nil {
		return fmt.Errorf("failed to delete game: %w", err)
	}

	return nil
}

func (d *TursoDatabase) ListPlayers(gameID string) ([3]*game.PlayerState, error) {
	rows, err := d.DB.Query(`
		SELECT pl.hand, pl.position, pr.id, pr.name, pr.is_agent
		FROM players pl
		JOIN profiles pr ON pr.id = pl.profile_id
		WHERE pl.game_id = $1
	`, gameID)
	if err != nil {
		return [3]*game.PlayerState{}, fmt.Errorf("failed to list players: %w", err)
	}
	defer rows.Close()

	var players [3]*game.PlayerState
	for rows.Next() {
		var handString string
		var position int
		var ps game.PlayerState
		var isAgent int
		if err := rows.Scan(
			&handString, &position, &ps.ID, &ps.Name, &isAgent); err != nil {
			return [3]*game.PlayerState{}, fmt.Errorf("failed to scan player: %w", err)
		}
		ps.Hand, err = game.ParseCards(handString)
		if err != nil {
			return [3]*game.PlayerState{}, fmt.Errorf("cannot parse hand: %s", handString)
		}
		ps.IsAgent = isAgent != 0
		if position >= 0 && position < 3 {
			players[position] = &ps
		}
	}
	return players, nil
}

func (d *TursoDatabase) SavePlayerResults(results []game.PlayerResultState) error {
	for _, result := range results {
		isWinner := 0
		if result.IsWinner {
			isWinner = 1
		}

		_, err := d.DB.Exec(
			`INSERT INTO player_results (
				game_id, session_id, player_id, player_position, player_points, is_winner
			) VALUES ($1, $2, $3, $4, $5, $6)`,
			result.GameID, result.SessionID, result.PlayerID, result.PlayerPosition, result.PlayerPoints, isWinner,
		)
		if err != nil {
			return fmt.Errorf("failed to save player result: %w", err)
		}
	}
	return nil
}

func (d *TursoDatabase) CountGamesInSession(sessionID string) (int, error) {
	var count int
	err := d.DB.QueryRow(`
		SELECT COUNT(*) FROM games
		WHERE session_id = $1 AND phase = 'complete'
	`, sessionID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count games in session: %w", err)
	}
	return count, nil
}

func (d *TursoDatabase) GetSessionResults(sessionID string) ([]game.PlayerResultState, error) {
	rows, err := d.DB.Query(`
		SELECT game_id, session_id, player_id, player_position, player_points, is_winner
		FROM player_results
		WHERE session_id = $1
		ORDER BY game_id ASC, player_position ASC
	`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session results: %w", err)
	}
	defer rows.Close()

	var results []game.PlayerResultState
	for rows.Next() {
		var result game.PlayerResultState
		var isWinner int
		if err := rows.Scan(&result.GameID, &result.SessionID, &result.PlayerID,
			&result.PlayerPosition, &result.PlayerPoints, &isWinner); err != nil {
			return nil, fmt.Errorf("failed to scan player result: %w", err)
		}
		result.IsWinner = isWinner != 0
		results = append(results, result)
	}

	return results, nil
}

func (d *TursoDatabase) GetPlayerResults(playerID string, limit int) ([]game.PlayerResultState, error) {
	query := `
		SELECT game_id, session_id, player_id, player_position, player_points, is_winner
		FROM player_results
		WHERE player_id = $1
		ORDER BY id DESC
	`
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := d.DB.Query(query, playerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get player results: %w", err)
	}
	defer rows.Close()

	var results []game.PlayerResultState
	for rows.Next() {
		var result game.PlayerResultState
		var isWinner int
		if err := rows.Scan(
			&result.GameID, &result.SessionID, &result.PlayerID,
			&result.PlayerPosition, &result.PlayerPoints, &isWinner,
		); err != nil {
			return nil, fmt.Errorf("failed to scan player result: %w", err)
		}
		result.IsWinner = isWinner != 0
		results = append(results, result)
	}
	return results, nil
}

func (d *TursoDatabase) ListAgentProfiles() ([]ProfileEntry, error) {
	rows, err := d.DB.Query(`
		SELECT id, name, is_agent FROM profiles WHERE is_agent = 1
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list agent profiles: %w", err)
	}
	defer rows.Close()

	var profiles []ProfileEntry
	for rows.Next() {
		var profile ProfileEntry
		if err := rows.Scan(&profile.ID, &profile.Name, &profile.IsAgent); err != nil {
			return nil, fmt.Errorf("failed to scan agent profile: %w", err)
		}
		profiles = append(profiles, profile)
	}
	return profiles, nil
}
