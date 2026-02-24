import type { PowerUpInHand } from "../types/game";
import { POWER_UP_DISPLAY } from "../powerups/registry";
import styles from "../styles/PowerUpHand.module.css";

interface PowerUpHandProps {
  hand: PowerUpInHand[];
  enabled: boolean;
  onUsePowerUp: (powerUpId: string) => void;
  /** Rounds the Second Chance power-up is still active (0 or undefined = inactive). */
  secondChanceRoundsRemaining?: number;
}

export default function PowerUpHand({
  hand,
  enabled,
  onUsePowerUp,
  secondChanceRoundsRemaining = 0,
}: PowerUpHandProps) {
  const items = hand.filter((item) => item.count > 0);

  return (
    <section className={styles.hand} aria-label="Power-up hand">
      {items.length === 0 ? (
        <p className={styles.empty}>No power-ups in hand. Match pairs to collect them.</p>
      ) : (
        <ul className={styles.list}>
          {items.map((item, index) => {
            const display = POWER_UP_DISPLAY[item.powerUpId];
            const isSecondChanceActive =
              item.powerUpId === "second_chance" && secondChanceRoundsRemaining > 0;
            const buttonDisabled = !enabled || isSecondChanceActive;

            return (
              <li key={item.powerUpId} className={styles.item}>
                <p className={styles.title}>
                  {display?.label ?? item.powerUpId}
                  {item.count > 1 ? ` ×${item.count}` : ""}
                </p>
                <div className={styles.symbol} aria-hidden>
                  {index + 1}
                </div>
                <p className={styles.description}>
                  {display?.description ?? ""}
                </p>
                {item.powerUpId === "second_chance" && (
                  <p className={styles.status} aria-live="polite">
                    {secondChanceRoundsRemaining > 0
                      ? `Active (${secondChanceRoundsRemaining} rounds left)`
                      : "Not active"}
                  </p>
                )}
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
