import styles from "../styles/BotTag.module.css";

export interface BotTagProps {
  /** Use "onAccent" when the tag is on a light background (e.g. almond cream card). */
  variant?: "default" | "onAccent";
  className?: string;
}

export function BotTag({ variant = "default", className }: BotTagProps) {
  const rootClass =
    variant === "onAccent"
      ? `${styles.botTag} ${styles.botTagOnAccent}`
      : styles.botTag;
  return (
    <span className={className ? `${rootClass} ${className}` : rootClass}>
      Bot
    </span>
  );
}
