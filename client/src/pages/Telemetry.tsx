import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import {
  RedirectToSignIn,
  SignedIn,
} from "@neondatabase/neon-js/auth/react/ui";
import { authClient } from "../lib/auth";
import { POWER_UP_DISPLAY } from "../powerups/registry";
import styles from "../styles/Telemetry.module.css";

const NEON_AUTH_URL = import.meta.env.VITE_NEON_AUTH_URL ?? "";
const WS_URL = import.meta.env.VITE_WS_URL ?? "ws://localhost:8080/ws";

function apiBase(): string {
  const apiUrl = import.meta.env.VITE_API_URL;
  if (apiUrl) return apiUrl.replace(/\/$/, "");
  const base = WS_URL.replace(/^ws/, "http").replace(/\/ws\/?$/, "") || "http://localhost:8080";
  return base;
}

export interface TelemetryTurnBucket {
  round: number;
  count: number;
}

export interface TelemetryByCard {
  power_up_id: string;
  total_matches: number;
  wins_with_card: number;
  win_rate_pct: number;
  use_count: number;
  avg_point_swing_player: number;
  avg_point_swing_opponent: number;
  avg_pairs_matched_before: number;
  turn_histogram: TelemetryTurnBucket[];
}

export interface TelemetryByCombo {
  combo_key: string;
  card_count: number;
  total_matches: number;
  wins: number;
  win_rate_pct: number;
}

export interface TelemetryPlayers {
  registered_count: number;
  active_last_week: number;
  total_matches: number;
}

export interface TelemetryGlobal {
  total_matches: number;
  total_turns: number;
  avg_turns_per_match: number;
  avg_net_point_swing_per_turn: number;
  avg_net_point_swing_per_card: number | null;
  cards_per_turn_avg: number;
  cards_per_turn_max: number;
}

export interface TelemetryMetrics {
  players?: TelemetryPlayers;
  global: TelemetryGlobal;
  by_card: TelemetryByCard[];
  by_combo: TelemetryByCombo[];
}

export function TelemetryPage() {
  const [metrics, setMetrics] = useState<TelemetryMetrics | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedCard, setSelectedCard] = useState<TelemetryByCard | null>(null);

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
        return fetch(`${base}/api/telemetry/metrics`, {
          headers: { Authorization: `Bearer ${token}` },
        });
      })
      .then((res) => {
        if (cancelled || !res) return;
        if (res.status === 403) {
          setError("Access denied. Only administrators can view this page.");
          setLoading(false);
          return;
        }
        if (!res.ok) {
          if (res.status === 401) {
            setError("Please sign in again.");
            return;
          }
          throw new Error(res.statusText || "Failed to load metrics");
        }
        return res.json();
      })
      .then((data: TelemetryMetrics | undefined) => {
        if (cancelled) return;
        setMetrics(data ?? null);
        setError(null);
      })
      .catch((err) => {
        if (!cancelled) {
          setError(err?.message ?? "Failed to load metrics");
          setMetrics(null);
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
          <h1 className={styles.title}>Telemetry analysis</h1>
          <Link to="/" className={styles.backLink}>
            Back to lobby
          </Link>
          {loading && <p className={styles.status}>Loading...</p>}
          {error && <p className={styles.error}>{error}</p>}
          {!loading && !error && metrics && (
            <>
              <section className={styles.section}>
                <h2 className={styles.sectionTitle}>Players</h2>
                <div className={styles.globalGrid}>
                  <div className={styles.globalCard}>
                    <span className={styles.globalLabel}>Registered players</span>
                    <span className={styles.globalValue}>{metrics.players?.registered_count ?? "—"}</span>
                  </div>
                  <div className={styles.globalCard}>
                    <span className={styles.globalLabel}>Active in the last week</span>
                    <span className={styles.globalValue}>{metrics.players?.active_last_week ?? "—"}</span>
                  </div>
                  <div className={styles.globalCard}>
                    <span className={styles.globalLabel}>Total matches</span>
                    <span className={styles.globalValue}>{metrics.players?.total_matches ?? metrics.global.total_matches ?? "—"}</span>
                  </div>
                </div>
              </section>

              <section className={styles.section}>
                <h2 className={styles.sectionTitle}>Global metrics</h2>
                <div className={styles.globalGrid}>
                  <div className={styles.globalCard}>
                    <span className={styles.globalLabel}>Avg turns per match</span>
                    <span className={styles.globalValue}>
                      {metrics.global.total_matches > 0
                        ? metrics.global.avg_turns_per_match.toFixed(1)
                        : "—"}
                    </span>
                  </div>
                  <div className={styles.globalCard}>
                    <span className={styles.globalLabel}>Avg point swing per turn</span>
                    <span className={styles.globalValue}>
                      {metrics.global.total_turns > 0
                        ? metrics.global.avg_net_point_swing_per_turn.toFixed(2)
                        : "—"}
                    </span>
                  </div>
                  <div
                    className={styles.globalCard}
                    title="Score change from card use until end of turn"
                  >
                    <span className={styles.globalLabel}>Avg point swing per card</span>
                    <span className={styles.globalValue}>
                      {metrics.global.avg_net_point_swing_per_card != null
                        ? metrics.global.avg_net_point_swing_per_card.toFixed(2)
                        : "—"}
                    </span>
                  </div>
                  <div className={styles.globalCard}>
                    <span className={styles.globalLabel}>Cards per turn (avg)</span>
                    <span className={styles.globalValue}>
                      {metrics.global.cards_per_turn_avg.toFixed(2)}
                    </span>
                  </div>
                  <div className={styles.globalCard}>
                    <span className={styles.globalLabel}>Cards per turn (max)</span>
                    <span className={styles.globalValue}>{metrics.global.cards_per_turn_max}</span>
                  </div>
                </div>
              </section>

              <section className={styles.section}>
                <h2 className={styles.sectionTitle}>Metrics by card</h2>
                <div className={styles.cardGrid}>
                  {metrics.by_card.map((card) => (
                    <ArcanaCard
                      key={card.power_up_id}
                      card={card}
                      onClick={() => setSelectedCard(card)}
                    />
                  ))}
                </div>
              </section>

              <section className={styles.section}>
                <h2 className={styles.sectionTitle}>Metrics by combo</h2>
                <p className={styles.comboHelp}>
                  Combos of 2+ cards played together in the same turn (synergy).
                </p>
                <ul className={styles.comboList}>
                  {metrics.by_combo.length === 0 && (
                    <li className={styles.comboEmpty}>
                      No multi-card combo recorded (need at least 2 cards used in the same turn).
                    </li>
                  )}
                  {metrics.by_combo.map((combo) => (
                    <li key={combo.combo_key} className={styles.comboItem}>
                      <div className={styles.comboCardArts}>
                        {combo.combo_key.split(",").map((powerUpId, idx) => {
                          const id = powerUpId.trim();
                          const display = POWER_UP_DISPLAY[id];
                          return display?.imagePath ? (
                            <img
                              key={`${id}-${idx}`}
                              className={styles.comboCardArt}
                              src={display.imagePath}
                              alt=""
                              title={display.label}
                              aria-hidden
                            />
                          ) : null;
                        })}
                      </div>
                      <span className={styles.comboKey}>
                        {combo.combo_key}
                        <span className={styles.comboCardCount}> ({combo.card_count} cards)</span>
                      </span>
                      <span className={styles.comboStats}>
                        {combo.total_matches} uses, {combo.wins} wins (
                        {combo.win_rate_pct.toFixed(1)}%)
                      </span>
                    </li>
                  ))}
                </ul>
              </section>
            </>
          )}
        </div>

        {selectedCard && (
          <ArcanaDetailModal
            card={selectedCard}
            onClose={() => setSelectedCard(null)}
          />
        )}
      </SignedIn>
      <RedirectToSignIn />
    </>
  );
}

function ArcanaCard({
  card,
  onClick,
}: {
  card: TelemetryByCard;
  onClick: () => void;
}) {
  const display = POWER_UP_DISPLAY[card.power_up_id];
  const label = display?.label ?? card.power_up_id;

  return (
    <button
      type="button"
      className={styles.arcanaCard}
      onClick={onClick}
    >
      {display?.imagePath && (
        <img
          className={styles.arcanaCardArt}
          src={display.imagePath}
          alt=""
          aria-hidden
        />
      )}
      <div className={styles.arcanaCardHeader}>
        <span className={styles.arcanaCardName}>{label}</span>
      </div>
      <div className={styles.arcanaCardStats}>
        <span className={styles.arcanaStat}>
          <span className={styles.arcanaStatLabel}>Win rate</span>{" "}
          {card.total_matches > 0 ? card.win_rate_pct.toFixed(1) : "—"}%
        </span>
        <span
          className={styles.arcanaStat}
          title="Net change (player − opponent) from card use until end of turn"
        >
          <span className={styles.arcanaStatLabel}>Point swing</span>{" "}
          {(card.avg_point_swing_player - card.avg_point_swing_opponent).toFixed(1)}
        </span>
        <span className={styles.arcanaStat}>
          <span className={styles.arcanaStatLabel}>Uses</span> {card.use_count}
        </span>
      </div>
    </button>
  );
}

function ArcanaDetailModal({
  card,
  onClose,
}: {
  card: TelemetryByCard;
  onClose: () => void;
}) {
  const display = POWER_UP_DISPLAY[card.power_up_id];
  const label = display?.label ?? card.power_up_id;
  const description = display?.description ?? "—";

  return (
    <div className={styles.modalOverlay} onClick={onClose} role="dialog" aria-modal="true" aria-labelledby="arcana-modal-title">
      <div className={styles.modal} onClick={(e) => e.stopPropagation()}>
        <div className={styles.modalHeader}>
          <h2 id="arcana-modal-title" className={styles.modalTitle}>{label}</h2>
          <button type="button" className={styles.modalClose} onClick={onClose} aria-label="Close">
            ×
          </button>
        </div>
        {display?.imagePath && (
          <img
            className={styles.modalCardArt}
            src={display.imagePath}
            alt=""
            aria-hidden
          />
        )}
        <p className={styles.modalDescription}>{description}</p>
        <div className={styles.modalMetrics}>
          <h3 className={styles.modalSubtitle}>Main metrics</h3>
          <p className={styles.modalHelper}>
            Point swing = score change from card use until end of turn (direct and indirect impact of the card).
          </p>
          <div className={styles.modalMetricsGrid}>
            <span className={styles.modalMetric}>
              Win rate: {card.total_matches > 0 ? card.win_rate_pct.toFixed(1) : "—"}%
            </span>
            <span className={styles.modalMetric}>
              Net point swing: {(card.avg_point_swing_player - card.avg_point_swing_opponent).toFixed(1)}
            </span>
            <span className={styles.modalMetric}>
              Point swing (player): {card.avg_point_swing_player.toFixed(1)}
            </span>
            <span className={styles.modalMetric}>
              Point swing (opponent): {card.avg_point_swing_opponent.toFixed(1)}
            </span>
            <span className={styles.modalMetric}>Uses: {card.use_count}</span>
          </div>
        </div>
        <div className={styles.modalExtra}>
          <h3 className={styles.modalSubtitle}>Game stage at use</h3>
          <p className={styles.modalText}>
            <strong>Turn (histogram):</strong>
          </p>
          {card.turn_histogram.length === 0 ? (
            <p className={styles.modalEmpty}>No data.</p>
          ) : (
            <div className={styles.histogram}>
              {card.turn_histogram.map((b) => (
                <div key={b.round} className={styles.histogramBar}>
                  <span className={styles.histogramLabel}>Turn {b.round}</span>
                  <span className={styles.histogramCount}>{b.count}</span>
                </div>
              ))}
            </div>
          )}
          <p className={styles.modalText}>
            <strong>Pairs already matched:</strong> {card.use_count > 0 ? card.avg_pairs_matched_before.toFixed(1) : "—"} on average
          </p>
        </div>
      </div>
    </div>
  );
}
