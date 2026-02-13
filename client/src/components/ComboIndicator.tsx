import { useEffect, useRef, useState } from "react";
import styles from "../styles/ComboIndicator.module.css";

interface ComboIndicatorProps {
  comboStreak: number;
  label: string;
}

export default function ComboIndicator({ comboStreak, label }: ComboIndicatorProps) {
  const previousComboRef = useRef(comboStreak);
  const [visibleCombo, setVisibleCombo] = useState<number | null>(null);

  useEffect(() => {
    if (comboStreak > previousComboRef.current && comboStreak > 1) {
      setVisibleCombo(comboStreak);
      const timeoutId = window.setTimeout(() => {
        setVisibleCombo(null);
      }, 900);

      previousComboRef.current = comboStreak;
      return () => {
        window.clearTimeout(timeoutId);
      };
    }

    previousComboRef.current = comboStreak;
    return undefined;
  }, [comboStreak]);

  if (visibleCombo === null) {
    return <div className={styles.placeholder} aria-hidden="true" />;
  }

  return (
    <div className={styles.combo} role="status" aria-live="polite">
      {label}: x{visibleCombo}
    </div>
  );
}
