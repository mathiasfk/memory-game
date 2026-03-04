import { useEffect, useState } from "react";
import type { GameOverMsg, GameStateMsg } from "../types/messages";
import styles from "../styles/GameOverScreen.module.css";

const RATING_HOLD_MS = 1000;
const RATING_ANIMATION_DURATION_MS = 1200;

/** Ease-out cubic: 1 - (1 - t)^3 */
function easeOutCubic(t: number): number {
  return 1 - (1 - t) ** 3;
}

interface GameOverScreenProps {
  connected: boolean;
  result: GameOverMsg | null;
  opponentDisconnected: boolean;
  latestGameState: GameStateMsg | null;
  onPlayAgain: () => void;
  onBackToHome: () => void;
}

function resultLabel(result: GameOverMsg["result"]): string {
  switch (result) {
    case "win":
      return "You Win!";
    case "lose":
      return "You Lose";
    default:
      return "Draw";
  }
}

export default function GameOverScreen({
  connected,
  result,
  opponentDisconnected,
  latestGameState,
  onPlayAgain,
  onBackToHome,
}: GameOverScreenProps) {
  const yourScore = result?.you.score ?? latestGameState?.you.score ?? 0;
  const opponentScore = result?.opponent.score ?? latestGameState?.opponent.score ?? 0;
  const yourName = result?.you.name ?? latestGameState?.you.name ?? "You";
  const opponentName = result?.opponent.name ?? latestGameState?.opponent.name ?? "Opponent";

  const eloBefore = result?.you_elo_before;
  const eloAfter = result?.you_elo_after;
  const showRating =
    result != null &&
    eloBefore != null &&
    eloAfter != null &&
    !opponentDisconnected;

  const [displayRating, setDisplayRating] = useState(
    showRating ? eloBefore : eloAfter ?? eloBefore ?? 0
  );
  const [defeatOverlayOpacity, setDefeatOverlayOpacity] = useState(0);

  const isDefeat = result?.result === "lose";

  useEffect(() => {
    if (!showRating || eloBefore == null || eloAfter == null) return;
    setDisplayRating(eloBefore);
    setDefeatOverlayOpacity(0);
    let cancelled = false;
    const start = performance.now();
    const totalDuration = RATING_HOLD_MS + RATING_ANIMATION_DURATION_MS;

    const tick = (now: number) => {
      if (cancelled) return;
      const elapsed = now - start;
      if (elapsed >= totalDuration) {
        setDisplayRating(eloAfter);
        setDefeatOverlayOpacity(isDefeat ? 1 : 0);
        return;
      }

      if (elapsed < RATING_HOLD_MS) {
        setDisplayRating(eloBefore);
        setDefeatOverlayOpacity(0);
      } else {
        const animProgress = (elapsed - RATING_HOLD_MS) / RATING_ANIMATION_DURATION_MS;
        const eased = easeOutCubic(animProgress);
        const value = Math.round(eloBefore + (eloAfter - eloBefore) * eased);
        setDisplayRating(value);
        setDefeatOverlayOpacity(isDefeat ? eased : 0);
      }
      requestAnimationFrame(tick);
    };
    requestAnimationFrame(tick);
    return () => {
      cancelled = true;
    };
  }, [showRating, eloBefore, eloAfter, isDefeat]);

  return (
    <section className={styles.screen}>
      <h2 className={styles.title}>
        {opponentDisconnected || result === null
          ? "Opponent disconnected"
          : resultLabel(result.result)}
      </h2>

      <div className={styles.scores}>
        <p>
          {yourName}: <strong>{yourScore}</strong>
        </p>
        <p>
          {opponentName}: <strong>{opponentScore}</strong>
        </p>
      </div>

      {showRating && (
        <div className={styles.ratingBlock}>
          <span className={styles.ratingLabel}>Rating</span>
          <div className={styles.ratingValueWrap}>
            <span className={`${styles.ratingValue} ${styles.ratingValueBase}`}>
              {displayRating}
            </span>
            {isDefeat && (
              <span
                className={`${styles.ratingValue} ${styles.ratingValueOverlay}`}
                style={{ opacity: defeatOverlayOpacity }}
                aria-hidden
              >
                {displayRating}
              </span>
            )}
          </div>
        </div>
      )}

      <div className={styles.actions}>
        <button
          type="button"
          onClick={onPlayAgain}
          disabled={!connected}
          className={styles.primaryButton}
        >
          Play Again
        </button>
        <button type="button" onClick={onBackToHome} className={styles.secondaryButton}>
          Back to home
        </button>
      </div>

      {!connected && <p className={styles.connection}>Reconnecting to server...</p>}
    </section>
  );
}
