import { useEffect, useState } from "react";
import Board from "../components/Board";
import ComboIndicator from "../components/ComboIndicator";
import PowerUpShop from "../components/PowerUpShop";
import ScorePanel from "../components/ScorePanel";
import TurnIndicator from "../components/TurnIndicator";
import type { GameStateMsg, MatchFoundMsg } from "../types/messages";
import styles from "../styles/GameScreen.module.css";

interface GameScreenProps {
  connected: boolean;
  matchInfo: MatchFoundMsg | null;
  gameState: GameStateMsg | null;
  onFlipCard: (index: number) => void;
  onUsePowerUp: (powerUpId: string, cardIndex?: number) => void;
}

export default function GameScreen({
  connected,
  matchInfo,
  gameState,
  onFlipCard,
  onUsePowerUp,
}: GameScreenProps) {
  const [pendingRadarTarget, setPendingRadarTarget] = useState(false);

  useEffect(() => {
    if (gameState && (!gameState.yourTurn || gameState.phase !== "first_flip")) {
      setPendingRadarTarget(false);
    }
  }, [gameState?.yourTurn, gameState?.phase]);

  if (!matchInfo) {
    return (
      <section className={styles.screen}>
        <p>Waiting for match details...</p>
      </section>
    );
  }

  if (!gameState) {
    return (
      <section className={styles.screen}>
        <p>Match found! Waiting for initial board state...</p>
      </section>
    );
  }

  const cardsClickable = connected && gameState.yourTurn && gameState.phase !== "resolve";
  const powerUpsEnabled = connected && gameState.yourTurn && gameState.phase === "first_flip";

  const handleCardClick = (index: number): void => {
    if (pendingRadarTarget) {
      onUsePowerUp("radar", index);
      setPendingRadarTarget(false);
    } else {
      onFlipCard(index);
    }
  };

  const handleUsePowerUpClick = (powerUpId: string): void => {
    if (powerUpId === "radar") {
      setPendingRadarTarget(true);
    } else {
      onUsePowerUp(powerUpId);
    }
  };

  return (
    <section className={styles.screen}>
      <header className={styles.header}>
        <h2>You vs {matchInfo.opponentName}</h2>
        <TurnIndicator yourTurn={gameState.yourTurn} phase={gameState.phase} />
      </header>

      <div className={styles.main}>
        <div className={styles.leftColumn}>
          <Board
            cards={gameState.cards}
            rows={matchInfo.boardRows}
            cols={matchInfo.boardCols}
            cardsClickable={cardsClickable}
            onCardClick={handleCardClick}
            radarTargetingMode={pendingRadarTarget}
          />
        </div>

        <aside className={styles.rightColumn}>
          <ScorePanel
            you={gameState.you}
            opponent={gameState.opponent}
            yourTurn={gameState.yourTurn}
          />
          <ComboIndicator comboStreak={gameState.you.comboStreak} label="Combo" />
          <PowerUpShop
            powerUps={gameState.availablePowerUps}
            enabled={powerUpsEnabled}
            onUsePowerUp={handleUsePowerUpClick}
            secondChanceRoundsRemaining={gameState.you.secondChanceRoundsRemaining}
          />
        </aside>
      </div>
    </section>
  );
}
