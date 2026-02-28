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
import { playSound } from "../lib/sounds";
import { ToastContainer, type ToastItem } from "./Toast";
import styles from "../styles/App.module.css";

const WS_URL = import.meta.env.VITE_WS_URL ?? "ws://localhost:8080/ws";
const NEON_AUTH_URL = import.meta.env.VITE_NEON_AUTH_URL ?? "";
const GAME_OVER_DELAY_MS = 600;

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
  const [userRole, setUserRole] = useState<string>("");
  const [screen, setScreen] = useState<ScreenName>("lobby");
  const [matchInfo, setMatchInfo] = useState<MatchFoundMsg | null>(null);
  const [gameState, setGameState] = useState<GameStateMsg | null>(null);
  const [gameResult, setGameResult] = useState<GameOverMsg | null>(null);
  const [opponentDisconnected, setOpponentDisconnected] = useState(false);
  const [opponentReconnecting, setOpponentReconnecting] = useState<number | null>(null);
  const [toasts, setToasts] = useState<ToastItem[]>([]);
  const [pendingGameOver, setPendingGameOver] = useState<GameOverMsg | null>(null);
  const [powerUpMessage, setPowerUpMessage] = useState<string | null>(null);
  const [authSent, setAuthSent] = useState(false);
  const [showAbandonConfirm, setShowAbandonConfirm] = useState(false);
  const gameOverTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const powerUpMessageTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  /** Index we just flipped (so we don't play tile sound again when server confirms). */
  const lastFlipIndexByUsRef = useRef<number | null>(null);
  /** Key for Board so it remounts when we receive the first game_state (new game or rejoin). */
  const [boardKey, setBoardKey] = useState(0);
  const prevHadGameStateRef = useRef(false);

  const addToast = useCallback((message: string) => {
    setToasts((prev) => [...prev, { id: crypto.randomUUID(), message }]);
  }, []);

  const dismissToast = useCallback((id: string) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, []);

  const handleMessage = useCallback(
    (msg: import("../types/messages").ServerMessage) => {
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
        case "game_state": {
          const prevCards = gameState?.cards ?? [];
          const prevByIndex = new Map(prevCards.map((c) => [c.index, c.state]));
          const newlyRevealed = msg.cards.filter(
            (c) =>
              prevByIndex.get(c.index) === "hidden" &&
              (c.state === "revealed" || c.state === "matched"),
          );
          const newlyHidden = msg.cards.filter(
            (c) =>
              c.state === "hidden" &&
              (prevByIndex.get(c.index) === "revealed" || prevByIndex.get(c.index) === "matched"),
          );
          const ourFlip = lastFlipIndexByUsRef.current;
          const onlyFlipWasOurs =
            newlyRevealed.length === 1 && newlyRevealed[0]?.index === ourFlip;
          const opponentFlipped = newlyRevealed.length > 0 && !onlyFlipWasOurs;
          if (opponentFlipped) {
            playSound("tileFlip");
          }
          if (newlyHidden.length > 0) {
            playSound("tileFlipBack");
          }
          lastFlipIndexByUsRef.current = null;
          setGameState(msg);
          break;
        }
      case "turn_timeout":
        break;
      case "game_over":
        clearGameSession();
        setGameResult(msg);
        setOpponentDisconnected(false);
        setOpponentReconnecting(null);
        setPendingGameOver(msg);
        if (gameOverTimeoutRef.current) clearTimeout(gameOverTimeoutRef.current);
        gameOverTimeoutRef.current = setTimeout(() => {
          gameOverTimeoutRef.current = null;
          setScreen("gameover");
          setPendingGameOver(null);
        }, GAME_OVER_DELAY_MS);
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
      case "powerup_used": {
        const text = msg.noEffect
          ? `${msg.playerName} used ${msg.powerUpLabel} but it had no effect`
          : `${msg.playerName} used ${msg.powerUpLabel}!`;
        setPowerUpMessage(text);
        if (powerUpMessageTimeoutRef.current) clearTimeout(powerUpMessageTimeoutRef.current);
        powerUpMessageTimeoutRef.current = setTimeout(() => {
          powerUpMessageTimeoutRef.current = null;
          setPowerUpMessage(null);
        }, 3000);
        break;
      }
      case "powerup_effect_resolved": {
        setPowerUpMessage(msg.message);
        if (powerUpMessageTimeoutRef.current) clearTimeout(powerUpMessageTimeoutRef.current);
        powerUpMessageTimeoutRef.current = setTimeout(() => {
          powerUpMessageTimeoutRef.current = null;
          setPowerUpMessage(null);
        }, 3000);
        break;
      }
      case "error":
        if (msg.message.includes("Game not found") || msg.message.includes("Invalid rejoin")) {
          clearGameSession();
        }
        // Do not show "No active game" as error; it is normal when user has no game in progress
        if (!msg.message.includes("No active game for this user")) {
          addToast(msg.message);
        }
        break;
      default: {
        const _exhaustiveCheck: never = msg;
        void _exhaustiveCheck;
      }
    }
  }, [addToast, gameState]);

  // Remount Board when we receive the first game_state (new game or rejoin) so it can show already matched/removed tiles as empty.
  useEffect(() => {
    const hasState = gameState != null;
    if (hasState && !prevHadGameStateRef.current) {
      setBoardKey((k) => k + 1);
    }
    prevHadGameStateRef.current = hasState;
  }, [gameState]);

  const { connected, send } = useGameSocket(WS_URL, { onMessage: handleMessage });

  // Load session user (display name and role for admin menu)
  useEffect(() => {
    authClient.getSession().then((result) => {
      const user = result.data?.user;
      if (user) {
        const displayName = getDisplayNameFromUser(user.name, user.email);
        setUserName(displayName);
        playerNameRef.current = displayName;
        const role = (user as { role?: string }).role ?? "";
        setUserRole(role);
      }
    });
  }, []);

  // Send auth token as first message when connected (must complete before Find Game).
  // Read JWT from get-session response header Set-Auth-Jwt (Neon sends it when session exists).
  const authSentRef = useRef(false);
  useEffect(() => {
    if (!connected || authSentRef.current) return;
    if (!NEON_AUTH_URL) {
      addToast("VITE_NEON_AUTH_URL is not set.");
      return;
    }

    let cancelled = false;
    const timeoutId = window.setTimeout(() => {
      if (cancelled || authSentRef.current) return;
      addToast("Auth token request timed out. Try signing out and back in.");
    }, 15000);

    function sendAuthAndReady(jwt: string) {
      if (cancelled || authSentRef.current) return;
      send({ type: "auth", token: jwt });
      authSentRef.current = true;
      setAuthSent(true);
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
          addToast(tokenResult.error.message ?? "Could not get auth token.");
          return;
        }
        addToast("Could not get auth token. Try signing out and back in.");
      })
      .catch((err) => {
        if (cancelled) return;
        const msg = err?.message ?? String(err);
        addToast(`Auth failed: ${msg}. Try signing out and back in.`);
      })
      .finally(() => {
        window.clearTimeout(timeoutId);
      });

    return () => {
      cancelled = true;
      window.clearTimeout(timeoutId);
    };
  }, [connected, send, addToast]);

  // Reset auth sent on disconnect so we re-send on reconnect
  useEffect(() => {
    if (!connected) {
      authSentRef.current = false;
      setAuthSent(false);
    }
  }, [connected]);

  // Clear game-over delay and power-up message timeouts on unmount
  useEffect(() => {
    return () => {
      if (gameOverTimeoutRef.current) {
        clearTimeout(gameOverTimeoutRef.current);
        gameOverTimeoutRef.current = null;
      }
      if (powerUpMessageTimeoutRef.current) {
        clearTimeout(powerUpMessageTimeoutRef.current);
        powerUpMessageTimeoutRef.current = null;
      }
    };
  }, []);

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
    send({ type: "rejoin_my_game" });
  }, [connected, authSent, send, screen]);

  const handleFindMatch = useCallback(() => {
    if (!authSentRef.current) return; // Auth must be sent first; button is disabled until then
    playerNameRef.current = userName;
    send({ type: "set_name", name: userName });
    setScreen("waiting");
  }, [userName, send]);

  const handleFlipCard = useCallback(
    (index: number) => {
      lastFlipIndexByUsRef.current = index;
      playSound("tileFlip");
      send({ type: "flip_card", index });
    },
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
    setGameResult(null);
    setOpponentDisconnected(false);
    clearGameSession();
    send({ type: "play_again" });
    setScreen("waiting");
  }, [send]);

  const handleBackToHome = useCallback(() => {
    setGameResult(null);
    setMatchInfo(null);
    setGameState(null);
    setOpponentDisconnected(false);
    setOpponentReconnecting(null);
    clearGameSession();
    setScreen("lobby");
  }, []);

  const handleCancelMatchmaking = useCallback(() => {
    send({ type: "leave_queue" });
    handleBackToHome();
  }, [send, handleBackToHome]);

  const handleAbandonClick = useCallback(() => {
    setShowAbandonConfirm(true);
  }, []);

  const handleConfirmAbandon = useCallback(() => {
    setShowAbandonConfirm(false);
    clearGameSession();
    send({ type: "leave_game" });
    setMatchInfo(null);
    setGameState(null);
    setOpponentDisconnected(false);
    setOpponentReconnecting(null);
    setScreen("lobby");
  }, [send]);

  const handleCancelAbandon = useCallback(() => {
    setShowAbandonConfirm(false);
  }, []);

  const handleSignOut = useCallback(async () => {
    await authClient.signOut();
    navigate("/auth/sign-in", { replace: true });
  }, [navigate]);

  return (
    <main className={styles.app}>
      <ToastContainer toasts={toasts} onDismiss={dismissToast} />

      {showAbandonConfirm && (
        <div
          className={styles.modalOverlay}
          role="dialog"
          aria-modal="true"
          aria-labelledby="abandon-modal-title"
          aria-describedby="abandon-modal-desc"
          onClick={(e) => {
            if (e.target === e.currentTarget) handleCancelAbandon();
          }}
        >
          <div className={styles.modalPanel} onClick={(e) => e.stopPropagation()}>
            <h2 id="abandon-modal-title" className={styles.modalTitle}>
              Leave game?
            </h2>
            <p id="abandon-modal-desc" className={styles.modalDescription}>
              Abandoning counts as a loss and will lower your Rating. Your opponent will be awarded a win. Are you sure?
            </p>
            <div className={styles.modalActions}>
              <button
                type="button"
                className={styles.modalCancel}
                onClick={handleCancelAbandon}
              >
                Cancel
              </button>
              <button
                type="button"
                className={styles.modalConfirm}
                onClick={handleConfirmAbandon}
              >
                Leave game
              </button>
            </div>
          </div>
        </div>
      )}

      {screen === "lobby" && (
        <LobbyScreen
          firstName={userName}
          connected={connected}
          authReady={authSent}
          isAdmin={userRole === "admin"}
          onFindMatch={handleFindMatch}
          onSignOut={handleSignOut}
        />
      )}
      {screen === "waiting" && (
        <WaitingScreen connected={connected} onCancel={handleCancelMatchmaking} />
      )}
      {(screen === "game" || pendingGameOver !== null) && (
        <GameScreen
          connected={connected}
          matchInfo={matchInfo}
          gameState={gameState}
          boardKey={boardKey}
          opponentReconnectingDeadlineMs={opponentReconnecting}
          powerUpMessage={powerUpMessage}
          onFlipCard={handleFlipCard}
          onUsePowerUp={handleUsePowerUp}
          onAbandon={handleAbandonClick}
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
