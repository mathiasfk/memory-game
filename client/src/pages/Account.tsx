import { AccountView } from "@neondatabase/neon-js/auth/react/ui";
import { useParams } from "react-router-dom";

export function Account() {
  const { pathname } = useParams<{ pathname: string }>();
  return <AccountView pathname={pathname ?? "profile"} />;
}
