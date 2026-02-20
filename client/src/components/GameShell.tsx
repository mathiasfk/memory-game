import { useCallback, useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { authClient } from "../lib/auth";
import { useGameSocket } from "../hooks/useGameSocket";
import GameOverScreen from "../screens/GameOverScreen";
import GameScreen from "../screens/GameScreen";
import LobbyScreen from "../screens/LobbyScreen";
import WaitingScreen from "../screens/WaitingScreen";
import type { GameOverMsg, GameStateMsg, MatchFoundMsg } from "../types/messages";
import { getGameSession, clearGameSession, saveGameSession } from "../lib/gameSession";
import styles from "../styles/App.module.css";

const WS_URL = import.meta.env.VITE_WS_URL ?? "ws://localhost:8080/ws";
const NEON_AUTH_URL = import.meta.env.VITE_NEON_AUTH_URL ?? "";

type ScreenName = "lobby" | "waiting" | "game" | "gameover";

function getDisplayNameFromUser(name: string | null | undefined, email?: string | null): string {
  const trimmed = (name ?? "").trim();
  if (trimmed.length > 0) {
    const first = trimmed.split(/\s+/)[0];
    if (first) return first;
  }
  if (email && email.trim().length > 0) return email.trim();
  return "Player";
}

export function GameShell() {
  const navigate = useNavigate();
  const playerNameRef = useRef<string>("");
  const [userName, setUserName] = useState<string>("Player");
  const [screen, setScreen] = useState<ScreenName>("lobby");
  const [matchInfo, setMatchInfo] = useState<MatchFoundMsg | null>(null);
  const [gameState, setGameState] = useState<GameStateMsg | null>(null);
  const [gameResult, setGameResult] = useState<GameOverMsg | null>(null);
  const [opponentDisconnected, setOpponentDisconnected] = useState(false);
  const [opponentReconnecting, setOpponentReconnecting] = useState<number | null>(null);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [authSent, setAuthSent] = useState(false);

  const handleMessage = useCallback((msg: import("../types/messages").ServerMessage) => {
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
        // Do not show "No active game" as error; it is normal when user has no game in progress
        if (!msg.message.includes("No active game for this user")) {
          setErrorMessage(msg.message);
        }
        break;
      default: {
        const _exhaustiveCheck: never = msg;
        void _exhaustiveCheck;
      }
    }
  }, []);

  const { connected, send } = useGameSocket(WS_URL, { onMessage: handleMessage });

  // Load session user and derive display name
  useEffect(() => {
    authClient.getSession().then((result) => {
      const user = result.data?.user;
      if (user) {
        const displayName = getDisplayNameFromUser(user.name, user.email);
        setUserName(displayName);
        playerNameRef.current = displayName;
      }
    });
  }, []);

  // Send auth token as first message when connected (must complete before Find Game).
  // Read JWT from get-session response header Set-Auth-Jwt (Neon sends it when session exists).
  const authSentRef = useRef(false);
  useEffect(() => {
    if (!connected || authSentRef.current) return;
    if (!NEON_AUTH_URL) {
      setErrorMessage("VITE_NEON_AUTH_URL is not set.");
      return;
    }

    let cancelled = false;
    const timeoutId = window.setTimeout(() => {
      if (cancelled || authSentRef.current) return;
      setErrorMessage("Auth token request timed out. Try signing out and back in.");
    }, 15000);

    function sendAuthAndReady(jwt: string) {
      if (cancelled || authSentRef.current) return;
      send({ type: "auth", token: jwt });
      authSentRef.current = true;
      setAuthSent(true);
      setErrorMessage(null); // Clear any prior error so rejoin can show clean state
    }

    const getSessionUrl = `${NEON_AUTH_URL.replace(/\/$/, "")}/get-session`;

    fetch(getSessionUrl, { credentials: "include" })
      .then((res) => {
        if (cancelled || authSentRef.current) return;
        // Header name is case-insensitive; Neon uses Set-Auth-Jwt
        const jwt =
          res.headers.get("set-auth-jwt") ?? res.headers.get("Set-Auth-Jwt");
        if (jwt) {
          sendAuthAndReady(jwt);
          return;
        }
        return authClient.token();
      })
      .then((tokenResult) => {
        if (cancelled || authSentRef.current) return;
        if (authSentRef.current) return;
        if (tokenResult?.data?.token) {
          sendAuthAndReady(tokenResult.data.token);
          return;
        }
        if (tokenResult?.error) {
          setErrorMessage(tokenResult.error.message ?? "Could not get auth token.");
          return;
        }
        setErrorMessage("Could not get auth token. Try signing out and back in.");
      })
      .catch((err) => {
        if (cancelled) return;
        const msg = err?.message ?? String(err);
        setErrorMessage(`Auth failed: ${msg}. Try signing out and back in.`);
      })
      .finally(() => {
        window.clearTimeout(timeoutId);
      });

    return () => {
      cancelled = true;
      window.clearTimeout(timeoutId);
    };
  }, [connected, send]);

  // Reset auth sent on disconnect so we re-send on reconnect
  useEffect(() => {
    if (!connected) {
      authSentRef.current = false;
      setAuthSent(false);
    }
  }, [connected]);

  // Rejoin saved game when socket reconnects (only after auth was sent to avoid "Authentication required").
  // With session (same device): send rejoin. Without session (e.g. other device): send rejoin_my_game once.
  const prevConnectedRef = useRef(false);
  const rejoinMyGameSentRef = useRef(false);
  useEffect(() => {
    if (!connected) {
      prevConnectedRef.current = false;
      rejoinMyGameSentRef.current = false;
      return;
    }
    if (!authSent) return; // Wait for auth first so server does not receive rejoin before auth
    if (screen !== "lobby" && screen !== "waiting") return;
    const session = getGameSession();
    if (session?.gameId && session?.rejoinToken && session?.playerName) {
      if (prevConnectedRef.current) return;
      prevConnectedRef.current = true;
      setErrorMessage(null);
      send({
        type: "rejoin",
        gameId: session.gameId,
        rejoinToken: session.rejoinToken,
        name: session.playerName,
      });
      return;
    }
    // No local session: try rejoin by user (cross-device, same account)
    if (rejoinMyGameSentRef.current) return;
    rejoinMyGameSentRef.current = true;
    setErrorMessage(null);
    send({ type: "rejoin_my_game" });
  }, [connected, authSent, send, screen]);

  const handleFindMatch = useCallback(() => {
    if (!authSentRef.current) return; // Auth must be sent first; button is disabled until then
    setErrorMessage(null);
    playerNameRef.current = userName;
    send({ type: "set_name", name: userName });
    setScreen("waiting");
  }, [userName, send]);

  const handleFlipCard = useCallback(
    (index: number) => send({ type: "flip_card", index }),
    [send],
  );

  const handleUsePowerUp = useCallback(
    (powerUpId: string, cardIndex?: number) => {
      const msg: { type: "use_power_up"; powerUpId: string; cardIndex?: number } = {
        type: "use_power_up",
        powerUpId,
      };
      if (cardIndex !== undefined) msg.cardIndex = cardIndex;
      send(msg);
    },
    [send],
  );

  const handlePlayAgain = useCallback(() => {
    setErrorMessage(null);
    setGameResult(null);
    setOpponentDisconnected(false);
    clearGameSession();
    send({ type: "play_again" });
    setScreen("waiting");
  }, [send]);

  const handleBackToHome = useCallback(() => {
    setErrorMessage(null);
    setGameResult(null);
    setMatchInfo(null);
    setGameState(null);
    setOpponentDisconnected(false);
    setOpponentReconnecting(null);
    clearGameSession();
    setScreen("lobby");
  }, []);

  const handleSignOut = useCallback(async () => {
    await authClient.signOut();
    navigate("/auth/sign-in", { replace: true });
  }, [navigate]);

  return (
    <main className={styles.app}>
      {errorMessage && <p className={styles.error}>Error: {errorMessage}</p>}

      {screen === "lobby" && (
        <LobbyScreen
          firstName={userName}
          connected={connected}
          authReady={authSent}
          onFindMatch={handleFindMatch}
          onSignOut={handleSignOut}
        />
      )}
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
          onBackToHome={handleBackToHome}
        />
      )}
    </main>
  );
}
