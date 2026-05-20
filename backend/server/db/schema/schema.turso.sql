CREATE TABLE IF NOT EXISTS profiles (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    is_agent INTEGER NOT NULL DEFAULT 0,
    profile_icon TEXT DEFAULT '',
    is_online INTEGER NOT NULL DEFAULT 0,
    password_hash TEXT NOT NULL DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_seen DATETIME DEFAULT CURRENT_TIMESTAMP
);

ALTER TABLE profiles ADD COLUMN password_hash TEXT NOT NULL DEFAULT '';

-- Insert initial agent profiles
INSERT OR IGNORE INTO profiles (id, name, is_agent, profile_icon, is_online) VALUES
    -- Heuristic agents
    ('550e8400-e29b-41d4-a716-446655440001', 'Bill', 1, '/res/profile_icons/bill.svg', 1),
    ('550e8400-e29b-41d4-a716-446655440002', 'Dave', 1, '/res/profile_icons/dave.svg', 1),
    -- MCTS agents
    ('550e8400-e29b-41d4-a716-446655440003', 'Lisa', 1, '/res/profile_icons/lisa.svg', 1),
    ('550e8400-e29b-41d4-a716-446655440004', 'Max', 1, '/res/profile_icons/max.svg', 1),
    -- Neural agents
    ('550e8400-e29b-41d4-a716-446655440005', 'Emma', 1, '/res/profile_icons/emma.svg', 1),
    ('550e8400-e29b-41d4-a716-446655440006', 'Sam', 1, '/res/profile_icons/sam.svg', 1);

CREATE TABLE IF NOT EXISTS game_sessions (
    id TEXT PRIMARY KEY,
    code TEXT NOT NULL,
    game_id TEXT DEFAULT NULL,
    player_count INT DEFAULT 0,
    max_games INTEGER NOT NULL DEFAULT 10,
    pass_policy TEXT NOT NULL DEFAULT 'reshuffle',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    ended_at DATETIME
);

CREATE TABLE IF NOT EXISTS games (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    game_number INTEGER DEFAULT 0,
    phase TEXT NOT NULL,
    skat TEXT DEFAULT '',
    trick TEXT DEFAULT '',
    trick_starter INTEGER DEFAULT 0,
    trick_winner INTEGER DEFAULT 0,
    current_player INTEGER DEFAULT 0,
    declarer INTEGER,
    player_score_dealer INTEGER DEFAULT 0,
    player_score_listener INTEGER DEFAULT 0,
    player_score_speaker INTEGER DEFAULT 0,
    game_mode TEXT DEFAULT '',
    trump_suit INTEGER DEFAULT 0,
    bid_value INTEGER DEFAULT 0,
    matadors INTEGER DEFAULT 0,
    played_hand INTEGER DEFAULT 0,
    announced_schneider INTEGER DEFAULT 0,
    announced_schwarz INTEGER DEFAULT 0,
    listener_passed INTEGER DEFAULT 0,
    speaker_passed INTEGER DEFAULT 0,
    dealer_passed INTEGER DEFAULT 0,
    overbid INTEGER DEFAULT 0,
    current_player_deadline DATETIME,
    forfeited_player INTEGER,
    cards_played TEXT DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (session_id) REFERENCES game_sessions(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS players (
    game_id TEXT NOT NULL,
    profile_id TEXT NOT NULL,
    hand TEXT NOT NULL,
    position INTEGER NOT NULL,
    ready_for_next INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (game_id, profile_id),
    FOREIGN KEY (game_id) REFERENCES games(id) ON DELETE CASCADE,
    FOREIGN KEY (profile_id) REFERENCES profiles(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS player_results (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    game_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    player_id TEXT NOT NULL,
    player_position INTEGER NOT NULL,
    player_points INTEGER DEFAULT 0,
    is_winner INTEGER DEFAULT 0,
    is_declarer INTEGER DEFAULT 0,
    FOREIGN KEY (session_id) REFERENCES game_sessions(id) ON DELETE CASCADE,
    FOREIGN KEY (player_id) REFERENCES profiles(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS player_session_results (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL,
    player_id TEXT NOT NULL,
    player_points INTEGER DEFAULT 0,
    is_winner INTEGER DEFAULT 0,
    is_forfeit INTEGER DEFAULT 0,
    rating_before INTEGER DEFAULT 1500,
    rating_after INTEGER DEFAULT 1500,
    rating_change INTEGER DEFAULT 0,
    UNIQUE(session_id, player_id),
    FOREIGN KEY (session_id) REFERENCES game_sessions(id) ON DELETE CASCADE,
    FOREIGN KEY (player_id) REFERENCES profiles(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_players_game_id ON players(game_id);
CREATE INDEX IF NOT EXISTS idx_player_results_game_id ON player_results(game_id);
CREATE INDEX IF NOT EXISTS idx_player_results_player_id ON player_results(player_id);
CREATE INDEX IF NOT EXISTS idx_player_results_session_id ON player_results(session_id);
CREATE INDEX IF NOT EXISTS idx_player_session_results_player_id ON player_session_results(player_id);
CREATE INDEX IF NOT EXISTS idx_player_session_results_session_id ON player_session_results(session_id);

CREATE TABLE IF NOT EXISTS player_ratings (
    profile_id TEXT PRIMARY KEY,
    rating INTEGER NOT NULL DEFAULT 1500,
    games_played INTEGER NOT NULL DEFAULT 0,
    wins INTEGER NOT NULL DEFAULT 0,
    losses INTEGER NOT NULL DEFAULT 0,
    peak_rating INTEGER NOT NULL DEFAULT 1500,
    last_updated DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (profile_id) REFERENCES profiles(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS agent_configs (
    profile_id TEXT PRIMARY KEY,
    bidding_type TEXT NOT NULL DEFAULT 'heuristic',
    bidding_threshold REAL DEFAULT 0.65,
    game_choice_type TEXT NOT NULL DEFAULT 'heuristic',
    card_play_type TEXT NOT NULL DEFAULT 'heuristic',
    mcts_simulations INTEGER DEFAULT 500,
    cardplay_weights_path TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (profile_id) REFERENCES profiles(id) ON DELETE CASCADE
);

-- Insert initial agent configs for agent profiles
INSERT OR IGNORE INTO agent_configs (profile_id, bidding_type, bidding_threshold, game_choice_type, card_play_type, mcts_simulations, cardplay_weights_path)
VALUES
    -- Heuristic agents (Bill, Dave)
    ('550e8400-e29b-41d4-a716-446655440001', 'heuristic', 0.65, 'heuristic', 'heuristic', NULL, NULL),
    ('550e8400-e29b-41d4-a716-446655440002', 'heuristic', 0.70, 'heuristic', 'heuristic', NULL, NULL),
    -- MCTS agents (Lisa, Max)
    ('550e8400-e29b-41d4-a716-446655440003', 'heuristic', 0.65, 'heuristic', 'mcts', 500, NULL),
    ('550e8400-e29b-41d4-a716-446655440004', 'heuristic', 0.65, 'heuristic', 'mcts', 1000, NULL),
    -- Neural agents (Emma, Sam) - using GCS bucket path for combined weights
    ('550e8400-e29b-41d4-a716-446655440005', 'heuristic', 0.65, 'heuristic', 'neural', NULL, 'gs://skat-ai-weights/cardplay.weights'),
    ('550e8400-e29b-41d4-a716-446655440006', 'heuristic', 0.70, 'heuristic', 'neural', NULL, 'gs://skat-ai-weights/cardplay.weights');
