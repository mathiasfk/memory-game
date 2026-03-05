export interface GameRecord {
  id: string;
  played_at: string;
  game_id: string;
  player0_user_id: string;
  player1_user_id: string;
  player0_name: string;
  player1_name: string;
  player0_score: number;
  player1_score: number;
  winner_index: number | null;
  end_reason: string;
  your_index: number | null;
  player0_elo_before?: number | null;
  player1_elo_before?: number | null;
  player0_elo_after?: number | null;
  player1_elo_after?: number | null;
}

export interface HistoryResponse {
  games: GameRecord[];
  has_more: boolean;
}
