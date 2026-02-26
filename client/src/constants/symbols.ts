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
const NUM_POWERUPS = 6;

export function getNormalSymbolForPairId(pairId: number): NormalCardSymbol {
  const len = NORMAL_CARD_SYMBOLS.length;
  const index = ((pairId - NUM_POWERUPS) % len + len) % len;
  return NORMAL_CARD_SYMBOLS[index]!;
}
