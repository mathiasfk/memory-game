import type { PlayerView } from "../types/game";
import styles from "../styles/ScorePanel.module.css";

interface ScorePanelProps {
  you: PlayerView;
  opponent: PlayerView;
  yourTurn: boolean;
}

function PlayerScoreCard({
  title,
  player,
  active,
}: {
  title: string;
  player: PlayerView;
  active: boolean;
}) {
  return (
    <div className={`${styles.playerCard} ${active ? styles.active : ""}`}>
      <h3>{title}</h3>
      <p className={styles.name}>{player.name}</p>
      <p>Score: {player.score}</p>
    </div>
  );
}

export default function ScorePanel({ you, opponent, yourTurn }: ScorePanelProps) {
  return (
    <section className={styles.panel} aria-label="Score panel">
      <PlayerScoreCard title="You" player={you} active={yourTurn} />
      <PlayerScoreCard title="Opponent" player={opponent} active={!yourTurn} />
    </section>
  );
}
