import { useCallback, useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import {
  RedirectToSignIn,
  SignedIn,
} from "@neondatabase/neon-js/auth/react/ui";
import { BotTag } from "../components/BotTag";
import { getApiBase, getAuthToken } from "../lib/api";
import { reportFrontendError } from "../lib/reportError";
import type { GameRecord, HistoryResponse } from "../types/history";
import styles from "../styles/History.module.css";

const HISTORY_PAGE_SIZE = 10;

export function HistoryPage() {
  const [list, setList] = useState<GameRecord[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [hasMore, setHasMore] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const sentinelRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    let cancelled = false;

    getAuthToken()
      .then((token) => {
        if (cancelled) return;
        if (!token) {
          setError("Not signed in");
          setLoading(false);
          return;
        }
        const base = getApiBase();
        return fetch(
          `${base}/api/history?limit=${HISTORY_PAGE_SIZE}&offset=0`,
          { headers: { Authorization: `Bearer ${token}` } }
        );
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
      .then((data: HistoryResponse | undefined) => {
        if (cancelled) return;
        const games = data?.games ?? [];
        setList(Array.isArray(games) ? games : []);
        setHasMore(Boolean(data?.has_more));
        setError(null);
      })
      .catch((err) => {
        if (!cancelled) {
          const message = err?.message ?? "Failed to load history";
          setError(message);
          setList([]);
          setHasMore(false);
          reportFrontendError({
            message: "History load failed",
            context: "api/history",
            errorDetail: message,
          });
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, []);

  const loadMore = useCallback(async () => {
    if (!hasMore || loadingMore) return;
    const token = await getAuthToken();
    if (!token) return;
    setLoadingMore(true);
    const base = getApiBase();
    try {
      const res = await fetch(
        `${base}/api/history?limit=${HISTORY_PAGE_SIZE}&offset=${list.length}`,
        { headers: { Authorization: `Bearer ${token}` } }
      );
      if (!res.ok) throw new Error(res.statusText || "Failed to load history");
      const data: HistoryResponse = await res.json();
      const games = data?.games ?? [];
      setList((prev) => [...prev, ...(Array.isArray(games) ? games : [])]);
      setHasMore(Boolean(data?.has_more));
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to load more";
      setError(message);
      reportFrontendError({
        message: "History load more failed",
        context: "api/history pagination",
        errorDetail: message,
      });
    } finally {
      setLoadingMore(false);
    }
  }, [hasMore, loadingMore, list.length]);

  useEffect(() => {
    const sentinel = sentinelRef.current;
    if (!sentinel || !hasMore || loadingMore || list.length === 0) return;

    const observer = new IntersectionObserver(
      (entries) => {
        if (!entries[0]?.isIntersecting) return;
        loadMore();
      },
      { rootMargin: "200px", threshold: 0 }
    );
    observer.observe(sentinel);
    return () => observer.disconnect();
  }, [hasMore, loadingMore, list.length, loadMore]);

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
            <>
              <ul className={styles.list}>
                {list.map((game) => (
                  <GameHistoryItem key={game.id} record={game} />
                ))}
              </ul>
              <div ref={sentinelRef} className={styles.sentinel} aria-hidden />
              {loadingMore && (
                <p className={styles.loadingMore}>Loading more…</p>
              )}
            </>
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
            {oppIsBot && <BotTag />}
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
