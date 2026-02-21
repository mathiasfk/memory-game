import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import {
  RedirectToSignIn,
  SignedIn,
} from "@neondatabase/neon-js/auth/react/ui";
import { authClient } from "../lib/auth";
import styles from "../styles/Leaderboard.module.css";

const NEON_AUTH_URL = import.meta.env.VITE_NEON_AUTH_URL ?? "";
const WS_URL = import.meta.env.VITE_WS_URL ?? "ws://localhost:8080/ws";
const LEADERBOARD_TOP = Number(import.meta.env.VITE_LEADERBOARD_TOP_COUNT) || 20;

function apiBase(): string {
  const apiUrl = import.meta.env.VITE_API_URL;
  if (apiUrl) return apiUrl.replace(/\/$/, "");
  const base = WS_URL.replace(/^ws/, "http").replace(/\/ws\/?$/, "") || "http://localhost:8080";
  return base;
}

export interface LeaderboardEntry {
  user_id: string;
  display_name: string;
  elo: number;
  wins: number;
  losses: number;
  draws: number;
  is_bot: boolean;
  is_current_user?: boolean;
}

interface LeaderboardResponse {
  entries: LeaderboardEntry[];
  current_user_entry: LeaderboardEntry | null;
}

function LeaderboardCard({
  entry,
  rank,
  isYou,
}: {
  entry: LeaderboardEntry;
  rank: number;
  isYou?: boolean;
}) {
  const name = entry.display_name?.trim() || "—";
  return (
    <li className={isYou ? `${styles.item} ${styles.yourPositionCard}` : styles.item}>
      <div className={styles.cardHeader}>
        <span className={styles.rank}>{rank > 0 ? `#${rank}` : "—"}</span>
        <span className={styles.playerName}>
          <span className={isYou ? styles.youHighlight : undefined}>
            {isYou ? "You" : name}
          </span>
          {entry.is_bot && <span className={styles.botTag}>Bot</span>}
        </span>
      </div>
      <div className={styles.cardStats}>
        <span className={styles.stat}><span className={styles.statLabel}>Elo</span> {entry.elo}</span>
        <span className={styles.stat}><span className={styles.statLabel}>Wins</span> {entry.wins}</span>
        <span className={styles.stat}><span className={styles.statLabel}>Losses</span> {entry.losses}</span>
      </div>
    </li>
  );
}

export function LeaderboardPage() {
  const [data, setData] = useState<LeaderboardResponse | null>(null);
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
        const base = apiBase();
        const headers: HeadersInit = {};
        if (token) headers.Authorization = `Bearer ${token}`;
        return fetch(`${base}/api/leaderboard?limit=${LEADERBOARD_TOP}`, {
          headers,
        });
      })
      .then((res) => {
        if (cancelled || !res) return;
        if (!res.ok) {
          if (res.status === 401) {
            setError("Please sign in again.");
            return;
          }
          throw new Error(res.statusText || "Failed to load leaderboard");
        }
        return res.json();
      })
      .then((json: LeaderboardResponse | undefined) => {
        if (cancelled) return;
        if (json && Array.isArray(json.entries)) {
          setData({
            entries: json.entries,
            current_user_entry: json.current_user_entry ?? null,
          });
          setError(null);
        } else {
          setData({ entries: [], current_user_entry: null });
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setError(err?.message ?? "Failed to load leaderboard");
          setData(null);
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, []);

  const showCurrentUser =
    data?.current_user_entry &&
    !data.entries.some((e) => e.user_id === data.current_user_entry!.user_id);

  return (
    <>
      <SignedIn>
        <div className={styles.wrapper}>
          <h1 className={styles.title}>Leaderboard</h1>
          <Link to="/" className={styles.backLink}>
            Back to lobby
          </Link>
          {loading && <p className={styles.status}>Loading...</p>}
          {error && <p className={styles.error}>{error}</p>}
          {!loading && !error && data && (
            <>
              {data.entries.length === 0 && !showCurrentUser && (
                <p className={styles.empty}>No ratings yet. Play matches to see the leaderboard.</p>
              )}
              {(data.entries.length > 0 || showCurrentUser) && (
                <>
                  <ul className={styles.list}>
                    {data.entries.map((entry, i) => (
                      <LeaderboardCard
                        key={entry.user_id}
                        entry={entry}
                        rank={i + 1}
                        isYou={entry.is_current_user}
                      />
                    ))}
                  </ul>
                  {showCurrentUser && data.current_user_entry && (
                    <>
                      <p className={styles.yourPositionSep}>Your position</p>
                      <ul className={styles.list}>
                        <LeaderboardCard
                          entry={data.current_user_entry}
                          rank={-1}
                          isYou
                        />
                      </ul>
                    </>
                  )}
                </>
              )}
            </>
          )}
        </div>
      </SignedIn>
      <RedirectToSignIn />
    </>
  );
}
