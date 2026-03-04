import { Link } from "react-router-dom";
import type { PowerUpInHand } from "../types/game";
import { POWER_UP_DISPLAY } from "../powerups/registry";
import PowerUpHand from "../components/PowerUpHand";
import styles from "../styles/PreviewCardsPage.module.css";

const syntheticHand: PowerUpInHand[] = Object.keys(POWER_UP_DISPLAY).map((powerUpId) => ({
  powerUpId,
  count: 1,
  usableCount: 1,
}));

export function PreviewCardsPage() {
  return (
    <div className={styles.wrapper}>
      <h1 className={styles.title}>Preview Cards</h1>
      <Link to="/admin" className={styles.backLink}>
        Back to Admin
      </Link>
      <p className={styles.subtitle}>
        All power-up cards as they appear in hand. Click a card to open the full modal.
      </p>
      <div className={styles.handWrap}>
        <PowerUpHand
          hand={syntheticHand}
          enabled={false}
          onUsePowerUp={() => {}}
        />
      </div>
    </div>
  );
}
