export interface PowerUpDisplayInfo {
  icon: string;
  label: string;
  description: string;
  imagePath: string;
}

export const POWER_UP_DISPLAY: Record<string, PowerUpDisplayInfo> = {
  chaos: {
    icon: "CHS",
    label: "Chaos",
    description: "Reshuffles the positions of all cards that are not yet matched.",
    imagePath: "/cards/Chaos.webp",
  },
  clairvoyance: {
    icon: "CLV",
    label: "Clairvoyance",
    description: "Reveals a 3x3 area around the card you choose for 2 seconds, then hides it again.",
    imagePath: "/cards/Clairvoyance.webp",
  },
  necromancy: {
    icon: "NEC",
    label: "Necromancy",
    description: "Returns all collected tiles back to the board in new random positions.",
    imagePath: "/cards/Necromancy.webp",
  },
  unveiling: {
    icon: "UNV",
    label: "Unveiling",
    description: "Highlights all tiles that have never been revealed (this turn only).",
    imagePath: "/cards/Unveiling.webp",
  },
};

/** Server maps pairId 0..3 to these power-up IDs (registry order). */
const PAIR_ID_TO_POWER_UP_ID: Record<number, string> = {
  0: "chaos",
  1: "clairvoyance",
  2: "necromancy",
  3: "unveiling",
};

export function getPowerUpDisplayByPairId(pairId: number): PowerUpDisplayInfo | null {
  const powerUpId = PAIR_ID_TO_POWER_UP_ID[pairId];
  if (powerUpId == null) return null;
  return POWER_UP_DISPLAY[powerUpId] ?? null;
}

export const NUM_POWERUP_PAIRS = 4;
