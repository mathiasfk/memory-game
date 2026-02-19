import type { GameState, PlayerView } from "./game";

export interface SetNameMsg {
  type: "set_name";
  name: string;
}

export interface FlipCardMsg {
  type: "flip_card";
  index: number;
}

export interface UsePowerUpMsg {
  type: "use_power_up";
  powerUpId: string;
  cardIndex?: number;
}

export interface PlayAgainMsg {
  type: "play_again";
}

export type ClientMessage = SetNameMsg | FlipCardMsg | UsePowerUpMsg | PlayAgainMsg;

export interface ErrorMsg {
  type: "error";
  message: string;
}

export interface WaitingForMatchMsg {
  type: "waiting_for_match";
}

export interface MatchFoundMsg {
  type: "match_found";
  opponentName: string;
  boardRows: number;
  boardCols: number;
  yourTurn: boolean;
}

export interface GameStateMsg extends GameState {
  type: "game_state";
}

export interface GameOverMsg {
  type: "game_over";
  result: "win" | "lose" | "draw";
  you: Omit<PlayerView, "comboStreak">;
  opponent: Omit<PlayerView, "comboStreak">;
}

export interface OpponentDisconnectedMsg {
  type: "opponent_disconnected";
}

export type ServerMessage =
  | ErrorMsg
  | WaitingForMatchMsg
  | MatchFoundMsg
  | GameStateMsg
  | GameOverMsg
  | OpponentDisconnectedMsg;
