import type { Card } from "../types";
import { useProfileStore } from "../stores/profileStore";

// Server player representation (without position)
export interface ServerPlayer {
  id: string;
  name: string;
  is_agent: boolean;
  profile_icon: string;
  is_online: boolean;
  ready_for_next: boolean;
}

// Client player representation (with position derived from array index)
export interface Player {
  id: string;
  name: string;
  is_agent: boolean;
  profile_icon: string;
  is_online: boolean;
  ready_for_next: boolean;
  position: number;
  card_count?: number;
}

export type { Card } from "../types";

const getApiUrl = () => import.meta.env.VITE_API_URL;

function authHeaders(
  username?: string | null,
  password?: string | null,
): HeadersInit {
  const state = useProfileStore.getState();
  const authUsername = username ?? state.username;
  const authPassword = password ?? state.password;
  if (!authUsername || !authPassword) {
    return {};
  }
  return {
    Authorization: `Basic ${btoa(`${authUsername}:${authPassword}`)}`,
  };
}

function jsonHeaders(
  username?: string | null,
  password?: string | null,
): HeadersInit {
  return {
    "Content-Type": "application/json",
    ...authHeaders(username, password),
  };
}

export type GameMode = "grand" | "suit" | "null" | "ramsch";
export type PassPolicy = "reshuffle" | "force_listener" | "ramsch";
export type TrumpSuit = "♣" | "♠" | "♥" | "♦";
export type GamePosition = 0 | 1 | 2;

export interface GameState {
  id: string;
  code: string;
  session_id: string;
  game_number: number;
  max_games: number;
  pass_policy: PassPolicy;
  timer_enabled: boolean;
  players: [ServerPlayer | null, ServerPlayer | null, ServerPlayer | null];
  current_player: GamePosition;
  declarer: GamePosition | null;
  mode: GameMode;
  trump_suit: TrumpSuit;
  trick: Card[] | null;
  trick_winner: GamePosition | null;
  trick_starter: GamePosition;
  phase: string;
  player_scores: [number, number, number];
  bid_value: number;
  listener_passed: boolean;
  speaker_passed: boolean;
  dealer_passed: boolean;
  matadors: number; // Positive=with, negative=without
  played_hand: boolean; // Declarer played without picking up skat
  announced_schneider: boolean; // Declarer announced schneider
  announced_schwarz: boolean; // Declarer announced schwarz
  current_player_deadline: string; // RFC3339 timestamp when current player times out
  forfeited_player: GamePosition | null; // Position of player who forfeited, if any
}

export interface GameResult {
  base_value: number;
  matadors: number;
  multiplier: number;
  declarer_won: boolean;
  is_schneider: boolean;
  is_schwarz: boolean;
  played_hand: boolean;
  announced_schneider: boolean;
  announced_schwarz: boolean;
  value: number;
  is_forfeit?: boolean;
}

export interface GameInfo {
  state: GameState;
  player_id?: string;
  hand?: Card[];
  skat?: [Card, Card];
  can_play_next: boolean;
  result?: GameResult;
}

export interface GameSession {
  id: string;
  code: string;
  game_id?: string;
  player_count: number;
  max_games: number;
  pass_policy: PassPolicy;
  timer_enabled: boolean;
  created_at: string;
  ended_at?: string;
}

export interface ActiveGame {
  id: string;
  code: string;
  session_id: string;
  game_number: number;
  player_count: number;
  phase: string;
  player_names: string[];
}

export async function fetchGameState(gameId: string): Promise<GameInfo> {
  const url = `${getApiUrl()}/api/games/${gameId}`;

  const response = await fetch(url, { headers: authHeaders() });

  if (!response.ok) {
    throw new Error("Failed to fetch game state");
  }

  return response.json();
}

export interface CreateGameOptions {
  max_games: number;
  pass_policy: PassPolicy;
  timer_enabled: boolean;
}

export async function createGame(
  options?: CreateGameOptions,
): Promise<{ game_id: string; code: string }> {
  const url = `${getApiUrl()}/api/games`;

  const response = await fetch(url, {
    method: "POST",
    headers: jsonHeaders(),
    body: options ? JSON.stringify(options) : undefined,
  });

  if (!response.ok) {
    const errorText = await response.text();
    throw new Error(errorText || "Failed to create game");
  }

  return response.json();
}

export async function createOrRetrieveProfile(
  playerName: string,
  password: string,
): Promise<{ player_id: string; player_name: string; profile_icon: string }> {
  const response = await fetch(`${getApiUrl()}/api/profiles`, {
    method: "POST",
    headers: jsonHeaders(playerName, password),
    body: JSON.stringify({
      player_name: playerName,
    }),
  });

  if (!response.ok) {
    throw new Error("Failed to create/retrieve profile");
  }

  return response.json();
}

export async function joinGame(gameId: string): Promise<{ game_id: string }> {
  const response = await fetch(`${getApiUrl()}/api/games/${gameId}/join`, {
    method: "POST",
    headers: authHeaders(),
  });

  if (!response.ok) {
    throw new Error("Failed to join game");
  }

  return response.json();
}

export async function addAIAgent(
  gameId: string,
  agentId?: string,
): Promise<void> {
  const response = await fetch(`${getApiUrl()}/api/games/${gameId}/agents`, {
    method: "POST",
    headers: jsonHeaders(),
    body: agentId ? JSON.stringify({ agent_id: agentId }) : undefined,
  });

  if (!response.ok) {
    throw new Error("Failed to add AI");
  }
}

export async function getGames(
  excludePlayerId?: string,
): Promise<GameSession[]> {
  const url = excludePlayerId
    ? `${getApiUrl()}/api/games?exclude_player_id=${excludePlayerId}`
    : `${getApiUrl()}/api/games`;
  const response = await fetch(url, { headers: authHeaders() });
  const data = await response.json();
  return data || [];
}

export async function getActiveGames(playerId: string): Promise<ActiveGame[]> {
  const response = await fetch(
    `${getApiUrl()}/api/players/${playerId}/active_games`,
    { headers: authHeaders() },
  );

  if (!response.ok) {
    console.error("Failed to fetch active games");
    return [];
  }

  const data = await response.json();
  return data || [];
}

export interface PlayerResult {
  session_id: string;
  player_id: string;
  player_points: number;
  is_winner: boolean;
  is_forfeit?: boolean;
  other_players?: string[];
  rating_before?: number;
  rating_after?: number;
  rating_change?: number;
}

export type SessionPlayerResult = PlayerResult;

export interface PlayerRating {
  profile_id: string;
  name: string;
  rating: number;
  games_played: number;
  wins: number;
  losses: number;
  peak_rating: number;
  rank?: number;
  timeline?: number[];
}

export interface LeaderboardEntry {
  rank: number;
  profile_id: string;
  name: string;
  profile_icon: string;
  rating: number;
  games_played: number;
  wins: number;
  losses: number;
  win_rate: number;
}

export async function getPlayerHistory(
  playerId: string,
  limit: number = 50,
): Promise<PlayerResult[]> {
  const response = await fetch(
    `${getApiUrl()}/api/players/${playerId}/history?limit=${limit}`,
    { headers: authHeaders() },
  );

  if (!response.ok) {
    console.error("Failed to fetch player history");
    return [];
  }

  const data = await response.json();
  return data || [];
}

export interface GameHistoryEntry {
  game_id: string;
  game_code: string;
  player_id: string;
  player_name: string;
  is_winner: boolean;
  is_declarer: boolean;
  final_score: number;
  game_mode: GameMode;
  opponent_names: string[];
  vs_ai: boolean;
  finished_at: string;
}

export async function getPlayerGameHistory(
  playerId: string,
  sessionId: string,
): Promise<GameHistoryEntry[]> {
  const response = await fetch(
    `${getApiUrl()}/api/players/${playerId}/history/${sessionId}`,
    { headers: authHeaders() },
  );

  if (!response.ok) {
    console.error("Failed to fetch game history");
    return [];
  }

  return response.json();
}

export interface SessionResults {
  session_id: string;
  results: Array<{
    game_id: string;
    game_number: number;
    declarer_name: string;
    declarer_won: boolean;
    game_mode: GameMode;
    trump_suit: TrumpSuit;
    player_results: { [playerId: string]: number };
    player_names: { [playerId: string]: string };
    player_winners: { [playerId: string]: boolean };
    forfeited_player: GamePosition | null;
  }>;
  player_results?: SessionPlayerResult[];
}

export async function getSessionResults(
  sessionId: string,
): Promise<SessionResults> {
  const response = await fetch(
    `${getApiUrl()}/api/games/${sessionId}/results`,
    {
      headers: authHeaders(),
    },
  );

  if (!response.ok) {
    throw new Error("Failed to fetch session results");
  }

  return response.json();
}

export async function uploadAvatar(
  playerId: string,
  file: File,
): Promise<{ profile_icon: string }> {
  const formData = new FormData();
  formData.append("avatar", file);

  const response = await fetch(
    `${getApiUrl()}/api/profiles/${playerId}/avatar`,
    {
      method: "POST",
      headers: authHeaders(),
      body: formData,
    },
  );

  if (!response.ok) {
    throw new Error("Failed to upload avatar");
  }

  return response.json();
}

// Game action API calls
async function gameAction(
  gameId: string,
  action: string,
  _playerId: string,
  body?: object,
): Promise<void> {
  const url = `${getApiUrl()}/api/games/${gameId}/${action}`;
  const response = await fetch(url, {
    method: "POST",
    headers: jsonHeaders(),
    body: body ? JSON.stringify(body) : undefined,
  });

  if (!response.ok) {
    const error = await response.text();
    throw new Error(error || `HTTP ${response.status}`);
  }
}

export async function dealCards(
  gameId: string,
  playerId: string,
): Promise<void> {
  return gameAction(gameId, "deal", playerId);
}

export async function playCard(
  gameId: string,
  playerId: string,
  card: string,
): Promise<void> {
  return gameAction(gameId, "play_card", playerId, { card });
}

export async function bid(
  gameId: string,
  playerId: string,
  accept: boolean,
): Promise<void> {
  return gameAction(gameId, "bid", playerId, { accept });
}

export async function chooseGame(
  gameId: string,
  playerId: string,
  mode: string,
  trump: string,
  announceSchneider: boolean = false,
  announceSchwarz: boolean = false,
): Promise<void> {
  return gameAction(gameId, "choose_game", playerId, {
    mode,
    trump,
    announce_schneider: announceSchneider,
    announce_schwarz: announceSchwarz,
  });
}

export async function skatDecision(
  gameId: string,
  playerId: string,
  pickup: boolean,
): Promise<void> {
  return gameAction(gameId, "skat_decision", playerId, { pickup });
}

export async function discardCards(
  gameId: string,
  playerId: string,
  cards: string,
): Promise<void> {
  return gameAction(gameId, "discard_cards", playerId, { cards });
}

export async function readyForNextGame(
  gameId: string,
  playerId: string,
): Promise<void> {
  return gameAction(gameId, "ready_for_next", playerId);
}

export async function endTournament(gameId: string): Promise<SessionResults> {
  const response = await fetch(
    `${getApiUrl()}/api/games/${gameId}/end_tournament`,
    {
      method: "POST",
      headers: jsonHeaders(),
    },
  );

  if (!response.ok) {
    const error = await response.text();
    throw new Error(error || `HTTP ${response.status}`);
  }

  return response.json();
}

export async function reportTimeout(
  gameId: string,
  playerId: string,
): Promise<void> {
  return gameAction(gameId, "timeout", playerId);
}

export async function leaveGame(gameId: string): Promise<void> {
  const url = `${getApiUrl()}/api/games/${gameId}/leave`;
  const response = await fetch(url, {
    method: "POST",
    headers: jsonHeaders(),
  });

  if (!response.ok) {
    const error = await response.text();
    throw new Error(error || `HTTP ${response.status}`);
  }
}

// Rating and Leaderboard API calls

export async function getPlayerRating(playerId: string): Promise<PlayerRating> {
  const response = await fetch(
    `${getApiUrl()}/api/players/${playerId}/rating`,
    {
      headers: authHeaders(),
    },
  );

  if (!response.ok) {
    throw new Error("Failed to fetch player rating");
  }

  return response.json();
}

export async function getLeaderboard(
  limit: number = 100,
): Promise<LeaderboardEntry[]> {
  const response = await fetch(
    `${getApiUrl()}/api/leaderboard?limit=${limit}`,
    {
      headers: authHeaders(),
    },
  );

  if (!response.ok) {
    console.error("Failed to fetch leaderboard");
    return [];
  }

  const data = await response.json();
  return data || [];
}

export interface AgentInfo {
  id: string;
  name: string;
  profile_icon: string;
  bidding_type: string;
  bidding_threshold: number;
  game_choice_type: string;
  card_play_type: string;
  mcts_simulations?: number;
}

export async function getAvailableAgents(): Promise<AgentInfo[]> {
  const response = await fetch(`${getApiUrl()}/api/agents`, {
    headers: authHeaders(),
  });

  if (!response.ok) {
    console.error("Failed to fetch agents");
    return [];
  }

  const data = await response.json();
  return data || [];
}
