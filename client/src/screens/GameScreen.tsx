import { useCallback, useEffect, useRef, useState } from "react";
import Board from "../components/Board";
import PowerUpHand from "../components/PowerUpHand";
import type { GameStateMsg, MatchFoundMsg } from "../types/messages";
import styles from "../styles/GameScreen.module.css";
import countdownStyles from "../styles/TurnCountdown.module.css";

interface GameScreenProps {
  connected: boolean;
  matchInfo: MatchFoundMsg | null;
  gameState: GameStateMsg | null;
  /** Key so Board remounts on first game_state (new game or rejoin) and can show already matched/removed tiles as empty. */
  boardKey: number;
  /** Temporary message when a power-up is used (e.g. "Thalia used Leech!"). */
  powerUpMessage?: string | null;
  onFlipCard: (index: number) => void;
  onUsePowerUp: (powerUpId: string, cardIndex?: number) => void;
  onAbandon: () => void;
}

function getSecondsRemaining(turnEndsAtUnixMs: number): number {
  return Math.max(0, Math.ceil((turnEndsAtUnixMs - Date.now()) / 1000));
}

export default function GameScreen({
  connected,
  matchInfo,
  gameState,
  boardKey,
  powerUpMessage = null,
  onFlipCard,
  onUsePowerUp,
  onAbandon,
}: GameScreenProps) {
  const [pendingClairvoyanceTarget, setPendingClairvoyanceTarget] = useState(false);
  const [pendingOblivionTarget, setPendingOblivionTarget] = useState(false);
  const [secondsRemaining, setSecondsRemaining] = useState<number | null>(null);
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
      setPendingClairvoyanceTarget(false);
      setPendingOblivionTarget(false);
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

  const opponentIsBot =
    matchInfo.opponentUserId === "ai" || (matchInfo.opponentUserId?.startsWith("ai:") ?? false);
  const cardsClickable = connected && gameState.yourTurn && gameState.phase !== "resolve";
  const handUseEnabled = connected && gameState.yourTurn && gameState.phase === "first_flip";

  const handleCardClick = (index: number): void => {
    if (pendingClairvoyanceTarget) {
      onUsePowerUp("clairvoyance", index);
      setPendingClairvoyanceTarget(false);
    } else if (pendingOblivionTarget) {
      onUsePowerUp("oblivion", index);
      setPendingOblivionTarget(false);
    } else {
      onFlipCard(index);
    }
  };

  const handleUsePowerUpClick = (powerUpId: string): void => {
    if (powerUpId === "clairvoyance") {
      setPendingClairvoyanceTarget(true);
    } else if (powerUpId === "oblivion") {
      setPendingOblivionTarget(true);
    } else {
      onUsePowerUp(powerUpId);
    }
  };

  const showCountdown =
    gameState.yourTurn &&
    secondsRemaining !== null &&
    gameState.turnCountdownShowSec != null &&
    secondsRemaining <= gameState.turnCountdownShowSec;

  return (
    <section className={styles.screen}>
      <header className={styles.header} ref={menuRef}>
        <div className={styles.headerCenter}>
          <div
            className={`${styles.playerBlock} ${gameState.yourTurn ? styles.playerBlockActive : ""}`}
            aria-current={gameState.yourTurn ? "true" : undefined}
          >
            <span className={styles.playerName}>{gameState.you.name}</span>
            <span className={styles.playerScore}>{gameState.you.score}</span>
          </div>
          <span className={styles.vs}>vs</span>
          <div
            className={`${styles.playerBlock} ${!gameState.yourTurn ? styles.playerBlockOpponentActive : ""}`}
            aria-current={!gameState.yourTurn ? "true" : undefined}
          >
            <span className={styles.playerScore}>{gameState.opponent.score}</span>
            <span className={styles.playerName}>{gameState.opponent.name}</span>
            {opponentIsBot && <span className={styles.botTag}>Bot</span>}
          </div>
        </div>
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
      </header>

      <div className={styles.contextualRow}>
        {powerUpMessage && (
          <p className={styles.powerUpMessage} role="status" aria-live="polite">
            {powerUpMessage}
          </p>
        )}
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
      </div>

      <div className={styles.main}>
        <div className={styles.leftColumn}>
          <div className={styles.boardWrapper}>
            <Board
            key={boardKey}
            cards={gameState.cards}
            initialRemovedIndices={gameState.cards
              .filter((c) => c.state === "matched" || c.state === "removed")
              .map((c) => c.index)}
            rows={matchInfo.boardRows}
            cols={matchInfo.boardCols}
            cardsClickable={cardsClickable}
            onCardClick={handleCardClick}
            radarTargetingMode={pendingClairvoyanceTarget}
            oblivionTargetingMode={pendingOblivionTarget}
            pairIdToPowerUp={gameState.pairIdToPowerUp ?? null}
            arcanaPairs={gameState.arcanaPairs}
            highlightIndices={gameState.highlightIndices ?? []}
          />
          </div>
        </div>
      </div>

      <div className={styles.handWrap}>
        <PowerUpHand
          hand={gameState.hand}
          enabled={handUseEnabled}
          onUsePowerUp={handleUsePowerUpClick}
        />
      </div>
    </section>
  );
}
