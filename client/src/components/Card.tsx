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
  /** When true (Discernment), this hidden card has never been revealed. */
  isUnknownHighlight?: boolean;
  onMouseEnter?: () => void;
  onMouseLeave?: () => void;
}

export default function Card({
  card,
  disabled,
  onClick,
  isRadarCenter = false,
  isRadarAffected = false,
  isUnknownHighlight = false,
  onMouseEnter,
  onMouseLeave,
}: CardProps) {
  const isFaceUp = card.state !== "hidden";

  const wrapperClasses = [
    styles.cardWrapper,
    isRadarCenter ? styles.radarCenter : "",
    isRadarAffected ? styles.radarAffected : "",
    isUnknownHighlight ? styles.unknownHighlight : "",
  ]
    .filter(Boolean)
    .join(" ");

  return (
    <button
      type="button"
      className={wrapperClasses}
      onClick={() => onClick(card.index)}
      disabled={disabled}
      onMouseEnter={onMouseEnter}
      onMouseLeave={onMouseLeave}
      aria-label={`Card ${card.index}`}
    >
      <div className={`${styles.cardInner} ${isFaceUp ? styles.faceUp : ""}`}>
        <div className={styles.cardBack}>
          <span className={styles.inner}>?</span>
        </div>
        <div
          className={`${styles.cardFront} ${card.state === "matched" ? styles.matched : ""}`}
        >
          <span className={styles.inner}>{String(card.pairId ?? "")}</span>
        </div>
      </div>
    </button>
  );
}
