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
  /** Rounds the Second Chance power-up is still active (0 = inactive). */
  secondChanceRoundsRemaining?: number;
}

export interface PowerUpView {
  id: string;
  name: string;
  description: string;
  cost: number;
  canAfford: boolean;
}

export type TurnPhase = "first_flip" | "second_flip" | "resolve";

export interface GameState {
  cards: CardView[];
  you: PlayerView;
  opponent: PlayerView;
  yourTurn: boolean;
  availablePowerUps: PowerUpView[];
  flippedIndices: number[];
  phase: TurnPhase;
}
