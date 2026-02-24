export type CardState = "hidden" | "revealed" | "matched";

export interface CardView {
  index: number;
  pairId?: number;
  state: CardState;
}

export interface PlayerView {
  name: string;
  score: number;
  comboStreak: number;
}

export interface PowerUpView {
  id: string;
  name: string;
  description: string;
  cost: number;
  canAfford: boolean;
}

/** One slot in the player's power-up hand (from server). */
export interface PowerUpInHand {
  powerUpId: string;
  count: number;
}

export type TurnPhase = "first_flip" | "second_flip" | "resolve";

export interface GameState {
  cards: CardView[];
  you: PlayerView;
  opponent: PlayerView;
  yourTurn: boolean;
  hand: PowerUpInHand[];
  flippedIndices: number[];
  phase: TurnPhase;
  /** When the current turn ends (Unix ms). Only set when it's your turn and timer is active. */
  turnEndsAtUnixMs?: number;
  /** How many seconds before turn end to show the countdown. */
  turnCountdownShowSec?: number;
  /** Card indices that have been revealed at some point (for Discernment: highlight tiles NOT in this list). */
  knownIndices?: number[];
  /** When true, highlight tiles that have never been revealed (hidden and not in knownIndices). */
  discernmentHighlightActive?: boolean;
}
