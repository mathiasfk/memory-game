const SESSION_STORAGE_KEY = "memory-game-session";

export interface GameSession {
  gameId: string;
  rejoinToken: string;
  playerName: string;
}

export function saveGameSession(session: GameSession): void {
  try {
    sessionStorage.setItem(SESSION_STORAGE_KEY, JSON.stringify(session));
  } catch {
    // ignore
  }
}

export function clearGameSession(): void {
  try {
    sessionStorage.removeItem(SESSION_STORAGE_KEY);
  } catch {
    // ignore
  }
}

export function getGameSession(): GameSession | null {
  try {
    const raw = sessionStorage.getItem(SESSION_STORAGE_KEY);
    if (!raw) return null;
    const parsed = JSON.parse(raw) as unknown;
    if (
      parsed &&
      typeof parsed === "object" &&
      typeof (parsed as GameSession).gameId === "string" &&
      typeof (parsed as GameSession).rejoinToken === "string" &&
      typeof (parsed as GameSession).playerName === "string"
    ) {
      return parsed as GameSession;
    }
    return null;
  } catch {
    return null;
  }
}
