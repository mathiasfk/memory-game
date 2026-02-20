import type { GameOverMsg, GameStateMsg } from "../types/messages";
import styles from "../styles/GameOverScreen.module.css";

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
