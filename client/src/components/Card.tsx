import type { CardView } from "../types/game";
import styles from "../styles/Card.module.css";

interface CardProps {
  card: CardView;
  disabled: boolean;
  onClick: (index: number) => void;
  /** When true, this card is the Radar target (center of 3x3). */
  isRadarCenter?: boolean;
  /** When true, this card is in the Radar 3x3 area but not the center. */
  isRadarAffected?: boolean;
  onMouseEnter?: () => void;
  onMouseLeave?: () => void;
}

function getCardValue(card: CardView): string {
  if (card.state === "hidden") {
    return "?";
  }
  return String(card.pairId ?? "");
}

export default function Card({
  card,
  disabled,
  onClick,
  isRadarCenter = false,
  isRadarAffected = false,
  onMouseEnter,
  onMouseLeave,
}: CardProps) {
  const cardClasses = [
    styles.card,
    card.state !== "hidden" ? styles.faceUp : "",
    card.state === "matched" ? styles.matched : "",
    isRadarCenter ? styles.radarCenter : "",
    isRadarAffected ? styles.radarAffected : "",
  ]
    .filter(Boolean)
    .join(" ");

  return (
    <button
      type="button"
      className={cardClasses}
      onClick={() => onClick(card.index)}
      disabled={disabled}
      onMouseEnter={onMouseEnter}
      onMouseLeave={onMouseLeave}
      aria-label={`Card ${card.index}`}
    >
      <span className={styles.inner}>{getCardValue(card)}</span>
    </button>
  );
}
