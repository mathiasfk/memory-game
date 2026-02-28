import { useNavigate } from "react-router-dom";
import styles from "../styles/LobbyScreen.module.css";

interface LobbyScreenProps {
  firstName: string;
  connected: boolean;
  authReady: boolean;
  isAdmin?: boolean;
  onFindMatch: () => void;
  onSignOut: () => void;
}

export default function LobbyScreen({
  firstName,
  connected,
  authReady,
  isAdmin = false,
  onFindMatch,
  onSignOut,
}: LobbyScreenProps) {
  const navigate = useNavigate();
  const canFindGame = connected && authReady;

  return (
    <section className={styles.screen}>
      <h1 className={styles.title}>Memory Game</h1>
      <p className={styles.welcome}>Welcome, {firstName}</p>
      <p className={styles.subtitle}>Click to enter the match queue.</p>
      <div className={styles.actions}>
        <button
          type="button"
          onClick={onFindMatch}
          disabled={!canFindGame}
          className={styles.primaryButton}
        >
          Find game
        </button>
        <button
          type="button"
          onClick={() => navigate("/history")}
          className={styles.historyButton}
        >
          History
        </button>
        <button
          type="button"
          onClick={() => navigate("/leaderboard")}
          className={styles.historyButton}
        >
          Leaderboard
        </button>
        {isAdmin && (
          <button
            type="button"
            onClick={() => navigate("/telemetry")}
            className={styles.historyButton}
          >
            Telemetry
          </button>
        )}
        <button type="button" onClick={onSignOut} className={styles.signOut}>
          Log out
        </button>
      </div>
      {!connected && (
        <p className={styles.connection}>Connecting to server...</p>
      )}
      {connected && !authReady && (
        <p className={styles.connection}>Authenticating...</p>
      )}
    </section>
  );
}
