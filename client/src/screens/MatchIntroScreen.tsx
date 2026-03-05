import { BotTag } from "../components/BotTag";
import styles from "../styles/MatchIntroScreen.module.css";

interface MatchIntroScreenProps {
  yourName: string;
  opponentName: string;
  yourElo?: number;
  opponentElo?: number;
  opponentUserId?: string;
}

function AvatarPlaceholder() {
  return (
    <div className={styles.avatarPlaceholder} aria-hidden="true">
      <svg
        xmlns="http://www.w3.org/2000/svg"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2" />
        <circle cx="12" cy="7" r="4" />
      </svg>
    </div>
  );
}

function formatRating(elo: number | undefined): string {
  if (elo === undefined) return "—";
  return String(elo);
}

export default function MatchIntroScreen({
  yourName,
  opponentName,
  yourElo,
  opponentElo,
  opponentUserId,
}: MatchIntroScreenProps) {
  const opponentIsBot =
    opponentUserId === "ai" || (opponentUserId?.startsWith("ai:") ?? false);

  return (
    <section
      className={styles.screen}
      aria-label="Match found - opponents"
      role="region"
    >
      <h2 className={styles.title}>Match found</h2>
      <div className={styles.versus}>
        <div className={styles.playerCard}>
          <AvatarPlaceholder />
          <span className={styles.playerName}>{yourName}</span>
          <p className={styles.rating}>
            Rating: <span className={styles.ratingValue}>{formatRating(yourElo)}</span>
          </p>
        </div>
        <span className={styles.vsDivider} aria-hidden="true">
          VS
        </span>
        <div className={styles.playerCard}>
          <AvatarPlaceholder />
          <div className={styles.nameRow}>
            <span className={styles.playerName}>{opponentName}</span>
            {opponentIsBot && <BotTag />}
          </div>
          <p className={styles.rating}>
            Rating: <span className={styles.ratingValue}>{formatRating(opponentElo)}</span>
          </p>
        </div>
      </div>
    </section>
  );
}
