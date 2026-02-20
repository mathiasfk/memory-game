import { AuthView } from "@neondatabase/neon-js/auth/react/ui";
import { useParams } from "react-router-dom";
import { authClient } from "../lib/auth";
import styles from "../styles/Auth.module.css";

export function Auth() {
  const { pathname } = useParams<{ pathname: string }>();

  const handleGoogleSignIn = async () => {
    try {
      await authClient.signIn.social({
        provider: "google",
        callbackURL: window.location.origin,
      });
    } catch (error) {
      console.error("Google sign-in error:", error);
    }
  };

  return (
    <div className={styles.authPage}>
      <div className={styles.socialSection}>
        <button
          type="button"
          className={styles.googleButton}
          onClick={handleGoogleSignIn}
        >
          <span className={styles.googleIcon} aria-hidden>
            G
          </span>
          Entrar com Google
        </button>
      </div>
      <div className={styles.divider}>
        <span>ou</span>
      </div>
      <AuthView pathname={pathname ?? "sign-in"} />
    </div>
  );
}
