export type CardState = "hidden" | "revealed" | "matched" | "removed";

export type ElementType = "fire" | "water" | "air" | "earth";

export interface CardView {
  index: number;
  pairId?: number;
  state: CardState;
  /** From server: element for normal cards when revealed (fire, water, air, earth). Only set for revealed/matched; never for hidden/removed or arcana. */
  element?: ElementType;
}

export interface PlayerView {
  name: string;
  score: number;
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
  /** Number of copies that can be used this turn (arcana have 1-turn cooldown after collection). */
  usableCount?: number;
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
  /** Card indices that have been revealed at some point (used when computing highlight). */
  knownIndices?: number[];
  /** From server: pairId -> power-up ID for arcana pairs. Defines which pairIds are arcana. */
  pairIdToPowerUp?: Record<string, string>;
  /** From server: number of arcana pairs (pairIDs 0..arcanaPairs-1). Remaining pairs are normal; their element is in card.element when revealed. */
  arcanaPairs?: number;
  /** Card indices to highlight (Unveiling: never-revealed hidden; Elementals: tiles of chosen element). Current turn only. */
  highlightIndices?: number[];
}
