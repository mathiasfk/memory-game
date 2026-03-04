import { AuthView } from "@neondatabase/neon-js/auth/react/ui";
import { useParams } from "react-router-dom";
import styles from "../styles/Auth.module.css";

export function Auth() {
  const { pathname } = useParams<{ pathname: string }>();

  return (
    <div className={styles.authPage}>
      <AuthView pathname={pathname ?? "sign-in"} />
    </div>
  );
}
