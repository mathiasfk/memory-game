export interface PowerUpDisplayInfo {
  icon: string;
  label: string;
  /** Full description shown in the modal. */
  description: string;
  /** Brief description shown in the hand (one line / few words). */
  shortDescription: string;
  imagePath: string;
}

export const POWER_UP_DISPLAY: Record<string, PowerUpDisplayInfo> = {
  chaos: {
    icon: "CHS",
    label: "Chaos",
    description: "Reshuffles the positions of all cards that are not yet matched.",
    shortDescription: "Reshuffles unmatched cards.",
    imagePath: "/cards/Chaos.webp",
  },
  clairvoyance: {
    icon: "CLV",
    label: "Clairvoyance",
    description: "Reveals a 3x3 area around the card you choose for 2 seconds, then hides it again.",
    shortDescription: "Reveals 3×3 area for 2 seconds.",
    imagePath: "/cards/Clairvoyance.webp",
  },
  necromancy: {
    icon: "NEC",
    label: "Necromancy",
    description: "Returns all collected tiles back to the board in new random positions.",
    shortDescription: "Returns matched tiles to the board.",
    imagePath: "/cards/Necromancy.webp",
  },
  unveiling: {
    icon: "UNV",
    label: "Unveiling",
    description: "Highlights all tiles that have never been revealed (this turn only).",
    shortDescription: "Highlights never-revealed tiles.",
    imagePath: "/cards/Unveiling.webp",
  },
  blood_pact: {
    icon: "BLP",
    label: "Blood Pact",
    description: "If you match 3 pairs in a row, you gain +5 points. If you fail (mismatch) before that, you lose 3 points.",
    shortDescription: "Match 3 in a row: +5; fail: -3.",
    imagePath: "/cards/BloodPact.webp",
  },
  leech: {
    icon: "LCH",
    label: "Leech",
    description: "This turn, points you earn from matching are subtracted from the opponent.",
    shortDescription: "Your match points drain from opponent.",
    imagePath: "/cards/Leech.webp",
  },
  oblivion: {
    icon: "OBL",
    label: "Oblivion",
    description: "Select a tile. It and its pair are removed from the game. No one gains or loses points.",
    shortDescription: "Remove a tile and its pair from the game.",
    imagePath: "/cards/Oblivion.webp",
  },
};

/** Fallback when server does not send pairIdToPowerUp (e.g. old client). */
const PAIR_ID_TO_POWER_UP_ID: Record<number, string> = {
  0: "chaos",
  1: "clairvoyance",
  2: "necromancy",
  3: "unveiling",
};

/**
 * Returns display info for the power-up at the given pair ID.
 * Uses pairIdToPowerUp from game state when provided (per-match arcana); otherwise fallback.
 */
export function getPowerUpDisplayByPairId(
  pairId: number,
  pairIdToPowerUp?: Record<string, string> | null
): PowerUpDisplayInfo | null {
  const powerUpId =
    pairIdToPowerUp != null && pairIdToPowerUp[String(pairId)] != null
      ? pairIdToPowerUp[String(pairId)]
      : PAIR_ID_TO_POWER_UP_ID[pairId];
  if (powerUpId == null) return null;
  return POWER_UP_DISPLAY[powerUpId] ?? null;
}

export const NUM_POWERUP_PAIRS = 4;
