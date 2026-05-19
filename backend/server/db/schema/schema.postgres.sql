CREATE TABLE IF NOT EXISTS profiles (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    is_agent BOOLEAN NOT NULL DEFAULT FALSE,
    profile_icon VARCHAR(255) DEFAULT '',
    is_online BOOLEAN NOT NULL DEFAULT FALSE,
    password_hash TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    last_seen TIMESTAMP NOT NULL DEFAULT NOW()
);

ALTER TABLE profiles ADD COLUMN IF NOT EXISTS password_hash TEXT NOT NULL DEFAULT '';

-- Insert initial agent profiles
INSERT INTO profiles (id, name, is_agent, profile_icon, is_online) VALUES
    -- Heuristic agents
    ('550e8400-e29b-41d4-a716-446655440001', 'Bill', TRUE, '/res/profile_icons/bill.svg', TRUE),
    ('550e8400-e29b-41d4-a716-446655440002', 'Dave', TRUE, '/res/profile_icons/dave.svg', TRUE),
    -- MCTS agents
    ('550e8400-e29b-41d4-a716-446655440003', 'Lisa', TRUE, '/res/profile_icons/lisa.svg', TRUE),
    ('550e8400-e29b-41d4-a716-446655440004', 'Max', TRUE, '/res/profile_icons/max.svg', TRUE),
    -- Neural agents
    ('550e8400-e29b-41d4-a716-446655440005', 'Emma', TRUE, '/res/profile_icons/emma.svg', TRUE),
    ('550e8400-e29b-41d4-a716-446655440006', 'Sam', TRUE, '/res/profile_icons/sam.svg', TRUE)
ON CONFLICT (id) DO NOTHING;

CREATE TABLE IF NOT EXISTS game_sessions (
    id VARCHAR(255) PRIMARY KEY,
    code VARCHAR(255) NOT NULL,
    game_id VARCHAR(255) DEFAULT NULL,
    player_count INT DEFAULT 0,
    max_games INT NOT NULL DEFAULT 10,
    pass_policy VARCHAR(50) NOT NULL DEFAULT 'force_listener',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    ended_at TIMESTAMP
);

CREATE TABLE IF NOT EXISTS games (
    id VARCHAR(255) PRIMARY KEY,
    session_id VARCHAR(255) NOT NULL,
    game_number INT DEFAULT 0,
    phase VARCHAR(50) NOT NULL,
    skat TEXT DEFAULT '',
    trick TEXT DEFAULT '',
    trick_starter INT DEFAULT 0,
    trick_winner INT DEFAULT 0,
    current_player INT DEFAULT 0,
    declarer INT,
    player_score_dealer INT DEFAULT 0,
    player_score_listener INT DEFAULT 0,
    player_score_speaker INT DEFAULT 0,
    game_mode VARCHAR(50) DEFAULT '',
    trump_suit INT DEFAULT 0,
    bid_value INT DEFAULT 0,
    matadors INT DEFAULT 0,
    played_hand BOOLEAN DEFAULT FALSE,
    announced_schneider BOOLEAN DEFAULT FALSE,
    announced_schwarz BOOLEAN DEFAULT FALSE,
    listener_passed BOOLEAN DEFAULT FALSE,
    speaker_passed BOOLEAN DEFAULT FALSE,
    dealer_passed BOOLEAN DEFAULT FALSE,
    overbid BOOLEAN DEFAULT FALSE,
    current_player_deadline TIMESTAMP,
    forfeited_player INTEGER,
    cards_played TEXT DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    FOREIGN KEY (session_id) REFERENCES game_sessions(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS players (
    profile_id VARCHAR(255) NOT NULL,
    game_id VARCHAR(255) NOT NULL,
    hand TEXT NOT NULL,
    position INT NOT NULL,
    ready_for_next BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY (game_id, profile_id),
    FOREIGN KEY (game_id) REFERENCES games(id) ON DELETE CASCADE,
    FOREIGN KEY (profile_id) REFERENCES profiles(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS player_results (
    id SERIAL PRIMARY KEY,
    game_id VARCHAR(255) NOT NULL,
    session_id VARCHAR(255) NOT NULL,
    player_id VARCHAR(255) NOT NULL,
    player_position INT NOT NULL,
    player_points INT DEFAULT 0,
    is_winner BOOLEAN DEFAULT FALSE,
    is_declarer BOOLEAN DEFAULT FALSE,
    FOREIGN KEY (session_id) REFERENCES game_sessions(id) ON DELETE CASCADE,
    FOREIGN KEY (player_id) REFERENCES profiles(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS player_session_results (
    id SERIAL PRIMARY KEY,
    session_id VARCHAR(255) NOT NULL,
    player_id VARCHAR(255) NOT NULL,
    player_points INT DEFAULT 0,
    is_winner BOOLEAN DEFAULT FALSE,
    is_forfeit BOOLEAN DEFAULT FALSE,
    rating_before INT DEFAULT 1500,
    rating_after INT DEFAULT 1500,
    rating_change INT DEFAULT 0,
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
    profile_id VARCHAR(255) PRIMARY KEY,
    rating INT NOT NULL DEFAULT 1500,
    games_played INT NOT NULL DEFAULT 0,
    wins INT NOT NULL DEFAULT 0,
    losses INT NOT NULL DEFAULT 0,
    peak_rating INT NOT NULL DEFAULT 1500,
    last_updated TIMESTAMP NOT NULL DEFAULT NOW(),
    FOREIGN KEY (profile_id) REFERENCES profiles(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS agent_configs (
    profile_id VARCHAR(255) PRIMARY KEY,
    bidding_type VARCHAR(50) NOT NULL DEFAULT 'heuristic',
    bidding_threshold DOUBLE PRECISION DEFAULT 0.65,
    game_choice_type VARCHAR(50) NOT NULL DEFAULT 'heuristic',
    card_play_type VARCHAR(50) NOT NULL DEFAULT 'heuristic',
    mcts_simulations INT DEFAULT 500,
    cardplay_weights_path VARCHAR(255),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    FOREIGN KEY (profile_id) REFERENCES profiles(id) ON DELETE CASCADE
);

-- Insert initial agent configs for agent profiles
INSERT INTO agent_configs (profile_id, bidding_type, bidding_threshold, game_choice_type, card_play_type, mcts_simulations, cardplay_weights_path)
VALUES
    -- Heuristic agents (Bill, Dave)
    ('550e8400-e29b-41d4-a716-446655440001', 'heuristic', 0.65, 'heuristic', 'heuristic', NULL, NULL),
    ('550e8400-e29b-41d4-a716-446655440002', 'heuristic', 0.70, 'heuristic', 'heuristic', NULL, NULL),
    -- MCTS agents (Lisa, Max)
    ('550e8400-e29b-41d4-a716-446655440003', 'heuristic', 0.65, 'heuristic', 'mcts', 500, NULL),
    ('550e8400-e29b-41d4-a716-446655440004', 'heuristic', 0.65, 'heuristic', 'mcts', 1000, NULL),
    -- Neural agents (Emma, Sam) - using combined weights file
    ('550e8400-e29b-41d4-a716-446655440005', 'heuristic', 0.65, 'heuristic', 'neural', NULL, '.data/models/cardplay.weights'),
    ('550e8400-e29b-41d4-a716-446655440006', 'heuristic', 0.70, 'heuristic', 'neural', NULL, '.data/models/cardplay.weights')
ON CONFLICT (profile_id) DO NOTHING;
