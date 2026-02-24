import { useEffect, useRef, useState } from "react";
import Card from "./Card";
import type { CardView } from "../types/game";
import styles from "../styles/Board.module.css";

const MATCHED_DISPLAY_MS = 800;

/** Returns the 3x3 region around centerIndex (center + affected indices). */
function radarRegion(rows: number, cols: number, centerIndex: number): { center: number; affected: number[] } {
  const centerRow = Math.floor(centerIndex / cols);
  const centerCol = centerIndex % cols;
  const minR = Math.max(0, centerRow - 1);
  const maxR = Math.min(rows - 1, centerRow + 1);
  const minC = Math.max(0, centerCol - 1);
  const maxC = Math.min(cols - 1, centerCol + 1);
  const affected: number[] = [];
  for (let r = minR; r <= maxR; r++) {
    for (let c = minC; c <= maxC; c++) {
      const idx = r * cols + c;
      if (idx !== centerIndex) affected.push(idx);
    }
  }
  return { center: centerIndex, affected };
}

interface BoardProps {
  cards: CardView[];
  rows: number;
  cols: number;
  cardsClickable: boolean;
  onCardClick: (index: number) => void;
  /** When true, player is choosing a card as Radar target; hover shows 3x3 preview. */
  radarTargetingMode?: boolean;
  /** When true, highlight hidden tiles that have never been revealed (Discernment). */
  knownIndices?: number[];
  discernmentHighlightActive?: boolean;
}

export default function Board({
  cards,
  rows,
  cols,
  cardsClickable,
  onCardClick,
  radarTargetingMode = false,
  knownIndices = [],
  discernmentHighlightActive = false,
}: BoardProps) {
  const [removedMatchedIndices, setRemovedMatchedIndices] = useState<Set<number>>(new Set());
  const [hoveredIndex, setHoveredIndex] = useState<number | null>(null);
  const timeoutsRef = useRef<Map<number, ReturnType<typeof setTimeout>>>(new Map());
  const pendingRemovalRef = useRef<Set<number>>(new Set());

  const radarPreview =
    radarTargetingMode && hoveredIndex !== null ? radarRegion(rows, cols, hoveredIndex) : null;

  const sortedCards = [...cards].sort((a, b) => a.index - b.index);
  const matchedIndices = sortedCards.filter((c) => c.state === "matched").map((c) => c.index);

  useEffect(() => {
    if (matchedIndices.length === 0) {
      setRemovedMatchedIndices((prev) => (prev.size > 0 ? new Set() : prev));
      pendingRemovalRef.current = new Set();
      timeoutsRef.current.forEach(clearTimeout);
      timeoutsRef.current = new Map();
      return;
    }

    const alreadyScheduledOrRemoved = (idx: number) =>
      removedMatchedIndices.has(idx) || pendingRemovalRef.current.has(idx);

    matchedIndices.forEach((index) => {
      if (alreadyScheduledOrRemoved(index)) return;
      pendingRemovalRef.current.add(index);
      const id = setTimeout(() => {
        setRemovedMatchedIndices((prev) => new Set(prev).add(index));
        timeoutsRef.current.delete(index);
      }, MATCHED_DISPLAY_MS);
      timeoutsRef.current.set(index, id);
    });

    return () => {};
  }, [matchedIndices.join(",")]);

  useEffect(() => {
    return () => {
      timeoutsRef.current.forEach(clearTimeout);
      timeoutsRef.current = new Map();
    };
  }, []);

  const showAsEmpty = (index: number, state: CardView["state"]) =>
    state === "matched" && removedMatchedIndices.has(index);

  const isUnknownHighlight = (index: number, state: CardView["state"]) =>
    discernmentHighlightActive && state === "hidden" && !knownIndices.includes(index);

  const isDisabled = (card: CardView) =>
    radarTargetingMode ? card.state !== "hidden" : !cardsClickable || card.state !== "hidden";

  return (
    <div
      className={styles.board}
      style={{
        gridTemplateColumns: `repeat(${cols}, minmax(0, 1fr))`,
        gridTemplateRows: `repeat(${rows}, minmax(0, 1fr))`,
      }}
      onMouseLeave={() => setHoveredIndex(null)}
    >
      {sortedCards.map((card) =>
        showAsEmpty(card.index, card.state) ? (
          <div key={card.index} className={styles.emptyCell} aria-hidden="true" />
        ) : (
          <Card
            key={card.index}
            card={card}
            disabled={isDisabled(card)}
            onClick={onCardClick}
            isRadarCenter={radarPreview?.center === card.index}
            isRadarAffected={radarPreview?.affected.includes(card.index) ?? false}
            isUnknownHighlight={isUnknownHighlight(card.index, card.state)}
            onMouseEnter={() => setHoveredIndex(card.index)}
            onMouseLeave={() => setHoveredIndex(null)}
          />
        )
      )}
    </div>
  );
}
