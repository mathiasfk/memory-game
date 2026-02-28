/**
 * Symbols and colors for normal (non-power-up) card faces.
 * Four-element palette: fire, water, air, earth.
 */

export const SYMBOL_COLORS = {
  fire: "#c2410c",
  water: "#1d4ed8",
  air: "#94a3b8",
  earth: "#15803d",
} as const;

/** Symbol and color for a single normal (element) card face. */
export interface NormalCardSymbol {
  symbol: string;
  color: (typeof SYMBOL_COLORS)[keyof typeof SYMBOL_COLORS];
}

/** 12 alchemical/astronomical symbols with element-based colors. */
export const NORMAL_CARD_SYMBOLS: NormalCardSymbol[] = [
  // 🔥 FIRE (3)
  { symbol: "\u{1F702}", color: SYMBOL_COLORS.fire },   // 🜂 fire
  { symbol: "\u2609", color: SYMBOL_COLORS.fire },      // ☉ sun
  { symbol: "\u16B4", color: SYMBOL_COLORS.fire },      // ᚴ Kaun

  // 💧 WATER (3)
  { symbol: "\u{1F704}", color: SYMBOL_COLORS.water },  // 🜄 water
  { symbol: "\u263D", color: SYMBOL_COLORS.water },     // ☽ moon
  { symbol: "\u16D7", color: SYMBOL_COLORS.water },     // ᛗ Mannaz

  // 🌬 AIR (3)
  { symbol: "\u{1F701}", color: SYMBOL_COLORS.air },    // 🜁 air
  { symbol: "\u263F", color: SYMBOL_COLORS.air },       // ☿ mercury
  { symbol: "\u16DF", color: SYMBOL_COLORS.air },       // ᛟ Othala

  // 🌍 EARTH (3)
  { symbol: "\u{1F703}", color: SYMBOL_COLORS.earth },  // 🜃 earth
  { symbol: "\u2644", color: SYMBOL_COLORS.earth },     // ♄ saturn
  { symbol: "\u16B7", color: SYMBOL_COLORS.earth },     // ᚷ Gyfu
];

/** Default when server does not send arcanaPairs (backward compat). Must match server ArcanaPairsPerMatch. */
const DEFAULT_ARCANA_PAIRS = 6;

const ELEMENT_TO_OFFSET: Record<string, number> = {
  fire: 0,
  water: 3,
  air: 6,
  earth: 9,
};

/**
 * Symbol and color for a normal card using the server's element (source of truth).
 * variantIndex = 0, 1, or 2 (which of the 3 pairs within that element); e.g. (pairId - arcanaPairs) % 3.
 */
export function getNormalSymbolByElement(
  element: string,
  variantIndex: number
): NormalCardSymbol {
  const offset = ELEMENT_TO_OFFSET[element] ?? 0;
  const index = offset + ((variantIndex % 3 + 3) % 3);
  return NORMAL_CARD_SYMBOLS[index]!;
}

/**
 * Symbol and color for a normal card by pairId. Fallback when server does not send element (backward compat).
 * Uses arcanaPairs from server so the derived element index stays in sync.
 */
export function getNormalSymbolForPairId(
  pairId: number,
  arcanaPairs: number = DEFAULT_ARCANA_PAIRS
): NormalCardSymbol {
  const len = NORMAL_CARD_SYMBOLS.length;
  const index = ((pairId - arcanaPairs) % len + len) % len;
  return NORMAL_CARD_SYMBOLS[index]!;
}
