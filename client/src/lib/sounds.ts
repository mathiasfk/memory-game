/**
 * Sound effects registry and playback.
 * Add new events in SOUND_REGISTRY and call playSound() from handlers/screens.
 */

export type SoundEvent = "tileFlip";

interface SoundEntry {
  path: string;
  volume?: number;
}

const SOUND_REGISTRY: Record<SoundEvent, SoundEntry> = {
  tileFlip: {
    path: "/sounds/tile-flip__484965.mp3",
  },
};

export function playSound(event: SoundEvent): void {
  const entry = SOUND_REGISTRY[event];
  if (!entry) return;

  const audio = new Audio(entry.path);
  if (entry.volume != null) {
    audio.volume = Math.max(0, Math.min(1, entry.volume));
  }
  audio.play().catch(() => {
    // Ignore autoplay policy or load errors (e.g. user hasn't interacted yet).
  });
}
