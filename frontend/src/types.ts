export interface Card {
  suit: string;
  rank: string;
}

export interface Player {
  player_id: string;
  name: string;
  position: number;
  card_count?: number;
  is_agent?: boolean;
}

export interface GameState {
  current_player?: number;
  phase?: string;
  trick?: Card[];
  hand?: Card[];
  position?: number;
  declarer_won?: boolean;
  declarer_score?: number;
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
