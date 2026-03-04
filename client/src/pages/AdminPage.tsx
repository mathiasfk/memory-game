import { useNavigate } from "react-router-dom";
import styles from "../styles/LobbyScreen.module.css";

export function AdminPage() {
  const navigate = useNavigate();

  return (
    <section className={styles.screen}>
      <h1 className={styles.title}>Admin Tools</h1>
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
          onClick={() => navigate("/")}
          className={styles.historyButton}
        >
          Back to lobby
        </button>
      </div>
    </section>
  );
}
