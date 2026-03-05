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
  /** ELO sent later via rating_update when server persists in background. */
  ratingOverride?: { you_elo_before: number; you_elo_after: number } | null;
  opponentDisconnected: boolean;
  latestGameState: GameStateMsg | null;
  /** When result is null and not opponentDisconnected (e.g. missed game_over), show this title instead of "Opponent disconnected". */
  titleWhenNoResult?: string;
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
  ratingOverride,
  opponentDisconnected,
  latestGameState,
  titleWhenNoResult,
  onPlayAgain,
  onBackToHome,
}: GameOverScreenProps) {
  const yourScore = result?.you.score ?? latestGameState?.you.score ?? 0;
  const opponentScore = result?.opponent.score ?? latestGameState?.opponent.score ?? 0;
  const yourName = result?.you.name ?? latestGameState?.you.name ?? "You";
  const opponentName = result?.opponent.name ?? latestGameState?.opponent.name ?? "Opponent";

  const eloBefore = ratingOverride?.you_elo_before ?? result?.you_elo_before;
  const eloAfter = ratingOverride?.you_elo_after ?? result?.you_elo_after;
  const hasRating = eloBefore != null && eloAfter != null;
  const showRatingBlock = result != null && !opponentDisconnected;
  const showRating = showRatingBlock && hasRating;

  const [displayRating, setDisplayRating] = useState(
    showRating ? eloBefore : eloAfter ?? eloBefore ?? 0
  );

  const youWon = result?.result === "win";
  const opponentWon = result?.result === "lose";

  useEffect(() => {
    if (!showRating || eloBefore == null || eloAfter == null) return;
    setDisplayRating(eloBefore);
    let cancelled = false;
    const start = performance.now();
    const totalDuration = RATING_HOLD_MS + RATING_ANIMATION_DURATION_MS;

    const tick = (now: number) => {
      if (cancelled) return;
      const elapsed = now - start;
      if (elapsed >= totalDuration) {
        setDisplayRating(eloAfter);
        return;
      }
      if (elapsed < RATING_HOLD_MS) {
        setDisplayRating(eloBefore);
      } else {
        const animProgress = (elapsed - RATING_HOLD_MS) / RATING_ANIMATION_DURATION_MS;
        const eased = easeOutCubic(animProgress);
        setDisplayRating(
          Math.round(eloBefore + (eloAfter - eloBefore) * eased)
        );
      }
      requestAnimationFrame(tick);
    };
    requestAnimationFrame(tick);
    return () => {
      cancelled = true;
    };
  }, [showRating, eloBefore, eloAfter]);

  return (
    <section className={styles.screen}>
      <h2 className={styles.title}>
        {result !== null
          ? resultLabel(result.result)
          : opponentDisconnected
            ? "Opponent disconnected"
            : titleWhenNoResult ?? "Opponent disconnected"}
      </h2>

      <div className={styles.scores}>
        <p className={youWon ? styles.winner : undefined}>
          {yourName}: <strong>{yourScore}</strong>
        </p>
        <p className={opponentWon ? styles.winner : undefined}>
          {opponentName}: <strong>{opponentScore}</strong>
        </p>
      </div>

      {showRatingBlock && (
        <div className={styles.ratingBlock}>
          <span className={styles.ratingLabel}>Rating</span>
          <div className={styles.ratingValueWrap}>
            {hasRating ? (
              <>
                <span className={`${styles.ratingValue} ${styles.ratingValueBase}`}>
                  {displayRating}
                </span>
                {opponentWon && (
                  <span
                    className={`${styles.ratingValue} ${styles.ratingValueOverlay} ${styles.ratingValueOverlayAnimating}`}
                    aria-hidden
                  >
                    {displayRating}
                  </span>
                )}
              </>
            ) : (
              <span className={styles.ratingPlaceholder} aria-live="polite">
                {" "}
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
