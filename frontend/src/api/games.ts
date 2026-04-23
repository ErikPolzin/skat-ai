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
  game_value: number;
  declarer_score: number;
  opponent_score: number;
  bid_value: number;
  listener_passed: boolean;
  speaker_passed: boolean;
  dealer_passed: boolean;
}

export interface GameInfo {
  state: GameState;
  player_id?: string;
  hand?: Card[];
  skat?: [Card, Card];
  can_play_next: boolean;
}

export interface GameSession {
  id: string;
  code: string;
  game_id?: string;
  player_count: number;
  created_at: string;
  ended_at?: string;
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

export async function createGame(): Promise<{ game_id: string; code: string }> {
  const response = await fetch(`${getApiUrl()}/api/games`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
  });

  if (!response.ok) {
    throw new Error("Failed to create game");
  }

  return response.json();
}

export async function createOrRetrieveProfile(
  playerName: string,
  playerId?: string,
): Promise<{ player_id: string; player_name: string }> {
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

export async function getGames(): Promise<GameSession[]> {
  const response = await fetch(`${getApiUrl()}/api/games`);
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
    game_value: number;
    player_results: { [playerId: string]: number };
    player_names: { [playerId: string]: string };
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
