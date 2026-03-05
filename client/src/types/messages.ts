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

export interface LeaveGameMsg {
  type: "leave_game";
}

export interface LeaveQueueMsg {
  type: "leave_queue";
}

export interface RejoinMsg {
  type: "rejoin";
  gameId: string;
  rejoinToken: string;
  name: string;
}

export interface RejoinMyGameMsg {
  type: "rejoin_my_game";
}

export type ClientMessage =
  | AuthMsg
  | SetNameMsg
  | RejoinMsg
  | RejoinMyGameMsg
  | FlipCardMsg
  | UsePowerUpMsg
  | PlayAgainMsg
  | LeaveGameMsg
  | LeaveQueueMsg;

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
  opponentUserId?: string;
  boardRows: number;
  boardCols: number;
  yourTurn: boolean;
  your_elo?: number;
  opponent_elo?: number;
}

export interface GameStateMsg extends GameState {
  type: "game_state";
}

export interface GameOverMsg {
  type: "game_over";
  result: "win" | "lose" | "draw";
  you: PlayerView;
  opponent: PlayerView;
  /** Your rating before this game (included when ratings were updated). */
  you_elo_before?: number;
  /** Your rating after this game (included when ratings were updated). */
  you_elo_after?: number;
}

/** Sent after game_over when ELO was computed asynchronously (e.g. after DB persist). */
export interface RatingUpdateMsg {
  type: "rating_update";
  you_elo_before: number;
  you_elo_after: number;
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

export interface PowerUpUsedMsg {
  type: "powerup_used";
  playerName: string;
  powerUpLabel: string;
  noEffect?: boolean;
}

export interface PowerUpEffectResolvedMsg {
  type: "powerup_effect_resolved";
  playerName: string;
  powerUpLabel: string;
  message: string;
}

export type ServerMessage =
  | ErrorMsg
  | WaitingForMatchMsg
  | MatchFoundMsg
  | GameStateMsg
  | GameOverMsg
  | RatingUpdateMsg
  | OpponentDisconnectedMsg
  | OpponentReconnectingMsg
  | OpponentReconnectedMsg
  | TurnTimeoutMsg
  | PowerUpUsedMsg
  | PowerUpEffectResolvedMsg;
