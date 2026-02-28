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
  player0_elo_before?: number | null;
  player1_elo_before?: number | null;
  player0_elo_after?: number | null;
  player1_elo_after?: number | null;
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
  const oppUserId = yourIdx === 0 ? record.player1_user_id : record.player0_user_id;
  const oppIsBot = oppUserId === "ai" || oppUserId.startsWith("ai:");
  const yourScore = yourIdx === 0 ? record.player0_score : record.player1_score;
  const oppScore = yourIdx === 0 ? record.player1_score : record.player0_score;
  const yourEloBefore = yourIdx === 0 ? record.player0_elo_before : record.player1_elo_before;
  const yourEloAfter = yourIdx === 0 ? record.player0_elo_after : record.player1_elo_after;
  const oppEloBefore = yourIdx === 0 ? record.player1_elo_before : record.player0_elo_before;
  const winnerIdx = record.winner_index;
  const isAbandoned = record.end_reason === "opponent_disconnected";
  const youAbandoned = isAbandoned && winnerIdx !== null && yourIdx === 1 - winnerIdx;
  const opponentAbandoned = isAbandoned && winnerIdx !== null && yourIdx === winnerIdx;
  const youWon = !isAbandoned && winnerIdx !== null && winnerIdx === yourIdx;
  const youLost = !isAbandoned && winnerIdx !== null && winnerIdx !== yourIdx;
  const draw = !isAbandoned && winnerIdx === null;

  const dateStr = (() => {
    try {
      const d = new Date(record.played_at);
      return d.toLocaleString(undefined, {
        dateStyle: "medium",
        timeStyle: "short",
      });
    } catch {
      return record.played_at;
    }
  })();

  return (
    <li className={styles.item}>
      <div className={styles.playersRow}>
        <div className={styles.playerBlock}>
          <div className={styles.playerNameRow}>
            <span className={[styles.playerName, (isAbandoned && !youAbandoned && !opponentAbandoned) && styles.abandoned, youWon && styles.winner, youLost && styles.loser, youAbandoned && styles.loser, opponentAbandoned && styles.winner].filter(Boolean).join(" ")}>
              You
            </span>
            {yourEloBefore != null && <span className={styles.playerElo}>({yourEloBefore})</span>}
          </div>
          <span className={[styles.playerScore, (isAbandoned && !youAbandoned && !opponentAbandoned) && styles.abandoned, youWon && styles.winner, youLost && styles.loser, youAbandoned && styles.loser, opponentAbandoned && styles.winner].filter(Boolean).join(" ")}>{yourScore}</span>
        </div>
        <span className={styles.vs}>vs</span>
        <div className={styles.playerBlock}>
          <div className={styles.playerNameRow}>
            <span className={[styles.playerName, (isAbandoned && !youAbandoned && !opponentAbandoned) && styles.abandoned, youLost && styles.winner, youWon && styles.loser, youAbandoned && styles.winner, opponentAbandoned && styles.loser].filter(Boolean).join(" ")}>
              {oppName}
            </span>
            {oppEloBefore != null && <span className={styles.playerElo}>({oppEloBefore})</span>}
            {oppIsBot && <span className={styles.botTag}>Bot</span>}
          </div>
          <span className={[styles.playerScore, (isAbandoned && !youAbandoned && !opponentAbandoned) && styles.abandoned, youLost && styles.winner, youWon && styles.loser, youAbandoned && styles.winner, opponentAbandoned && styles.loser].filter(Boolean).join(" ")}>{oppScore}</span>
        </div>
      </div>
      <p className={styles.date}>{dateStr}</p>
      <p className={styles.result}>
        {youAbandoned && "You abandoned (loss)"}
        {opponentAbandoned && "Opponent abandoned (victory)"}
        {isAbandoned && !youAbandoned && !opponentAbandoned && "Abandoned"}
        {!isAbandoned && draw && "Draw"}
        {!isAbandoned && youWon && "Victory!"}
        {!isAbandoned && youLost && "Defeat!"}
        {(youWon || youLost || youAbandoned || opponentAbandoned) && yourEloBefore != null && yourEloAfter != null && (
          <>
            {'  Rating: '}
            <span className={yourEloAfter >= yourEloBefore ? styles.resultRatingUp : styles.resultRatingDown}>
              {yourEloBefore}{' → '}{yourEloAfter}
            </span>
          </>
        )}
      </p>
    </li>
  );
}
