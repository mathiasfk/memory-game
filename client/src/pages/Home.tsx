import {
  RedirectToSignIn,
  SignedIn,
} from "@neondatabase/neon-js/auth/react/ui";
import { GameShell } from "../components/GameShell";

export function Home() {
  return (
    <>
      <SignedIn>
        <GameShell />
      </SignedIn>
      <RedirectToSignIn />
    </>
  );
}
