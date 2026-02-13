import { FormEvent, useMemo, useState } from "react";
import styles from "../styles/LobbyScreen.module.css";

const MAX_NAME_LENGTH = 24;

interface LobbyScreenProps {
  connected: boolean;
  onFindMatch: (name: string) => void;
}

export default function LobbyScreen({ connected, onFindMatch }: LobbyScreenProps): JSX.Element {
  const [name, setName] = useState("");

  const trimmedName = useMemo(() => name.trim(), [name]);
  const isValidName = trimmedName.length >= 1 && trimmedName.length <= MAX_NAME_LENGTH;

  const handleSubmit = (event: FormEvent<HTMLFormElement>): void => {
    event.preventDefault();
    if (!isValidName || !connected) {
      return;
    }
    onFindMatch(trimmedName);
  };

  return (
    <section className={styles.screen}>
      <h1 className={styles.title}>Memory Game</h1>
      <p className={styles.subtitle}>Set your display name to enter matchmaking.</p>
      <form className={styles.form} onSubmit={handleSubmit}>
        <label htmlFor="display-name">Display name</label>
        <input
          id="display-name"
          value={name}
          onChange={(event) => setName(event.target.value)}
          minLength={1}
          maxLength={MAX_NAME_LENGTH}
          placeholder="Your name"
          autoComplete="off"
          required
        />
        <button type="submit" disabled={!isValidName || !connected}>
          Find Match
        </button>
      </form>
      {!connected && <p className={styles.connection}>Connecting to server...</p>}
    </section>
  );
}
