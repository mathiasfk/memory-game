import type { CardView } from "../types/game";
import { getPowerUpDisplayByPairId } from "../powerups/registry";
import {
  getNormalSymbolByElement,
  getNormalSymbolForPairId,
} from "../constants/symbols";
import styles from "../styles/Card.module.css";

/** Fallback when server does not send arcanaPairs (backward compat). Must match server ArcanaPairsPerMatch. */
const DEFAULT_ARCANA_PAIRS = 6;

/**
 * Renders a single card. Arcana vs normal and element (fire/water/air/earth) come only from server:
 * - pairIdToPowerUp (which pairIds are arcana), arcanaPairs (count), card.element (element for normal tiles when revealed).
 */
interface CardProps {
  card: CardView;
  disabled: boolean;
  onClick: (index: number) => void;
  /** From server: pairId -> powerUpId for arcana pairs. If present, used to decide arcana; else arcanaPairs is used. */
  pairIdToPowerUp?: Record<string, string> | null;
  /** From server: number of arcana pairs. Used when pairIdToPowerUp is absent and for normal-symbol fallback. */
  arcanaPairs?: number;
  /** When true, this card is the Radar target (center of 3x3). */
  isRadarCenter?: boolean;
  /** When true, this card is in the Radar 3x3 area but not the center. */
  isRadarAffected?: boolean;
  /** When true, this card is highlighted (Unveiling or Elemental power-up). */
  isHighlighted?: boolean;
  /** When true, this card is part of a match (show white outline until removed). */
  isMatched?: boolean;
  onMouseEnter?: () => void;
  onMouseLeave?: () => void;
}

export default function Card({
  card,
  disabled,
  onClick,
  pairIdToPowerUp = null,
  arcanaPairs,
  isRadarCenter = false,
  isRadarAffected = false,
  isHighlighted = false,
  isMatched = false,
  onMouseEnter,
  onMouseLeave,
}: CardProps) {
  const isFaceUp = card.state !== "hidden" && card.state !== "removed";
  const pairId = card.pairId ?? null;
  const effectiveArcanaPairs = arcanaPairs ?? DEFAULT_ARCANA_PAIRS;
  // Arcana vs normal: server truth via pairIdToPowerUp (which pairIds are arcana) or arcanaPairs cutoff.
  const isArcana =
    pairId != null &&
    (pairIdToPowerUp != null
      ? pairIdToPowerUp[String(pairId)] != null
      : pairId < effectiveArcanaPairs);
  const powerUpDisplay =
    isArcana && pairId != null ? getPowerUpDisplayByPairId(pairId, pairIdToPowerUp) : null;
  // Normal card display: prefer server element (card.element) for symbol/color; fallback to pairId + arcanaPairs only when element not sent.
  const normalSymbol =
    pairId != null && !isArcana
      ? card.element != null
        ? getNormalSymbolByElement(
            card.element,
            (pairId - effectiveArcanaPairs) % 3
          )
        : getNormalSymbolForPairId(pairId, effectiveArcanaPairs)
      : null;

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
      <div
        className={`${styles.cardInner} ${isFaceUp ? styles.faceUp : ""} ${isMatched ? styles.cardMatchHighlight : ""}`}
      >
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
