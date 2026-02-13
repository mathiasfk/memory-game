import { useEffect, useState } from "react";
import { useGameSocket } from "./hooks/useGameSocket";
import GameOverScreen from "./screens/GameOverScreen";
import GameScreen from "./screens/GameScreen";
import LobbyScreen from "./screens/LobbyScreen";
import WaitingScreen from "./screens/WaitingScreen";
import type { GameOverMsg, GameStateMsg, MatchFoundMsg } from "./types/messages";
import styles from "./styles/App.module.css";

type ScreenName = "lobby" | "waiting" | "game" | "gameover";

const WS_URL = import.meta.env.VITE_WS_URL ?? "ws://localhost:8080/ws";

export default function App(): JSX.Element {
  const { connected, send, lastMessage } = useGameSocket(WS_URL);

  const [screen, setScreen] = useState<ScreenName>("lobby");
  const [matchInfo, setMatchInfo] = useState<MatchFoundMsg | null>(null);
  const [gameState, setGameState] = useState<GameStateMsg | null>(null);
  const [gameResult, setGameResult] = useState<GameOverMsg | null>(null);
  const [opponentDisconnected, setOpponentDisconnected] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  useEffect(() => {
    if (!lastMessage) {
      return;
    }

    switch (lastMessage.type) {
      case "waiting_for_match":
        setScreen("waiting");
        setGameResult(null);
        setOpponentDisconnected(false);
        break;
      case "match_found":
        setMatchInfo(lastMessage);
        setScreen("game");
        setGameResult(null);
        setOpponentDisconnected(false);
        break;
      case "game_state":
        setGameState(lastMessage);
        break;
      case "game_over":
        setGameResult(lastMessage);
        setOpponentDisconnected(false);
        setScreen("gameover");
        break;
      case "opponent_disconnected":
        setOpponentDisconnected(true);
        setGameResult(null);
        setScreen("gameover");
        break;
      case "error":
        setErrorMessage(lastMessage.message);
        break;
      default: {
        const _exhaustiveCheck: never = lastMessage;
        void _exhaustiveCheck;
      }
    }
  }, [lastMessage]);

  const handleFindMatch = (name: string): void => {
    setErrorMessage(null);
    send({ type: "set_name", name });
    setScreen("waiting");
  };

  const handleFlipCard = (index: number): void => {
    send({ type: "flip_card", index });
  };

  const handleUsePowerUp = (powerUpId: string): void => {
    send({ type: "use_power_up", powerUpId });
  };

  const handlePlayAgain = (): void => {
    setErrorMessage(null);
    setGameResult(null);
    setOpponentDisconnected(false);
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
