import type { PowerUpInHand } from "../types/game";
import { POWER_UP_DISPLAY } from "../powerups/registry";
import styles from "../styles/PowerUpHand.module.css";

interface PowerUpHandProps {
  hand: PowerUpInHand[];
  enabled: boolean;
  onUsePowerUp: (powerUpId: string) => void;
}

export default function PowerUpHand({
  hand,
  enabled,
  onUsePowerUp,
}: PowerUpHandProps) {
  const items = hand.filter((item) => item.count > 0);

  return (
    <section className={styles.hand} aria-label="Power-up hand">
      {items.length === 0 ? (
        <p className={styles.empty}>No power-ups in hand. Match pairs to collect them.</p>
      ) : (
        <ul className={styles.list}>
          {items.map((item) => {
            const display = POWER_UP_DISPLAY[item.powerUpId];
            const buttonDisabled = !enabled;

            return (
              <li key={item.powerUpId} className={styles.item}>
                {display?.imagePath ? (
                  <img
                    className={styles.cardArt}
                    src={display.imagePath}
                    alt=""
                    aria-hidden
                  />
                ) : null}
                <p className={styles.title}>
                  {display?.label ?? item.powerUpId}
                  {item.count > 1 ? ` ×${item.count}` : ""}
                </p>
                <p className={styles.description}>
                  {display?.description ?? ""}
                </p>
                <button
                  type="button"
                  className={styles.useButton}
                  disabled={buttonDisabled}
                  onClick={() => onUsePowerUp(item.powerUpId)}
                >
                  Use
                </button>
              </li>
            );
          })}
        </ul>
      )}
    </section>
  );
}
