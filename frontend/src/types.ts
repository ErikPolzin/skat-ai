import type { GameInfo } from "./api/games";

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
  player_scores?: [number, number, number];
}

export interface SessionGameResult {
  game_id: string;
  game_number: number;
  declarer_name: string;
  declarer_won: boolean;
  game_mode: "grand" | "suit" | "null" | "ramsch";
  trump_suit: "♣" | "♠" | "♥" | "♦";
  player_results: { [playerId: string]: number };
  player_names: { [playerId: string]: string };
}

export interface Message<T> {
  type: string;
  data: T;
}

export interface StateUpdateMessage extends Message<{
  diff: GameInfo;
  description?: string;
  action_type: string;
  session_results: SessionGameResult[];
  games_played?: number;
  from_player?: number;
}> {
  type: "state_update";
}

export interface StartNextGameMessage extends Message<{
  game_id: string;
}> {
  type: "start_next_game";
}

export interface PlayerOfflineMessage extends Message<{
  player_id?: string;
  player_name?: string;
}> {
  type: "player_offline";
}

export interface PlayerLeftMessage extends Message<{
  player_name?: string;
}> {
  type: "player_left";
}

export interface PlayerForfeitMessage extends Message<{
  player_name?: string;
}> {
  type: "player_forfeit";
}

export interface ErrorMessage extends Message<{
  message: string;
}> {
  type: "error";
}

export interface RoomInfo {
  id: string;
  player_count: number;
  started: boolean;
}
