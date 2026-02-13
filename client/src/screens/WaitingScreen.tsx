import styles from "../styles/WaitingScreen.module.css";

interface WaitingScreenProps {
  connected: boolean;
}

export default function WaitingScreen({ connected }: WaitingScreenProps) {
  return (
    <section className={styles.screen}>
      <div className={styles.spinner} aria-hidden="true" />
      <h2>Looking for an opponent...</h2>
      <p>{connected ? "Waiting in matchmaking queue." : "Reconnecting to server..."}</p>
    </section>
  );
}
