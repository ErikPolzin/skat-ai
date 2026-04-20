export interface Card {
  suit: string;
  rank: string;
}

export type { Player } from "./api/games";

export interface GameState {
  current_player?: number;
  phase?: string;
  trick?: Card[];
  hand?: Card[];
  position?: number;
  declarer_won?: boolean;
  declarer_score?: number;
}

export interface SessionGameResult {
  game_id: string;
  game_number: number;
  declarer_name: string;
  declarer_won: boolean;
  game_mode: "grand" | "suit" | "null";
  trump_suit: "♣" | "♠" | "♥" | "♦";
  game_value: number;
  player_results: { [playerId: string]: number };
  player_names: { [playerId: string]: string };
}

export interface Message {
  type: string;
  data: any;
}

export interface RoomInfo {
  id: string;
  player_count: number;
  started: boolean;
}
