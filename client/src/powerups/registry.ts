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
    description: "Reshuffles the positions of all tiles that are not yet matched.",
    shortDescription: "Reshuffles unmatched tiles.",
    imagePath: "/cards/Chaos.webp",
  },
  clairvoyance: {
    icon: "CLV",
    label: "Clairvoyance",
    description: "Reveals a 3x3 area around the tile you choose for a few seconds, then hides it again.",
    shortDescription: "Reveals 3×3 area for a few seconds.",
    imagePath: "/cards/Clairvoyance.webp",
  },
  necromancy: {
    icon: "NEC",
    label: "Necromancy",
    description: "Returns all other collected tiles back to the board in new random positions. Tiles that were still on the board stay in place.",
    shortDescription: "Returns matched tiles to board.",
    imagePath: "/cards/Necromancy.webp",
  },
  unveiling: {
    icon: "UNV",
    label: "Unveiling",
    description: "Highlights (for both players) all tiles that have never been revealed. Lasts until the end of the turn.",
    shortDescription: "Highlights never-revealed tiles.",
    imagePath: "/cards/Unveiling.webp",
  },
  blood_pact: {
    icon: "BLP",
    label: "Blood Pact",
    description: "If you match 3 pairs in a row, you gain +5 points. If you fail before that, you lose 3 points.",
    shortDescription: "Match 3 in a row: +5; fail: -3.",
    imagePath: "/cards/BloodPact.webp",
  },
  leech: {
    icon: "LCH",
    label: "Leech",
    description: "This turn, points you earn from matching are subtracted from the opponent.",
    shortDescription: "Your points drain from opponent.",
    imagePath: "/cards/Leech.webp",
  },
  oblivion: {
    icon: "OBL",
    label: "Oblivion",
    description: "Select a tile. It and its pair are removed from the game. No one gains or loses points.",
    shortDescription: "Remove a tile and its pair.",
    imagePath: "/cards/Oblivion.webp",
  },
  silence: {
    icon: "SIL",
    label: "Silence",
    description: "Pass your turn immediately without revealing a pair.",
    shortDescription: "Pass the turn without revealing a pair.",
    imagePath: "/cards/Silence.webp",
  },
  earth_elemental: {
    icon: "ERT",
    label: "Earth Elemental",
    description: "Highlights (for both players) all Earth element tiles, without revealing the symbol. Lasts until the end of the turn.",
    shortDescription: "Highlights earth tiles.",
    imagePath: "/cards/EarthElemental.webp",
  },
  fire_elemental: {
    icon: "FIR",
    label: "Fire Elemental",
    description: "Highlights (for both players) all Fire element tiles, without revealing the symbol. Lasts until the end of the turn.",
    shortDescription: "Highlights fire tiles.",
    imagePath: "/cards/FireElemental.webp",
  },
  water_elemental: {
    icon: "WAT",
    label: "Water Elemental",
    description: "Highlights (for both players) all Water element tiles, without revealing the symbol. Lasts until the end of the turn.",
    shortDescription: "Highlights water tiles.",
    imagePath: "/cards/WaterElemental.webp",
  },
  air_elemental: {
    icon: "AIR",
    label: "Air Elemental",
    description: "Highlights (for both players) all Air element tiles, without revealing the symbol. Lasts until the end of the turn.",
    shortDescription: "Highlights air tiles.",
    imagePath: "/cards/AirElemental.webp",
  },
};

/**
 * Returns display info for the power-up at the given pair ID.
 * Only uses pairIdToPowerUp from the server; when absent (unexpected state), returns null
 * so the UI can show a placeholder instead of wrong powerup art.
 */
export function getPowerUpDisplayByPairId(
  pairId: number,
  pairIdToPowerUp?: Record<string, string> | null
): PowerUpDisplayInfo | null {
  const powerUpId =
    pairIdToPowerUp != null ? pairIdToPowerUp[String(pairId)] : undefined;
  if (powerUpId == null) return null;
  return POWER_UP_DISPLAY[powerUpId] ?? null;
}

export const NUM_POWERUP_PAIRS = 6;
