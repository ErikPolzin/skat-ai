package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"skat/game"
	"strings"
	"time"

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
	var isAgent, isOnline int
	err := d.DB.QueryRow(`
		SELECT id, name, is_agent, profile_icon, is_online FROM profiles WHERE id = ?
	`, profileID).Scan(&profile.ID, &profile.Name, &isAgent, &profile.ProfileIcon, &isOnline)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("player profile not found")
	}
	profile.IsAgent = isAgent != 0
	profile.IsOnline = isOnline != 0
	return &profile, err
}

func (d *TursoDatabase) GetProfileByName(name string) (*ProfileEntry, error) {
	var profile ProfileEntry
	var isAgent, isOnline int
	err := d.DB.QueryRow(`
		SELECT id, name, is_agent, profile_icon, is_online FROM profiles WHERE name = ?
	`, name).Scan(&profile.ID, &profile.Name, &isAgent, &profile.ProfileIcon, &isOnline)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("player profile not found")
	}
	profile.IsAgent = isAgent != 0
	profile.IsOnline = isOnline != 0
	return &profile, err
}

func (d *TursoDatabase) SaveProfile(profile ProfileEntry) error {
	isAgent := 0
	if profile.IsAgent {
		isAgent = 1
	}
	isOnline := 0
	if profile.IsOnline {
		isOnline = 1
	}
	_, err := d.DB.Exec(
		`INSERT INTO profiles (id, name, is_agent, profile_icon, is_online)
		 	VALUES (?, ?, ?, ?, ?)
		 	ON CONFLICT (id) DO UPDATE SET
		 	id = excluded.id, name = excluded.name, is_agent = excluded.is_agent,
			profile_icon = excluded.profile_icon, is_online = excluded.is_online`,
		profile.ID, profile.Name, isAgent, profile.ProfileIcon, isOnline,
	)
	if err != nil {
		return fmt.Errorf("failed to save profile: %w", err)
	}
	return nil
}

func (d *TursoDatabase) SaveGameSession(session game.GameSessionState) error {
	_, err := d.DB.Exec(
		`INSERT INTO game_sessions (id, code, game_id, player_count)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET
			code = excluded.code,
			game_id = excluded.game_id,
			player_count = excluded.player_count`,
		session.ID, session.Code, session.GameID, session.PlayerCount,
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
		WHERE id = ?
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
	var deadline sql.NullString
	err := d.DB.QueryRow(
		`SELECT g.id, g.session_id, gss.code, g.game_number, g.phase, g.skat, g.trick,
			g.trick_starter, g.trick_winner, g.current_player, g.declarer,
			g.declarer_score, g.opponent_score, g.game_mode, g.trump_suit,
			g.bid_value, g.matadors, g.played_hand, g.announced_schneider, g.announced_schwarz,
			g.listener_passed, g.speaker_passed, g.dealer_passed, g.overbid,
			g.current_player_deadline, g.forfeited_player
		FROM games g
		JOIN game_sessions gss ON g.session_id = gss.id
		WHERE g.id = ?`,
		gameID,
	).Scan(
		&gs.ID, &gs.SessionID, &gs.Code, &gs.GameNumber, &gs.Phase, &skatString, &trickString,
		&gs.TrickStarter, &gs.TrickWinner, &gs.CurrentPlayer, &gs.Declarer,
		&gs.DeclarerScore, &gs.OpponentScore, &gs.Mode, &gs.TrumpSuit,
		&gs.BidValue, &gs.Matadors, &gs.PlayedHand, &gs.AnnouncedSchneider, &gs.AnnouncedSchwarz,
		&gs.ListenerPassed, &gs.SpeakerPassed, &gs.DealerPassed, &gs.Overbid,
		&deadline, &gs.ForfeitedPlayer)
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

	// Handle nullable deadline
	if deadline.Valid {
		gs.CurrentPlayerDeadline = deadline.String
	} else {
		gs.CurrentPlayerDeadline = ""
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
	var deadline sql.NullString
	err := d.DB.QueryRow(
		`SELECT g.id, g.session_id, gs.code, g.game_number, g.phase, g.skat, g.trick,
			g.trick_starter, g.trick_winner, g.current_player, g.declarer,
			g.declarer_score, g.opponent_score, g.game_mode, g.trump_suit,
			g.bid_value, g.matadors, g.played_hand, g.announced_schneider, g.announced_schwarz,
			g.listener_passed, g.speaker_passed, g.dealer_passed, g.overbid,
			g.current_player_deadline, g.forfeited_player
		FROM games g
		JOIN game_sessions gs ON g.session_id = gs.id
		WHERE gs.code = ?
		ORDER BY g.created_at DESC
		LIMIT 1`,
		sessionCode,
	).Scan(
		&gs.ID, &gs.SessionID, &gs.Code, &gs.GameNumber, &gs.Phase, &skatString, &trickString,
		&gs.TrickStarter, &gs.TrickWinner, &gs.CurrentPlayer, &gs.Declarer,
		&gs.DeclarerScore, &gs.OpponentScore, &gs.Mode, &gs.TrumpSuit,
		&gs.BidValue, &gs.Matadors, &gs.PlayedHand, &gs.AnnouncedSchneider, &gs.AnnouncedSchwarz,
		&gs.ListenerPassed, &gs.SpeakerPassed, &gs.DealerPassed, &gs.Overbid,
		&deadline, &gs.ForfeitedPlayer)
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

	// Handle nullable deadline
	if deadline.Valid {
		gs.CurrentPlayerDeadline = deadline.String
	} else {
		gs.CurrentPlayerDeadline = ""
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

	// Handle empty deadline as NULL
	var deadline interface{}
	if gs.CurrentPlayerDeadline == "" {
		deadline = nil
	} else {
		deadline = gs.CurrentPlayerDeadline
	}

	// SQLite/Turso syntax
	_, err := d.DB.Exec(
		`INSERT INTO games (
			id, session_id, game_number, phase, skat, trick,
			trick_starter, trick_winner, current_player,
			declarer, declarer_score, opponent_score,
			game_mode, trump_suit, bid_value, matadors,
			played_hand, announced_schneider, announced_schwarz,
			listener_passed, speaker_passed, dealer_passed, overbid,
			current_player_deadline, forfeited_player,
			created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
			CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT (id) DO UPDATE SET
			session_id = excluded.session_id, game_number = excluded.game_number,
			phase = excluded.phase, skat = excluded.skat, trick = excluded.trick,
			trick_starter = excluded.trick_starter, trick_winner = excluded.trick_winner,
			current_player = excluded.current_player,
			declarer = excluded.declarer, declarer_score = excluded.declarer_score,
			opponent_score = excluded.opponent_score,
			game_mode = excluded.game_mode, trump_suit = excluded.trump_suit,
			bid_value = excluded.bid_value, matadors = excluded.matadors,
			played_hand = excluded.played_hand, announced_schneider = excluded.announced_schneider,
			announced_schwarz = excluded.announced_schwarz,
			listener_passed = excluded.listener_passed, speaker_passed = excluded.speaker_passed,
			dealer_passed = excluded.dealer_passed, overbid = excluded.overbid,
			current_player_deadline = excluded.current_player_deadline,
			forfeited_player = excluded.forfeited_player,
			updated_at = CURRENT_TIMESTAMP`,
		gs.ID, gs.SessionID, gs.GameNumber, gs.Phase, skatString, trickString,
		gs.TrickStarter, gs.TrickWinner, gs.CurrentPlayer,
		gs.Declarer, gs.DeclarerScore, gs.OpponentScore,
		gs.Mode, gs.TrumpSuit, gs.BidValue, gs.Matadors,
		gs.PlayedHand, gs.AnnouncedSchneider, gs.AnnouncedSchwarz,
		gs.ListenerPassed, gs.SpeakerPassed, gs.DealerPassed, gs.Overbid,
		deadline, gs.ForfeitedPlayer,
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
				VALUES (?, ?, ?, ?)
				ON CONFLICT (game_id, profile_id) DO UPDATE SET
					hand = excluded.hand, position = excluded.position`,
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
	// Query for games in waiting_for_players phase
	// Count actual players dynamically instead of relying on stale player_count column
	rows, err := d.DB.Query(`
		SELECT gs.id, gs.code, gs.game_id, COALESCE(COUNT(p.profile_id), 0) as player_count, gs.created_at, gs.ended_at
		FROM game_sessions gs
		JOIN games g ON g.id = gs.game_id
		LEFT JOIN players p ON p.game_id = g.id
		WHERE g.phase = 'waiting_for_players'
		GROUP BY gs.id, gs.code, gs.game_id, gs.created_at, gs.ended_at
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
	_, err := d.DB.Exec(`DELETE FROM games WHERE id = ?`, gameID)
	if err != nil {
		return fmt.Errorf("failed to delete game: %w", err)
	}

	return nil
}

func (d *TursoDatabase) ListPlayers(gameID string) ([3]*game.PlayerState, error) {
	rows, err := d.DB.Query(`
		SELECT pl.hand, pl.position, pr.id, pr.name, pr.is_agent, pr.profile_icon, pr.is_online
		FROM players pl
		JOIN profiles pr ON pr.id = pl.profile_id
		WHERE pl.game_id = ?
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
		var isAgent, isOnline int
		if err := rows.Scan(
			&handString, &position, &ps.ID, &ps.Name, &isAgent, &ps.ProfileIcon, &isOnline); err != nil {
			return [3]*game.PlayerState{}, fmt.Errorf("failed to scan player: %w", err)
		}
		ps.Hand, err = game.ParseCards(handString)
		if err != nil {
			return [3]*game.PlayerState{}, fmt.Errorf("cannot parse hand: %s", handString)
		}
		ps.IsAgent = isAgent != 0
		ps.IsOnline = isOnline != 0
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
				game_id, session_id, player_id, player_position, player_points, is_winner,
				rating_before, rating_after, rating_change
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			result.GameID, result.SessionID, result.PlayerID, result.PlayerPosition, result.PlayerPoints, isWinner,
			result.RatingBefore, result.RatingAfter, result.RatingChange,
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
		WHERE session_id = ? AND phase = 'complete'
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
		WHERE session_id = ?
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
		SELECT pr.game_id, pr.session_id, pr.player_id, pr.player_position, pr.player_points, pr.is_winner,
			   pr.rating_before, pr.rating_after, pr.rating_change,
			   GROUP_CONCAT(DISTINCT CASE WHEN p.profile_id != pr.player_id THEN prof.name END) AS other_players
		FROM player_results pr
		JOIN games g ON g.id = pr.game_id
		JOIN players p ON p.game_id = g.id
		JOIN profiles prof ON prof.id = p.profile_id
		WHERE pr.player_id = ?
		GROUP BY pr.id, pr.game_id, pr.session_id, pr.player_id, pr.player_position, pr.player_points, pr.is_winner,
		         pr.rating_before, pr.rating_after, pr.rating_change
		ORDER BY pr.id DESC
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
		var otherPlayersStr *string
		if err := rows.Scan(
			&result.GameID, &result.SessionID, &result.PlayerID,
			&result.PlayerPosition, &result.PlayerPoints, &isWinner,
			&result.RatingBefore, &result.RatingAfter, &result.RatingChange,
			&otherPlayersStr,
		); err != nil {
			return nil, fmt.Errorf("failed to scan player result: %w", err)
		}
		result.IsWinner = isWinner != 0

		// Convert comma-separated string to []string
		if otherPlayersStr != nil && *otherPlayersStr != "" {
			result.OtherPlayers = strings.Split(*otherPlayersStr, ",")
		}

		results = append(results, result)
	}
	return results, nil
}

func (d *TursoDatabase) ListAgentProfiles() ([]ProfileEntry, error) {
	rows, err := d.DB.Query(`
		SELECT id, name, is_agent, profile_icon, is_online FROM profiles WHERE is_agent = 1
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list agent profiles: %w", err)
	}
	defer rows.Close()

	var profiles []ProfileEntry
	for rows.Next() {
		var profile ProfileEntry
		var isAgent, isOnline int
		if err := rows.Scan(&profile.ID, &profile.Name, &isAgent, &profile.ProfileIcon, &isOnline); err != nil {
			return nil, fmt.Errorf("failed to scan agent profile: %w", err)
		}
		profile.IsAgent = isAgent != 0
		profile.IsOnline = isOnline != 0
		profiles = append(profiles, profile)
	}
	return profiles, nil
}

// CleanupStaleGames deletes games where no moves have been made in the specified minutes
// and no human players are currently online
func (d *TursoDatabase) CleanupStaleGames(inactiveMinutes int, onlinePlayerIDs []string) (int, error) {
	// Build a query to find stale games
	// A game is stale if:
	// 1. updated_at is older than inactiveMinutes
	// 2. No human players in the game are currently online

	// Build the NOT IN clause for online players
	onlineClause := ""
	if len(onlinePlayerIDs) > 0 {
		placeholders := make([]string, len(onlinePlayerIDs))
		for i := range placeholders {
			placeholders[i] = "?"
		}
		onlineClause = fmt.Sprintf(`
			AND g.id NOT IN (
				SELECT DISTINCT p.game_id
				FROM players p
				JOIN profiles pr ON p.profile_id = pr.id
				WHERE pr.is_agent = 0
				AND p.profile_id IN (%s)
			)`, strings.Join(placeholders, ","))
	}

	query := fmt.Sprintf(`
		DELETE FROM games
		WHERE id IN (
			SELECT g.id
			FROM games g
			LEFT JOIN players p ON g.id = p.game_id
			LEFT JOIN profiles pr ON p.profile_id = pr.id
			WHERE datetime(g.updated_at) < datetime('now', '-%d minutes')
			%s
		)
	`, inactiveMinutes, onlineClause)

	// Execute the delete query
	args := make([]interface{}, len(onlinePlayerIDs))
	for i, id := range onlinePlayerIDs {
		args[i] = id
	}

	result, err := d.DB.Exec(query, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup stale games: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return int(rowsAffected), nil
}

func (d *TursoDatabase) GetActiveGamesByPlayer(playerID string) ([]game.GameState, error) {
	rows, err := d.DB.Query(`
		SELECT DISTINCT g.id
		FROM games g
		JOIN players p ON g.id = p.game_id
		WHERE p.profile_id = ? AND g.phase != ?
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

func (d *TursoDatabase) GetAllExpiredGames() ([]game.GameState, error) {
	rows, err := d.DB.Query(`
		SELECT id
		FROM games
		WHERE phase != ? AND phase != ? AND current_player_deadline != '' AND current_player_deadline < datetime('now')
	`, game.PhaseComplete, game.PhaseWaitingForPlayers)
	if err != nil {
		return nil, fmt.Errorf("failed to query expired games: %w", err)
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

// Rating methods

func (d *TursoDatabase) GetPlayerRating(profileID string) (*PlayerRating, error) {
	var rating PlayerRating
	err := d.DB.QueryRow(`
		SELECT profile_id, rating, games_played, wins, losses, peak_rating, last_updated
		FROM player_ratings
		WHERE profile_id = ?
	`, profileID).Scan(
		&rating.ProfileID, &rating.Rating, &rating.GamesPlayed,
		&rating.Wins, &rating.Losses, &rating.PeakRating, &rating.LastUpdated,
	)
	if err == sql.ErrNoRows {
		// Return default rating for new player
		return &PlayerRating{
			ProfileID:   profileID,
			Rating:      1500,
			GamesPlayed: 0,
			Wins:        0,
			Losses:      0,
			PeakRating:  1500,
			LastUpdated: time.Time{},
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get player rating: %w", err)
	}
	return &rating, nil
}

func (d *TursoDatabase) SavePlayerRating(rating PlayerRating) error {
	_, err := d.DB.Exec(`
		INSERT INTO player_ratings (profile_id, rating, games_played, wins, losses, peak_rating, last_updated)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (profile_id) DO UPDATE SET
			rating = excluded.rating,
			games_played = excluded.games_played,
			wins = excluded.wins,
			losses = excluded.losses,
			peak_rating = excluded.peak_rating,
			last_updated = excluded.last_updated
	`, rating.ProfileID, rating.Rating, rating.GamesPlayed, rating.Wins, rating.Losses, rating.PeakRating, rating.LastUpdated)
	if err != nil {
		return fmt.Errorf("failed to save player rating: %w", err)
	}
	return nil
}

func (d *TursoDatabase) GetLeaderboard(limit int) ([]PlayerRating, error) {
	query := `
		SELECT pr.profile_id, pr.rating, pr.games_played, pr.wins, pr.losses, pr.peak_rating, pr.last_updated
		FROM player_ratings pr
		JOIN profiles p ON p.id = pr.profile_id
		ORDER BY pr.rating DESC
	`
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := d.DB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get leaderboard: %w", err)
	}
	defer rows.Close()

	var ratings []PlayerRating
	for rows.Next() {
		var rating PlayerRating
		if err := rows.Scan(
			&rating.ProfileID, &rating.Rating, &rating.GamesPlayed,
			&rating.Wins, &rating.Losses, &rating.PeakRating, &rating.LastUpdated,
		); err != nil {
			return nil, fmt.Errorf("failed to scan rating: %w", err)
		}
		ratings = append(ratings, rating)
	}
	return ratings, nil
}
