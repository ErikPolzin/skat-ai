import type { Card } from "../types";

// Server player representation (without position)
export interface ServerPlayer {
  id: string;
  name: string;
  is_agent: boolean;
  profile_icon: string;
  is_online: boolean;
}

// Client player representation (with position derived from array index)
export interface Player {
  id: string;
  name: string;
  is_agent: boolean;
  profile_icon: string;
  is_online: boolean;
  position: number;
  card_count?: number;
}

export type { Card } from "../types";

const getApiUrl = () => process.env.REACT_APP_API_URL || "";

export type GameMode = "grand" | "suit" | "null";
export type TrumpSuit = "♣" | "♠" | "♥" | "♦";

export interface GameState {
  id: string;
  code: string;
  session_id: string;
  game_number: number;
  players: [ServerPlayer | null, ServerPlayer | null, ServerPlayer | null];
  current_player: number;
  declarer: number;
  mode: GameMode;
  trump_suit: TrumpSuit;
  trick: Card[] | null;
  trick_winner: number;
  trick_starter: number;
  phase: string;
  declarer_score: number;
  opponent_score: number;
  bid_value: number;
  listener_passed: boolean;
  speaker_passed: boolean;
  dealer_passed: boolean;
  matadors: number; // Positive=with, negative=without
  current_player_deadline: string; // RFC3339 timestamp when current player times out
}

export interface GameResult {
  base_value: number;
  matadors: number;
  multiplier: number;
  declarer_won: boolean;
  is_schneider: boolean;
  is_schwarz: boolean;
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

export async function fetchGameState(
  gameId: string,
  playerId?: string,
): Promise<GameInfo> {
  const url = playerId
    ? `${getApiUrl()}/api/games/${gameId}?player_id=${playerId}`
    : `${getApiUrl()}/api/games/${gameId}`;

  const response = await fetch(url);

  if (!response.ok) {
    throw new Error("Failed to fetch game state");
  }

  return response.json();
}

export async function createGame(
  playerId?: string,
): Promise<{ game_id: string; code: string }> {
  const url = playerId
    ? `${getApiUrl()}/api/games?player_id=${playerId}`
    : `${getApiUrl()}/api/games`;

  const response = await fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
  });

  if (!response.ok) {
    const errorText = await response.text();
    throw new Error(errorText || "Failed to create game");
  }

  return response.json();
}

export async function createOrRetrieveProfile(
  playerName: string,
  playerId?: string,
): Promise<{ player_id: string; player_name: string; profile_icon: string }> {
  const response = await fetch(`${getApiUrl()}/api/profiles`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      player_name: playerName,
      ...(playerId && { player_id: playerId }),
    }),
  });

  if (!response.ok) {
    throw new Error("Failed to create/retrieve profile");
  }

  return response.json();
}

export async function joinGame(
  gameId: string,
  playerName: string,
  playerId?: string,
): Promise<{ game_id: string }> {
  const response = await fetch(`${getApiUrl()}/api/games/${gameId}/join`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      player_name: playerName,
      ...(playerId && { player_id: playerId }),
    }),
  });

  if (!response.ok) {
    throw new Error("Failed to join game");
  }

  return response.json();
}

export async function addAIAgent(gameId: string): Promise<void> {
  const response = await fetch(`${getApiUrl()}/api/games/${gameId}/agents`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
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
  const response = await fetch(url);
  const data = await response.json();
  return data || [];
}

export async function getActiveGames(playerId: string): Promise<ActiveGame[]> {
  const response = await fetch(
    `${getApiUrl()}/api/players/${playerId}/active_games`,
  );

  if (!response.ok) {
    console.error("Failed to fetch active games");
    return [];
  }

  const data = await response.json();
  return data || [];
}

export interface PlayerResult {
  game_id: string;
  session_id: string;
  player_id: string;
  player_position: number;
  player_points: number;
  is_winner: boolean;
  other_players?: string[];
  rating_change?: number;
}

export interface PlayerRating {
  profile_id: string;
  name: string;
  rating: number;
  games_played: number;
  wins: number;
  losses: number;
  peak_rating: number;
  rank?: number;
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
    forfeited_player: number;
  }>;
}

export async function getSessionResults(
  sessionId: string,
): Promise<SessionResults> {
  const response = await fetch(`${getApiUrl()}/api/games/${sessionId}/results`);

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
  playerId: string,
  body?: any,
): Promise<void> {
  const url = `${getApiUrl()}/api/games/${gameId}/${action}?player_id=${encodeURIComponent(playerId)}`;
  const response = await fetch(url, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
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
): Promise<void> {
  return gameAction(gameId, "choose_game", playerId, { mode, trump });
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

export async function startNextGame(
  gameId: string,
  playerId: string,
): Promise<void> {
  return gameAction(gameId, "start_next_game", playerId);
}

export async function reportTimeout(
  gameId: string,
  playerId: string,
): Promise<void> {
  return gameAction(gameId, "timeout", playerId);
}

export async function leaveGame(
  gameId: string,
  playerId: string,
): Promise<void> {
  const url = `${getApiUrl()}/api/games/${gameId}/leave`;
  const response = await fetch(url, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ player_id: playerId }),
  });

  if (!response.ok) {
    const error = await response.text();
    throw new Error(error || `HTTP ${response.status}`);
  }
}

// Rating and Leaderboard API calls

export async function getPlayerRating(playerId: string): Promise<PlayerRating> {
  const response = await fetch(`${getApiUrl()}/api/players/${playerId}/rating`);

  if (!response.ok) {
    throw new Error("Failed to fetch player rating");
  }

  return response.json();
}

export async function getLeaderboard(
  limit: number = 100,
): Promise<LeaderboardEntry[]> {
  const response = await fetch(`${getApiUrl()}/api/leaderboard?limit=${limit}`);

  if (!response.ok) {
    console.error("Failed to fetch leaderboard");
    return [];
  }

  const data = await response.json();
  return data || [];
}
