import type { CardView } from "../types/game";
import { getPowerUpDisplayByPairId, NUM_POWERUP_PAIRS } from "../powerups/registry";
import { getNormalSymbolForPairId } from "../constants/symbols";
import styles from "../styles/Card.module.css";

interface CardProps {
  card: CardView;
  disabled: boolean;
  onClick: (index: number) => void;
  /** Per-match arcana mapping (pairId -> powerUpId); when set, used for power-up display. */
  pairIdToPowerUp?: Record<string, string> | null;
  /** When true, this card is the Radar target (center of 3x3). */
  isRadarCenter?: boolean;
  /** When true, this card is in the Radar 3x3 area but not the center. */
  isRadarAffected?: boolean;
  /** When true, this card is highlighted (Unveiling or Elemental power-up). */
  isHighlighted?: boolean;
  onMouseEnter?: () => void;
  onMouseLeave?: () => void;
}

export default function Card({
  card,
  disabled,
  onClick,
  pairIdToPowerUp = null,
  isRadarCenter = false,
  isRadarAffected = false,
  isHighlighted = false,
  onMouseEnter,
  onMouseLeave,
}: CardProps) {
  const isFaceUp = card.state !== "hidden" && card.state !== "removed";
  const pairId = card.pairId ?? null;
  const powerUpDisplay =
    pairId != null && pairId < NUM_POWERUP_PAIRS ? getPowerUpDisplayByPairId(pairId, pairIdToPowerUp) : null;
  const normalSymbol = pairId != null && pairId >= NUM_POWERUP_PAIRS ? getNormalSymbolForPairId(pairId) : null;

  const ariaLabel =
    pairId != null
      ? powerUpDisplay
        ? `Card ${powerUpDisplay.label}`
        : "Card symbol"
      : `Card ${card.index}`;

  const wrapperClasses = [
    styles.cardWrapper,
    isRadarCenter ? styles.radarCenter : "",
    isRadarAffected ? styles.radarAffected : "",
    isHighlighted ? styles.cardHighlight : "",
  ]
    .filter(Boolean)
    .join(" ");

  const faceContent =
    pairId != null && isFaceUp ? (
      powerUpDisplay ? (
        <img
          className={styles.cardImage}
          src={powerUpDisplay.imagePath}
          alt=""
          aria-hidden
        />
      ) : normalSymbol ? (
        <span
          className={styles.cardSymbol}
          style={{ color: normalSymbol.color }}
          aria-hidden
        >
          {normalSymbol.symbol}
        </span>
      ) : (
        <span className={styles.inner}>{String(pairId)}</span>
      )
    ) : (
      <span className={styles.inner}>{String(pairId ?? "")}</span>
    );

  return (
    <button
      type="button"
      className={wrapperClasses}
      onClick={() => onClick(card.index)}
      disabled={disabled}
      onMouseEnter={onMouseEnter}
      onMouseLeave={onMouseLeave}
      aria-label={ariaLabel}
    >
      <div className={`${styles.cardInner} ${isFaceUp ? styles.faceUp : ""}`}>
        <div
          className={styles.cardBack}
          style={
            isHighlighted
              ? {
                  boxShadow:
                    "0 0 0 2px rgba(255, 255, 255, 0.9), 0 0 12px rgba(255, 255, 255, 0.5)",
                }
              : undefined
          }
        >
          <img
            className={styles.cardBackImage}
            src="/cards/Verse.webp"
            alt=""
            aria-hidden
          />
        </div>
        <div className={styles.cardFront}>
          {faceContent}
        </div>
      </div>
    </button>
  );
}
