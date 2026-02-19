import { useCallback, useEffect, useRef, useState } from "react";
import { useGameSocket } from "./hooks/useGameSocket";
import GameOverScreen from "./screens/GameOverScreen";
import GameScreen from "./screens/GameScreen";
import LobbyScreen from "./screens/LobbyScreen";
import WaitingScreen from "./screens/WaitingScreen";
import type { GameOverMsg, GameStateMsg, MatchFoundMsg } from "./types/messages";
import styles from "./styles/App.module.css";

type ScreenName = "lobby" | "waiting" | "game" | "gameover";

const WS_URL = import.meta.env.VITE_WS_URL ?? "ws://localhost:8080/ws";
const SESSION_STORAGE_KEY = "memory-game-session";

export interface GameSession {
  gameId: string;
  rejoinToken: string;
  playerName: string;
}

function saveGameSession(session: GameSession): void {
  try {
    sessionStorage.setItem(SESSION_STORAGE_KEY, JSON.stringify(session));
  } catch {
    // ignore
  }
}

function clearGameSession(): void {
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

export default function App() {
  const playerNameRef = useRef<string>("");
  const [screen, setScreen] = useState<ScreenName>("lobby");
  const [matchInfo, setMatchInfo] = useState<MatchFoundMsg | null>(null);
  const [gameState, setGameState] = useState<GameStateMsg | null>(null);
  const [gameResult, setGameResult] = useState<GameOverMsg | null>(null);
  const [opponentDisconnected, setOpponentDisconnected] = useState(false);
  const [opponentReconnecting, setOpponentReconnecting] = useState<number | null>(null);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const handleMessage = useCallback((msg: import("./types/messages").ServerMessage) => {
    switch (msg.type) {
      case "waiting_for_match":
        setScreen("waiting");
        setGameResult(null);
        setOpponentDisconnected(false);
        setOpponentReconnecting(null);
        break;
      case "match_found":
        setMatchInfo(msg);
        setScreen("game");
        setGameResult(null);
        setOpponentDisconnected(false);
        setOpponentReconnecting(null);
        if (msg.gameId && msg.rejoinToken) {
          saveGameSession({
            gameId: msg.gameId,
            rejoinToken: msg.rejoinToken,
            playerName: playerNameRef.current || "",
          });
        }
        break;
      case "game_state":
        setGameState(msg);
        break;
      case "turn_timeout":
        /* Turn already switched; countdown message is shown during countdown. */
        break;
      case "game_over":
        clearGameSession();
        setGameResult(msg);
        setOpponentDisconnected(false);
        setOpponentReconnecting(null);
        setScreen("gameover");
        break;
      case "opponent_disconnected":
        clearGameSession();
        setOpponentDisconnected(true);
        setGameResult(null);
        setOpponentReconnecting(null);
        setScreen("gameover");
        break;
      case "opponent_reconnecting":
        setOpponentReconnecting(msg.reconnectionDeadlineUnixMs);
        break;
      case "opponent_reconnected":
        setOpponentReconnecting(null);
        break;
      case "error":
        if (msg.message.includes("Game not found") || msg.message.includes("Invalid rejoin")) {
          clearGameSession();
        }
        setErrorMessage(msg.message);
        break;
      default: {
        const _exhaustiveCheck: never = msg;
        void _exhaustiveCheck;
      }
    }
  }, []);

  const { connected, send } = useGameSocket(WS_URL, { onMessage: handleMessage });

  // When socket connects and we have a saved session (e.g. after refresh), send rejoin only if not already in game
  const prevConnectedRef = useRef(false);
  useEffect(() => {
    if (!connected) {
      prevConnectedRef.current = false;
      return;
    }
    if (prevConnectedRef.current) return;
    const session = getGameSession();
    if (!session?.gameId || !session?.rejoinToken || !session?.playerName) return;
    if (screen !== "lobby" && screen !== "waiting") return;
    prevConnectedRef.current = true;
    send({ type: "rejoin", gameId: session.gameId, rejoinToken: session.rejoinToken, name: session.playerName });
  }, [connected, send, screen]);

  const handleFindMatch = (name: string): void => {
    setErrorMessage(null);
    playerNameRef.current = name;
    send({ type: "set_name", name });
    setScreen("waiting");
  };

  const handleFlipCard = (index: number): void => {
    send({ type: "flip_card", index });
  };

  const handleUsePowerUp = (powerUpId: string, cardIndex?: number): void => {
    const msg: { type: "use_power_up"; powerUpId: string; cardIndex?: number } = {
      type: "use_power_up",
      powerUpId,
    };
    if (cardIndex !== undefined) {
      msg.cardIndex = cardIndex;
    }
    send(msg);
  };

  const handlePlayAgain = (): void => {
    setErrorMessage(null);
    setGameResult(null);
    setOpponentDisconnected(false);
    clearGameSession();
    send({ type: "play_again" });
    setScreen("waiting");
  };

  return (
    <main className={styles.app}>
      {errorMessage && <p className={styles.error}>Error: {errorMessage}</p>}

      {screen === "lobby" && <LobbyScreen connected={connected} onFindMatch={handleFindMatch} />}
      {screen === "waiting" && <WaitingScreen connected={connected} />}
      {screen === "game" && (
        <GameScreen
          connected={connected}
          matchInfo={matchInfo}
          gameState={gameState}
          opponentReconnectingDeadlineMs={opponentReconnecting}
          onFlipCard={handleFlipCard}
          onUsePowerUp={handleUsePowerUp}
        />
      )}
      {screen === "gameover" && (
        <GameOverScreen
          connected={connected}
          result={gameResult}
          opponentDisconnected={opponentDisconnected}
          latestGameState={gameState}
          onPlayAgain={handlePlayAgain}
        />
      )}
    </main>
  );
}
