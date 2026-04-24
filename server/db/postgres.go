package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"

	"skat/game"
)

// Database wraps the sql.DB connection
type PgDatabase struct {
	DB *sql.DB
}

// NewDatabase creates a new database connection
func NewPgDatabase(connStr string) (*PgDatabase, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open PostgreSQL database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	log.Printf("Connected to PostgreSQL database")

	return &PgDatabase{DB: db}, nil
}

// Close closes the database connection
func (d *PgDatabase) Close() error {
	return d.DB.Close()
}

// InitSchema initializes the database schema
func (d *PgDatabase) InitSchema() error {
	schema, err := os.ReadFile("server/db/schema/schema.postgres.sql")
	if err != nil {
		return fmt.Errorf("failed to read schema file: %w", err)
	}

	// PostgreSQL can handle multiple statements
	_, err = d.DB.Exec(string(schema))
	if err != nil {
		return fmt.Errorf("failed to initialize schema: %w", err)
	}

	log.Println("Database schema initialized")
	return nil
}
func (d *PgDatabase) GetProfile(profileID string) (*ProfileEntry, error) {
	var profile ProfileEntry
	err := d.DB.QueryRow(`
		SELECT id, name, is_agent, profile_icon, is_online FROM profiles WHERE id = $1
	`, profileID).Scan(&profile.ID, &profile.Name, &profile.IsAgent, &profile.ProfileIcon, &profile.IsOnline)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("player profile not found")
	}
	return &profile, err
}

func (d *PgDatabase) GetProfileByName(name string) (*ProfileEntry, error) {
	var profile ProfileEntry
	err := d.DB.QueryRow(`
		SELECT id, name, is_agent, profile_icon, is_online FROM profiles WHERE name = $1
	`, name).Scan(&profile.ID, &profile.Name, &profile.IsAgent, &profile.ProfileIcon, &profile.IsOnline)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("player profile not found")
	}
	return &profile, err
}

func (d *PgDatabase) SaveProfile(profile ProfileEntry) error {
	_, err := d.DB.Exec(
		`INSERT INTO profiles (id, name, is_agent, profile_icon, is_online)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO UPDATE SET
		name = $2, is_agent = $3, profile_icon = $4, is_online = $5`,
		profile.ID, profile.Name, profile.IsAgent, profile.ProfileIcon, profile.IsOnline,
	)
	if err != nil {
		return fmt.Errorf("failed to save profile: %w", err)
	}
	return nil
}

func (d *PgDatabase) SaveGameSession(session game.GameSessionState) error {
	_, err := d.DB.Exec(
		`INSERT INTO game_sessions (id, code, game_id, player_count, ended_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO NOTHING`,
		session.ID, session.Code, session.GameID, session.PlayerCount, session.EndedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to save game session: %w", err)
	}
	return nil
}

func (d *PgDatabase) GetGameSession(sessionID string) (*game.GameSessionState, error) {
	var session game.GameSessionState
	err := d.DB.QueryRow(`
		SELECT id, code, created_at, ended_at
		FROM game_sessions
		WHERE id = $1
	`, sessionID).Scan(&session.ID, &session.Code, &session.CreatedAt, &session.EndedAt)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, fmt.Errorf("game session not found")
		}
		return nil, fmt.Errorf("failed to get game session: %w", err)
	}
	return &session, nil
}

func (d *PgDatabase) GetGameByID(gameID string) (*game.GameState, error) {
	var skatString, trickString string
	var gs game.GameState
	err := d.DB.QueryRow(
		`SELECT g.id, g.session_id, gs.code, g.game_number, g.phase, g.skat, g.trick,
			g.trick_starter, g.trick_winner, g.current_player, g.declarer,
			g.declarer_score, g.opponent_score, g.game_mode, g.trump_suit,
			g.bid_value, g.matadors, g.listener_passed, g.speaker_passed, g.dealer_passed
		FROM games g
		JOIN game_sessions gs ON g.session_id = gs.id
		WHERE g.id = $1`,
		gameID,
	).Scan(
		&gs.ID, &gs.SessionID, &gs.Code, &gs.GameNumber, &gs.Phase, &skatString, &trickString,
		&gs.TrickStarter, &gs.TrickWinner, &gs.CurrentPlayer, &gs.Declarer,
		&gs.DeclarerScore, &gs.OpponentScore, &gs.Mode, &gs.TrumpSuit,
		&gs.BidValue, &gs.Matadors, &gs.ListenerPassed, &gs.SpeakerPassed, &gs.DealerPassed)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("game not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load game: %w", err)
	}

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

func (d *PgDatabase) GetGameBySessionCode(sessionCode string) (*game.GameState, error) {
	var gs game.GameState
	var skatString, trickString string
	err := d.DB.QueryRow(
		`SELECT g.id, g.session_id, gs.code, g.game_number, g.phase, g.skat, g.trick,
			g.trick_starter, g.trick_winner, g.current_player, g.declarer,
			g.declarer_score, g.opponent_score, g.game_mode, g.trump_suit,
			g.bid_value, g.matadors, g.listener_passed, g.speaker_passed, g.dealer_passed
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
		&gs.BidValue, &gs.Matadors, &gs.ListenerPassed, &gs.SpeakerPassed, &gs.DealerPassed)
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

func (d *PgDatabase) SaveGame(gs game.GameState) error {
	skatCards := game.SkatCards(gs.Skat)
	skatString := skatCards.String()
	trickString := gs.Trick.String()

	_, err := d.DB.Exec(
		`INSERT INTO games (
			id, session_id, game_number, phase, skat, trick,
			trick_starter, trick_winner, current_player,
			declarer, declarer_score, opponent_score,
			game_mode, trump_suit, bid_value, matadors,
			listener_passed, speaker_passed, dealer_passed,
			created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19,
			NOW(), NOW())
		ON CONFLICT (id) DO UPDATE SET
			session_id = $2, game_number = $3, phase = $4, skat = $5, trick = $6,
			trick_starter = $7, trick_winner = $8, current_player = $9,
			declarer = $10, declarer_score = $11, opponent_score = $12,
			game_mode = $13, trump_suit = $14, bid_value = $15, matadors = $16,
			listener_passed = $17, speaker_passed = $18, dealer_passed = $19,
			updated_at = NOW()`,
		gs.ID, gs.SessionID, gs.GameNumber, gs.Phase, skatString, trickString,
		gs.TrickStarter, gs.TrickWinner, gs.CurrentPlayer,
		gs.Declarer, gs.DeclarerScore, gs.OpponentScore,
		gs.Mode, gs.TrumpSuit, gs.BidValue, gs.Matadors,
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

func (d *PgDatabase) ListOpenSessions() ([]game.GameSessionState, error) {
	// Query for games in waiting_for_players phase
	// Count actual players dynamically instead of relying on stale player_count column
	rows, err := d.DB.Query(`
		SELECT gs.id, gs.game_id, gs.code, COALESCE(COUNT(p.profile_id), 0) as player_count, gs.created_at, gs.ended_at
		FROM game_sessions gs
		JOIN games g ON g.id = gs.game_id
		LEFT JOIN players p ON p.game_id = g.id
		WHERE g.phase = 'waiting_for_players'
		GROUP BY gs.id, gs.game_id, gs.code, gs.created_at, gs.ended_at
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list open games: %w", err)
	}
	defer rows.Close()

	var sessions []game.GameSessionState
	for rows.Next() {
		var se game.GameSessionState
		if err := rows.Scan(
			&se.ID, &se.GameID, &se.Code, &se.PlayerCount, &se.CreatedAt, &se.EndedAt); err != nil {
			return nil, fmt.Errorf("failed to scan game: %w", err)
		}

		sessions = append(sessions, se)
	}
	return sessions, nil
}

func (d *PgDatabase) DeleteGame(gameID string) error {
	_, err := d.DB.Exec(`DELETE FROM games WHERE id = $1`, gameID)
	if err != nil {
		return fmt.Errorf("failed to delete game: %w", err)
	}

	return nil
}

func (d *PgDatabase) ListPlayers(gameID string) ([3]*game.PlayerState, error) {
	rows, err := d.DB.Query(`
		SELECT pl.hand, pl.position, pr.id, pr.name, pr.is_agent, pr.profile_icon, pr.is_online
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
		if err := rows.Scan(
			&handString, &position, &ps.ID, &ps.Name, &ps.IsAgent, &ps.ProfileIcon, &ps.IsOnline); err != nil {
			return [3]*game.PlayerState{}, fmt.Errorf("failed to scan player: %w", err)
		}
		ps.Hand, err = game.ParseCards(handString)
		if err != nil {
			return [3]*game.PlayerState{}, fmt.Errorf("cannot parse hand: %s", handString)
		}
		if position >= 0 && position < 3 {
			players[position] = &ps
		}
	}
	return players, nil
}

func (d *PgDatabase) SavePlayerResults(results []game.PlayerResultState) error {
	for _, result := range results {
		_, err := d.DB.Exec(
			`INSERT INTO player_results (
				game_id, session_id, player_id, player_position, player_points, is_winner
			) VALUES ($1, $2, $3, $4, $5, $6)`,
			result.GameID, result.SessionID, result.PlayerID, result.PlayerPosition, result.PlayerPoints, result.IsWinner,
		)
		if err != nil {
			return fmt.Errorf("failed to save player result: %w", err)
		}
	}
	return nil
}

func (d *PgDatabase) GetPlayerResults(playerID string, limit int) ([]game.PlayerResultState, error) {
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
		if err := rows.Scan(
			&result.GameID, &result.SessionID, &result.PlayerID,
			&result.PlayerPosition, &result.PlayerPoints, &result.IsWinner,
		); err != nil {
			return nil, fmt.Errorf("failed to scan player result: %w", err)
		}
		results = append(results, result)
	}
	return results, nil
}

func (d *PgDatabase) CountGamesInSession(sessionID string) (int, error) {
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

func (d *PgDatabase) GetSessionResults(sessionID string) ([]game.PlayerResultState, error) {
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
		if err := rows.Scan(&result.GameID, &result.SessionID, &result.PlayerID,
			&result.PlayerPosition, &result.PlayerPoints, &result.IsWinner); err != nil {
			return nil, fmt.Errorf("failed to scan player result: %w", err)
		}
		results = append(results, result)
	}

	return results, nil
}

func (d *PgDatabase) ListAgentProfiles() ([]ProfileEntry, error) {
	rows, err := d.DB.Query(`
		SELECT id, name, is_agent, profile_icon, is_online FROM profiles WHERE is_agent = TRUE
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list agent profiles: %w", err)
	}
	defer rows.Close()

	var profiles []ProfileEntry
	for rows.Next() {
		var profile ProfileEntry
		if err := rows.Scan(&profile.ID, &profile.Name, &profile.IsAgent, &profile.ProfileIcon, &profile.IsOnline); err != nil {
			return nil, fmt.Errorf("failed to scan agent profile: %w", err)
		}
		profiles = append(profiles, profile)
	}
	return profiles, nil
}

// CleanupStaleGames deletes games where no moves have been made in the specified minutes
// and no human players are currently online
func (d *PgDatabase) CleanupStaleGames(inactiveMinutes int, onlinePlayerIDs []string) (int, error) {
	// Build a query to find stale games
	// A game is stale if:
	// 1. updated_at is older than inactiveMinutes
	// 2. No human players in the game are currently online

	var result sql.Result
	var err error

	if len(onlinePlayerIDs) > 0 {
		// Use PostgreSQL array type for the NOT IN clause
		query := `
			DELETE FROM games
			WHERE updated_at < NOW() - INTERVAL '1 minute' * $1
			AND id NOT IN (
				SELECT DISTINCT p.game_id
				FROM players p
				JOIN profiles pr ON p.profile_id = pr.id
				WHERE pr.is_agent = FALSE
				AND p.profile_id = ANY($2::text[])
			)
		`
		result, err = d.DB.Exec(query, inactiveMinutes, onlinePlayerIDs)
	} else {
		// No online players, so we just check the timestamp
		query := `
			DELETE FROM games
			WHERE updated_at < NOW() - INTERVAL '1 minute' * $1
		`
		result, err = d.DB.Exec(query, inactiveMinutes)
	}

	if err != nil {
		return 0, fmt.Errorf("failed to cleanup stale games: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return int(rowsAffected), nil
}

func (d *PgDatabase) GetActiveGamesByPlayer(playerID string) ([]game.GameState, error) {
	rows, err := d.DB.Query(`
		SELECT DISTINCT g.id
		FROM games g
		JOIN players p ON g.id = p.game_id
		WHERE p.profile_id = $1 AND g.phase != $2
	`, playerID, game.PhaseComplete)
	if err != nil {
		return nil, fmt.Errorf("failed to query active games by player: %w", err)
	}
	defer rows.Close()

	var games []game.GameState
	for rows.Next() {
		var gameID string
		if err := rows.Scan(&gameID); err != nil {
			return nil, fmt.Errorf("failed to scan game ID: %w", err)
		}

		gs, err := d.GetGameByID(gameID)
		if err != nil {
			return nil, fmt.Errorf("failed to get game %s: %w", gameID, err)
		}
		games = append(games, *gs)
	}

	return games, nil
}
