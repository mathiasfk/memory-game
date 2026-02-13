import Card from "./Card";
import type { CardView } from "../types/game";
import styles from "../styles/Board.module.css";

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
  const sortedCards = [...cards].sort((a, b) => a.index - b.index);

  return (
    <div
      className={styles.board}
      style={{
        gridTemplateColumns: `repeat(${cols}, minmax(0, 1fr))`,
        gridTemplateRows: `repeat(${rows}, minmax(0, 1fr))`,
      }}
    >
      {sortedCards.map((card) =>
        card.state === "matched" ? (
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
