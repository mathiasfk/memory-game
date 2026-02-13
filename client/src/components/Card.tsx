import type { CardView } from "../types/game";
import styles from "../styles/Card.module.css";

interface CardProps {
  card: CardView;
  disabled: boolean;
  onClick: (index: number) => void;
}

function getCardValue(card: CardView): string {
  if (card.state === "hidden") {
    return "?";
  }
  return String(card.pairId ?? "");
}

export default function Card({ card, disabled, onClick }: CardProps): JSX.Element {
  const cardClasses = [
    styles.card,
    card.state !== "hidden" ? styles.faceUp : "",
    card.state === "matched" ? styles.matched : "",
  ]
    .filter(Boolean)
    .join(" ");

  return (
    <button
      type="button"
      className={cardClasses}
      onClick={() => onClick(card.index)}
      disabled={disabled}
      aria-label={`Card ${card.index}`}
    >
      <span className={styles.inner}>{getCardValue(card)}</span>
    </button>
  );
}
