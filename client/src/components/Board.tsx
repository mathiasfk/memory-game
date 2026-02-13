import { useEffect, useRef, useState } from "react";
import Card from "./Card";
import type { CardView } from "../types/game";
import styles from "../styles/Board.module.css";

const MATCHED_DISPLAY_MS = 800;

interface BoardProps {
  cards: CardView[];
  rows: number;
  cols: number;
  cardsClickable: boolean;
  onCardClick: (index: number) => void;
}

export default function Board({
  cards,
  rows,
  cols,
  cardsClickable,
  onCardClick,
}: BoardProps) {
  const [removedMatchedIndices, setRemovedMatchedIndices] = useState<Set<number>>(new Set());
  const timeoutsRef = useRef<Map<number, ReturnType<typeof setTimeout>>>(new Map());
  const pendingRemovalRef = useRef<Set<number>>(new Set());

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

  return (
    <div
      className={styles.board}
      style={{
        gridTemplateColumns: `repeat(${cols}, minmax(0, 1fr))`,
        gridTemplateRows: `repeat(${rows}, minmax(0, 1fr))`,
      }}
    >
      {sortedCards.map((card) =>
        showAsEmpty(card.index, card.state) ? (
          <div key={card.index} className={styles.emptyCell} aria-hidden="true" />
        ) : (
          <Card
            key={card.index}
            card={card}
            disabled={!cardsClickable || card.state !== "hidden"}
            onClick={onCardClick}
          />
        )
      )}
    </div>
  );
}
