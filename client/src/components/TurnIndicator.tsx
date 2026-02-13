import type { TurnPhase } from "../types/game";
import styles from "../styles/TurnIndicator.module.css";

interface TurnIndicatorProps {
  yourTurn: boolean;
  phase: TurnPhase;
}

function getMessage(yourTurn: boolean, phase: TurnPhase): string {
  if (phase === "resolve") {
    return "Resolving turn...";
  }
  return yourTurn ? "Your turn" : "Opponent's turn";
}

export default function TurnIndicator({ yourTurn, phase }: TurnIndicatorProps) {
  return (
    <div className={`${styles.indicator} ${yourTurn ? styles.yours : styles.theirs}`}>
      {getMessage(yourTurn, phase)}
    </div>
  );
}
