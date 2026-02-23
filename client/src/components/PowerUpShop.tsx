import type { PowerUpInHand } from "../types/game";
import { POWER_UP_DISPLAY } from "../powerups/registry";
import styles from "../styles/PowerUpShop.module.css";

interface PowerUpShopProps {
  hand: PowerUpInHand[];
  enabled: boolean;
  onUsePowerUp: (powerUpId: string) => void;
  /** Rounds the Second Chance power-up is still active (0 or undefined = inactive). */
  secondChanceRoundsRemaining?: number;
}

export default function PowerUpShop({
  hand,
  enabled,
  onUsePowerUp,
  secondChanceRoundsRemaining = 0,
}: PowerUpShopProps) {
  const items = hand.filter((item) => item.count > 0);

  return (
    <section className={styles.shop} aria-label="Power-up hand">
      <h3>Power-Ups</h3>
      {items.length === 0 ? (
        <p className={styles.empty}>No power-ups in hand. Match pairs to earn them.</p>
      ) : (
        <ul className={styles.list}>
          {items.map((item) => {
            const display = POWER_UP_DISPLAY[item.powerUpId];
            const isSecondChanceActive =
              item.powerUpId === "second_chance" && secondChanceRoundsRemaining > 0;
            const buttonDisabled = !enabled || isSecondChanceActive;

            return (
              <li key={item.powerUpId} className={styles.item}>
                <div className={styles.info}>
                  <span className={styles.icon}>{display?.icon ?? "PWR"}</span>
                  <div>
                    <p className={styles.name}>
                      {display?.label ?? item.powerUpId}
                      {item.count > 1 ? ` ×${item.count}` : ""}
                    </p>
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
                  </div>
                </div>
                <button
                  type="button"
                  className={styles.buyButton}
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
      {!enabled && <p className={styles.hint}>Use power-ups on your turn before first flip.</p>}
    </section>
  );
}
