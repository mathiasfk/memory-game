import styles from "../styles/WaitingScreen.module.css";

interface WaitingScreenProps {
  connected: boolean;
  onCancel?: () => void;
}

export default function WaitingScreen({ connected, onCancel }: WaitingScreenProps) {
  return (
    <section className={styles.screen}>
      <div className={styles.spinner} aria-hidden="true" />
      <h2>Looking for an opponent...</h2>
      <p>{connected ? "Waiting in matchmaking queue." : "Reconnecting to server..."}</p>
      {onCancel && (
        <button
          type="button"
          className={styles.cancelBtn}
          onClick={onCancel}
          aria-label="Cancelar e voltar ao início"
        >
          Voltar
        </button>
      )}
    </section>
  );
}
