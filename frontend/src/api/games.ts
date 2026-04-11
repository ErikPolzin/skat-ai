import type { Player, Card } from "../types";

export type { Card } from "../types";

const getApiUrl = () => process.env.REACT_APP_API_URL || "";

export interface GameState {
  id: string;
  code: string; // Game code for joining
  players: (Player | undefined)[];
  current_player: number;
  phase: string;
  trick: Card[];
  declarer_score: number;
  opponent_score: number;
  declarer: number;
  game_over: boolean;
  game_mode?: string;
  trump_suit?: string;
  valid_bids?: string[];
  player_id?: string;
  hand?: Card[];
  declarer_pile_count?: number;
  opponent_pile_count?: number;
  trick_starter?: number;
  trick_winner?: number;
  declarer_tricks?: number; // For null games
  skat_cards?: Card[]; // Available during skat_exchange phase
  has_picked_up_skat?: boolean;
}

export async function fetchGameState(
  gameId: string,
  playerId?: string,
): Promise<GameState> {
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
): Promise<{ player_id: string; player_name: string; game_id: string }> {
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

export async function addAIAgent(
  gameId: string,
  agentType: string = "mcts",
): Promise<void> {
  const response = await fetch(`${getApiUrl()}/api/games/${gameId}/agents`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      agent_type: agentType,
    }),
  });

  if (!response.ok) {
    throw new Error("Failed to add AI");
  }
}

export async function getGames(): Promise<GameState[]> {
  const response = await fetch(`${getApiUrl()}/api/games?open=true`);
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
  game_mode: string;
  opponent_names: string[];
  vs_ai: boolean;
  finished_at: string;
}

export async function getPlayerGameHistory(
  playerId: string,
  limit: number = 10,
): Promise<GameHistoryEntry[]> {
  const response = await fetch(
    `${getApiUrl()}/api/players/${playerId}/history?limit=${limit}`,
  );

  if (!response.ok) {
    console.error("Failed to fetch game history");
    return [];
  }

  return response.json();
}
