import { POWER_UP_DISPLAY } from "../powerups/registry";

/** Card back image used on every card. */
const CARD_BACK_PATH = "/cards/Verse.webp";

/**
 * Returns all card art image paths used in the game (power-ups + card back).
 */
export function getCardImagePaths(): string[] {
  const powerUpPaths = Object.values(POWER_UP_DISPLAY).map((d) => d.imagePath);
  return [...new Set([...powerUpPaths, CARD_BACK_PATH])];
}

/**
 * Pre-fetches all card images so they are in the browser cache before the user
 * flips cards. Call once on app load (e.g. in main.tsx).
 */
export function prefetchCardImages(): void {
  const paths = getCardImagePaths();
  paths.forEach((path) => {
    const img = new Image();
    img.src = path;
  });
}
