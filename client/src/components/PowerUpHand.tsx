import { useCallback, useEffect, useRef, useState } from "react";
import type { PowerUpInHand } from "../types/game";
import { POWER_UP_DISPLAY } from "../powerups/registry";
import styles from "../styles/PowerUpHand.module.css";

function handDescription(
  display: { shortDescription?: string; description: string } | undefined
): string {
  if (!display) return "";
  return display.shortDescription ?? display.description;
}

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
  const [selectedPowerUpId, setSelectedPowerUpId] = useState<string | null>(null);
  const modalPanelRef = useRef<HTMLDivElement>(null);
  const closeButtonRef = useRef<HTMLButtonElement>(null);

  const closeModal = useCallback(() => {
    setSelectedPowerUpId(null);
  }, []);

  const handleUseFromModal = useCallback(
    (powerUpId: string) => {
      onUsePowerUp(powerUpId);
      closeModal();
    },
    [onUsePowerUp, closeModal]
  );

  useEffect(() => {
    if (selectedPowerUpId == null) return;
    closeButtonRef.current?.focus();
  }, [selectedPowerUpId]);

  useEffect(() => {
    if (selectedPowerUpId == null) return;
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.preventDefault();
        closeModal();
      }
    };
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [selectedPowerUpId, closeModal]);

  return (
    <>
      <section className={styles.hand} aria-label="Power-up hand">
        {items.length === 0 ? (
          <p className={styles.empty}>No power-ups in hand. Match pairs to collect them.</p>
        ) : (
          <ul className={styles.list}>
            {items.map((item) => {
              const display = POWER_UP_DISPLAY[item.powerUpId];
              const usable = item.usableCount ?? item.count;
              const onCooldown = item.count > 0 && usable === 0;

              return (
                <li key={item.powerUpId}>
                  <button
                    type="button"
                    className={styles.item}
                    onClick={() => setSelectedPowerUpId(item.powerUpId)}
                    aria-label={`${display?.label ?? item.powerUpId}${item.count > 1 ? `, ${item.count} in hand` : ""}${onCooldown ? ", available next turn" : ""}. Click to view details and use.`}
                  >
                    {display?.imagePath ? (
                      <img
                        className={styles.cardArt}
                        src={display.imagePath}
                        alt=""
                        aria-hidden
                      />
                    ) : null}
                    <span className={styles.title}>
                      {display?.label ?? item.powerUpId}
                      {item.count > 1 ? ` ×${item.count}` : ""}
                    </span>
                    <span className={styles.description}>
                      {onCooldown
                        ? "Available next turn."
                        : handDescription(display)}
                    </span>
                  </button>
                </li>
              );
            })}
          </ul>
        )}
      </section>

      {selectedPowerUpId != null && (() => {
        const display = POWER_UP_DISPLAY[selectedPowerUpId];
        const item = items.find((i) => i.powerUpId === selectedPowerUpId);
        const usable = item ? (item.usableCount ?? item.count) : 0;
        const buttonDisabled = !enabled || usable < 1;
        const onCooldown = item != null && item.count > 0 && (item.usableCount ?? 0) === 0;

        return (
          <div
            className={styles.modalOverlay}
            role="dialog"
            aria-modal="true"
            aria-labelledby="powerup-modal-title"
            aria-describedby="powerup-modal-desc"
            onClick={(e) => {
              if (e.target === e.currentTarget) closeModal();
            }}
          >
            <div
              ref={modalPanelRef}
              className={styles.modalPanel}
              onClick={(e) => e.stopPropagation()}
            >
              {display?.imagePath ? (
                <img
                  className={styles.modalArt}
                  src={display.imagePath}
                  alt=""
                  aria-hidden
                />
              ) : null}
              <h2 id="powerup-modal-title" className={styles.modalTitle}>
                {display?.label ?? selectedPowerUpId}
                {item && item.count > 1 ? ` ×${item.count}` : ""}
              </h2>
              <p id="powerup-modal-desc" className={styles.modalDescription}>
                {display?.description ?? ""}
                {onCooldown ? " Available next turn." : ""}
              </p>
              <div className={styles.modalActions}>
                <button
                  type="button"
                  className={styles.modalUseButton}
                  disabled={buttonDisabled}
                  onClick={() => handleUseFromModal(selectedPowerUpId)}
                >
                  Use
                </button>
                <button
                  ref={closeButtonRef}
                  type="button"
                  className={styles.modalCloseButton}
                  onClick={closeModal}
                >
                  Close
                </button>
              </div>
            </div>
          </div>
        );
      })()}
    </>
  );
}
