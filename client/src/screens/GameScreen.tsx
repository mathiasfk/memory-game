import { useCallback, useEffect, useRef, useState } from "react";
import Board from "../components/Board";
import ComboIndicator from "../components/ComboIndicator";
import PowerUpShop from "../components/PowerUpShop";
import ScorePanel from "../components/ScorePanel";
import TurnIndicator from "../components/TurnIndicator";
import type { GameStateMsg, MatchFoundMsg } from "../types/messages";
import styles from "../styles/GameScreen.module.css";
import countdownStyles from "../styles/TurnCountdown.module.css";

interface GameScreenProps {
  connected: boolean;
  matchInfo: MatchFoundMsg | null;
  gameState: GameStateMsg | null;
  opponentReconnectingDeadlineMs: number | null;
  onFlipCard: (index: number) => void;
  onUsePowerUp: (powerUpId: string, cardIndex?: number) => void;
  onAbandon: () => void;
}

function getSecondsRemaining(turnEndsAtUnixMs: number): number {
  return Math.max(0, Math.ceil((turnEndsAtUnixMs - Date.now()) / 1000));
}

function getReconnectSecondsRemaining(deadlineUnixMs: number): number {
  return Math.max(0, Math.ceil((deadlineUnixMs - Date.now()) / 1000));
}

function formatReconnectCountdown(seconds: number): string {
  const m = Math.floor(seconds / 60);
  const s = seconds % 60;
  return `${m}:${s.toString().padStart(2, "0")}`;
}

export default function GameScreen({
  connected,
  matchInfo,
  gameState,
  opponentReconnectingDeadlineMs,
  onFlipCard,
  onUsePowerUp,
  onAbandon,
}: GameScreenProps) {
  const [pendingRadarTarget, setPendingRadarTarget] = useState(false);
  const [secondsRemaining, setSecondsRemaining] = useState<number | null>(null);
  const [reconnectSecondsRemaining, setReconnectSecondsRemaining] = useState<number | null>(null);
  const [menuOpen, setMenuOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  const closeMenu = useCallback(() => setMenuOpen(false), []);

  useEffect(() => {
    if (!menuOpen) return;
    const handleClickOutside = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        closeMenu();
      }
    };
    document.addEventListener("click", handleClickOutside);
    return () => document.removeEventListener("click", handleClickOutside);
  }, [menuOpen, closeMenu]);

  useEffect(() => {
    if (gameState && (!gameState.yourTurn || gameState.phase !== "first_flip")) {
      setPendingRadarTarget(false);
    }
  }, [gameState?.yourTurn, gameState?.phase]);

  // Update countdown every second when it's our turn and we have a turn deadline
  useEffect(() => {
    if (
      !gameState?.yourTurn ||
      gameState.turnEndsAtUnixMs == null ||
      gameState.turnEndsAtUnixMs <= 0
    ) {
      setSecondsRemaining(null);
      return;
    }
    const update = (): void => {
      setSecondsRemaining(getSecondsRemaining(gameState.turnEndsAtUnixMs!));
    };
    update();
    const interval = setInterval(update, 1000);
    return () => clearInterval(interval);
  }, [gameState?.yourTurn, gameState?.turnEndsAtUnixMs]);

  // Update reconnection countdown every second when opponent is reconnecting
  useEffect(() => {
    if (opponentReconnectingDeadlineMs == null || opponentReconnectingDeadlineMs <= 0) {
      setReconnectSecondsRemaining(null);
      return;
    }
    const update = (): void => {
      const sec = getReconnectSecondsRemaining(opponentReconnectingDeadlineMs);
      setReconnectSecondsRemaining(sec);
    };
    update();
    const interval = setInterval(update, 1000);
    return () => clearInterval(interval);
  }, [opponentReconnectingDeadlineMs]);

  if (!matchInfo) {
    return (
      <section className={styles.screen}>
        <p>Waiting for match details...</p>
      </section>
    );
  }

  if (!gameState) {
    return (
      <section className={styles.screen}>
        <p>Match found! Waiting for initial board state...</p>
      </section>
    );
  }

  const cardsClickable = connected && gameState.yourTurn && gameState.phase !== "resolve";
  const powerUpsEnabled = connected && gameState.yourTurn && gameState.phase === "first_flip";

  const handleCardClick = (index: number): void => {
    if (pendingRadarTarget) {
      onUsePowerUp("radar", index);
      setPendingRadarTarget(false);
    } else {
      onFlipCard(index);
    }
  };

  const handleUsePowerUpClick = (powerUpId: string): void => {
    if (powerUpId === "radar") {
      setPendingRadarTarget(true);
    } else {
      onUsePowerUp(powerUpId);
    }
  };

  const showCountdown =
    gameState.yourTurn &&
    secondsRemaining !== null &&
    gameState.turnCountdownShowSec != null &&
    secondsRemaining <= gameState.turnCountdownShowSec;

  const showReconnectBanner =
    opponentReconnectingDeadlineMs != null &&
    reconnectSecondsRemaining !== null &&
    reconnectSecondsRemaining >= 0;

  return (
    <section className={styles.screen}>
      {showReconnectBanner && (
        <div className={styles.reconnectBanner} role="alert">
          <span>Opponent lost connection. Waiting for them to rejoin… </span>
          <span className={styles.reconnectCountdown}>
            ({formatReconnectCountdown(reconnectSecondsRemaining)})
          </span>
        </div>
      )}
      <header className={styles.header}>
        <div className={styles.titleRow} ref={menuRef}>
          <h2>You vs {matchInfo.opponentName}</h2>
          <div className={styles.menuWrap}>
            <button
              type="button"
              onClick={() => setMenuOpen((o) => !o)}
              className={styles.kebabTrigger}
              title="Game options"
              aria-expanded={menuOpen}
              aria-haspopup="menu"
              aria-label="Open game menu"
            >
              <span className={styles.kebabDots} aria-hidden>⋮</span>
            </button>
            {menuOpen && (
              <div className={styles.contextMenu} role="menu">
                <button
                  type="button"
                  role="menuitem"
                  className={styles.contextMenuItem}
                  onClick={() => {
                    closeMenu();
                    onAbandon();
                  }}
                  title="Leave game and return to lobby"
                >
                  Leave game
                </button>
              </div>
            )}
          </div>
        </div>
        <div className={styles.countdownRow}>
          {showCountdown && (
            <div className={countdownStyles.countdownWrap} aria-live="polite" aria-atomic="true">
              <span className={countdownStyles.countdownLabel}>Time is almost up!</span>
              <span
                className={countdownStyles.countdown}
                key={secondsRemaining}
              >
                {secondsRemaining}s
              </span>
            </div>
          )}
        </div>
        <div className={styles.turnIndicatorWrap}>
          <TurnIndicator yourTurn={gameState.yourTurn} phase={gameState.phase} />
        </div>
      </header>

      <div className={styles.main}>
        <div className={styles.leftColumn}>
          <Board
            cards={gameState.cards}
            rows={matchInfo.boardRows}
            cols={matchInfo.boardCols}
            cardsClickable={cardsClickable}
            onCardClick={handleCardClick}
            radarTargetingMode={pendingRadarTarget}
          />
        </div>

        <aside className={styles.rightColumn}>
          <ScorePanel
            you={gameState.you}
            opponent={gameState.opponent}
            yourTurn={gameState.yourTurn}
          />
          <ComboIndicator comboStreak={gameState.you.comboStreak} label="Combo" />
          <PowerUpShop
            powerUps={gameState.availablePowerUps}
            enabled={powerUpsEnabled}
            onUsePowerUp={handleUsePowerUpClick}
            secondChanceRoundsRemaining={gameState.you.secondChanceRoundsRemaining}
          />
        </aside>
      </div>
    </section>
  );
}
