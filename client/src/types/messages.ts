import type { GameState, PlayerView } from "./game";

export interface AuthMsg {
  type: "auth";
  token: string;
}

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

export interface RejoinMsg {
  type: "rejoin";
  gameId: string;
  rejoinToken: string;
  name: string;
}

export type ClientMessage =
  | AuthMsg
  | SetNameMsg
  | RejoinMsg
  | FlipCardMsg
  | UsePowerUpMsg
  | PlayAgainMsg;

export interface ErrorMsg {
  type: "error";
  message: string;
}

export interface WaitingForMatchMsg {
  type: "waiting_for_match";
}

export interface MatchFoundMsg {
  type: "match_found";
  gameId: string;
  rejoinToken: string;
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

export interface OpponentReconnectingMsg {
  type: "opponent_reconnecting";
  reconnectionDeadlineUnixMs: number;
}

export interface OpponentReconnectedMsg {
  type: "opponent_reconnected";
}

export interface TurnTimeoutMsg {
  type: "turn_timeout";
}

export type ServerMessage =
  | ErrorMsg
  | WaitingForMatchMsg
  | MatchFoundMsg
  | GameStateMsg
  | GameOverMsg
  | OpponentDisconnectedMsg
  | OpponentReconnectingMsg
  | OpponentReconnectedMsg
  | TurnTimeoutMsg;
