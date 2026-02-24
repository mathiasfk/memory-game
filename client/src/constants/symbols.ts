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

export interface NormalCardSymbol {
  symbol: string;
  color: string;
}

/** 12 alchemical/astronomical symbols with element-based colors. */
export const NORMAL_CARD_SYMBOLS: NormalCardSymbol[] = [
  { symbol: "\u{1F702}", color: SYMBOL_COLORS.fire },   // 🜂 fire
  { symbol: "\u{1F704}", color: SYMBOL_COLORS.water },  // 🜄 water
  { symbol: "\u{1F701}", color: SYMBOL_COLORS.air },    // 🜁 air
  { symbol: "\u{1F703}", color: SYMBOL_COLORS.earth },  // 🜃 earth
  { symbol: "\u{1F70D}", color: SYMBOL_COLORS.fire },   // 🜍 sulfur
  { symbol: "\u{1F714}", color: SYMBOL_COLORS.earth },  // 🜔 salt
  { symbol: "\u{1F713}", color: SYMBOL_COLORS.water },  // 🜓 mercury (alch)
  { symbol: "\u2609", color: SYMBOL_COLORS.fire },      // ☉ sun
  { symbol: "\u263D", color: SYMBOL_COLORS.water },     // ☽ moon
  { symbol: "\u263F", color: SYMBOL_COLORS.air },        // ☿ mercury (planet)
  { symbol: "\u2644", color: SYMBOL_COLORS.earth },      // ♄ saturn
  { symbol: "\u2643", color: SYMBOL_COLORS.air },        // ♃ jupiter
];

const NUM_POWERUPS = 4;

export function getNormalSymbolForPairId(pairId: number): NormalCardSymbol {
  const len = NORMAL_CARD_SYMBOLS.length;
  const index = ((pairId - NUM_POWERUPS) % len + len) % len;
  return NORMAL_CARD_SYMBOLS[index]!;
}
