import type { PowerUpView } from "../types/game";
import { POWER_UP_DISPLAY } from "../powerups/registry";
import styles from "../styles/PowerUpShop.module.css";

interface PowerUpShopProps {
  powerUps: PowerUpView[];
  enabled: boolean;
  onUsePowerUp: (powerUpId: string) => void;
  /** Rounds the Second Chance power-up is still active (0 or undefined = inactive). */
  secondChanceRoundsRemaining?: number;
}

export default function PowerUpShop({
  powerUps,
  enabled,
  onUsePowerUp,
  secondChanceRoundsRemaining = 0,
}: PowerUpShopProps) {
  return (
    <section className={styles.shop} aria-label="Power-up shop">
      <h3>Power-Ups</h3>
      {powerUps.length === 0 ? (
        <p className={styles.empty}>No power-ups available.</p>
      ) : (
        <ul className={styles.list}>
          {powerUps.map((powerUp) => {
            const display = POWER_UP_DISPLAY[powerUp.id];
            const isSecondChanceActive = powerUp.id === "second_chance" && secondChanceRoundsRemaining > 0;
            const buttonDisabled =
              !enabled ||
              !powerUp.canAfford ||
              isSecondChanceActive;

            return (
              <li key={powerUp.id} className={styles.item}>
                <div className={styles.info}>
                  <span className={styles.icon}>{display?.icon ?? "PWR"}</span>
                  <div>
                    <p className={styles.name}>{display?.label ?? powerUp.name}</p>
                    <p className={styles.description}>
                      {display?.description ?? powerUp.description}
                    </p>
                    {powerUp.id === "second_chance" && (
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
                  onClick={() => onUsePowerUp(powerUp.id)}
                >
                  Use ({powerUp.cost})
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
