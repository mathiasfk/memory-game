import { Link, useNavigate } from "react-router-dom";
import styles from "../styles/LobbyScreen.module.css";

export function AdminPage() {
  const navigate = useNavigate();

  return (
    <section className={styles.screen}>
      <h1 className={styles.title}>Admin Tools</h1>
      <Link to="/" className={styles.backLink}>
        Back to lobby
      </Link>
      <p className={styles.subtitle}>Manage and inspect game data.</p>
      <div className={styles.actions}>
        <button
          type="button"
          onClick={() => navigate("/admin/telemetry")}
          className={styles.historyButton}
        >
          Telemetry
        </button>
        <button
          type="button"
          onClick={() => navigate("/admin/preview-cards")}
          className={styles.historyButton}
        >
          Preview Cards
        </button>
      </div>
    </section>
  );
}
