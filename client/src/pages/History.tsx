import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import {
  RedirectToSignIn,
  SignedIn,
} from "@neondatabase/neon-js/auth/react/ui";
import { authClient } from "../lib/auth";
import styles from "../styles/History.module.css";

const NEON_AUTH_URL = import.meta.env.VITE_NEON_AUTH_URL ?? "";
const WS_URL = import.meta.env.VITE_WS_URL ?? "ws://localhost:8080/ws";

function apiBase(): string {
  const apiUrl = import.meta.env.VITE_API_URL;
  if (apiUrl) return apiUrl.replace(/\/$/, "");
  const base = WS_URL.replace(/^ws/, "http").replace(/\/ws\/?$/, "") || "http://localhost:8080";
  return base;
}

export interface GameRecord {
  id: string;
  played_at: string;
  game_id: string;
  player0_user_id: string;
  player1_user_id: string;
  player0_name: string;
  player1_name: string;
  player0_score: number;
  player1_score: number;
  winner_index: number | null;
  end_reason: string;
  your_index: number | null;
}

export function HistoryPage() {
  const [list, setList] = useState<GameRecord[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    async function fetchToken(): Promise<string | null> {
      if (!NEON_AUTH_URL) return null;
      const getSessionUrl = `${NEON_AUTH_URL.replace(/\/$/, "")}/get-session`;
      const res = await fetch(getSessionUrl, { credentials: "include" });
      const jwt =
        res.headers.get("set-auth-jwt") ?? res.headers.get("Set-Auth-Jwt");
      if (jwt) return jwt;
      const result = await authClient.token();
      return result.data?.token ?? null;
    }

    fetchToken()
      .then((token) => {
        if (cancelled) return;
        if (!token) {
          setError("Not signed in");
          setLoading(false);
          return;
        }
        const base = apiBase();
        return fetch(`${base}/api/history`, {
          headers: { Authorization: `Bearer ${token}` },
        });
      })
      .then((res) => {
        if (cancelled || !res) return;
        if (!res.ok) {
          if (res.status === 401) {
            setError("Please sign in again.");
            return;
          }
          throw new Error(res.statusText || "Failed to load history");
        }
        return res.json();
      })
      .then((data: GameRecord[] | undefined) => {
        if (cancelled) return;
        setList(Array.isArray(data) ? data : []);
        setError(null);
      })
      .catch((err) => {
        if (!cancelled) {
          setError(err?.message ?? "Failed to load history");
          setList([]);
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <>
      <SignedIn>
        <div className={styles.wrapper}>
          <h1 className={styles.title}>History</h1>
          <Link to="/" className={styles.backLink}>
            Back to lobby
          </Link>
          {loading && <p className={styles.status}>Loading...</p>}
          {error && <p className={styles.error}>{error}</p>}
          {!loading && !error && list.length === 0 && (
            <p className={styles.empty}>No games yet. Play a match to see history here.</p>
          )}
          {!loading && !error && list.length > 0 && (
            <ul className={styles.list}>
              {list.map((game) => (
                <GameHistoryItem key={game.id} record={game} />
              ))}
            </ul>
          )}
        </div>
      </SignedIn>
      <RedirectToSignIn />
    </>
  );
}

function GameHistoryItem({ record }: { record: GameRecord }) {
  const yourIdx = record.your_index ?? 0;
  const oppName = yourIdx === 0 ? record.player1_name : record.player0_name;
  const yourScore = yourIdx === 0 ? record.player0_score : record.player1_score;
  const oppScore = yourIdx === 0 ? record.player1_score : record.player0_score;
  const winnerIdx = record.winner_index;
  const youWon = winnerIdx !== null && winnerIdx === yourIdx;
  const youLost = winnerIdx !== null && winnerIdx !== yourIdx;
  const draw = winnerIdx === null;

  const dateStr = (() => {
    try {
      const d = new Date(record.played_at);
      return d.toLocaleDateString(undefined, {
        dateStyle: "medium",
        timeStyle: "short",
      });
    } catch {
      return record.played_at;
    }
  })();

  return (
    <li className={styles.item}>
      <p className={styles.date}>{dateStr}</p>
      <p className={styles.players}>
        <span className={youWon ? styles.winner : youLost ? styles.loser : undefined}>
          You: {yourScore}
        </span>
        {" vs "}
        <span className={youLost ? styles.winner : youWon ? styles.loser : undefined}>
          {oppName}: {oppScore}
        </span>
      </p>
      <p className={styles.result}>
        {draw && "Draw"}
        {youWon && "You won"}
        {youLost && "You lost"}
        {record.end_reason === "opponent_disconnected" && " (opponent left)"}
      </p>
    </li>
  );
}
