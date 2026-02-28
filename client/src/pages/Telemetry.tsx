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
          setError("Acesso negado. Apenas administradores podem ver esta página.");
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
          <h1 className={styles.title}>Análise da telemetria</h1>
          <Link to="/" className={styles.backLink}>
            Voltar ao lobby
          </Link>
          {loading && <p className={styles.status}>Carregando...</p>}
          {error && <p className={styles.error}>{error}</p>}
          {!loading && !error && metrics && (
            <>
              <section className={styles.section}>
                <h2 className={styles.sectionTitle}>Jogadores</h2>
                <div className={styles.globalGrid}>
                  <div className={styles.globalCard}>
                    <span className={styles.globalLabel}>Jogadores registrados</span>
                    <span className={styles.globalValue}>{metrics.players?.registered_count ?? "—"}</span>
                  </div>
                  <div className={styles.globalCard}>
                    <span className={styles.globalLabel}>Ativos na última semana</span>
                    <span className={styles.globalValue}>{metrics.players?.active_last_week ?? "—"}</span>
                  </div>
                  <div className={styles.globalCard}>
                    <span className={styles.globalLabel}>Total de partidas</span>
                    <span className={styles.globalValue}>{metrics.players?.total_matches ?? metrics.global.total_matches ?? "—"}</span>
                  </div>
                </div>
              </section>

              <section className={styles.section}>
                <h2 className={styles.sectionTitle}>Métricas globais</h2>
                <div className={styles.globalGrid}>
                  <div className={styles.globalCard}>
                    <span className={styles.globalLabel}>Turnos médio por partida</span>
                    <span className={styles.globalValue}>
                      {metrics.global.total_matches > 0
                        ? metrics.global.avg_turns_per_match.toFixed(1)
                        : "—"}
                    </span>
                  </div>
                  <div className={styles.globalCard}>
                    <span className={styles.globalLabel}>Point swing médio por turno</span>
                    <span className={styles.globalValue}>
                      {metrics.global.total_turns > 0
                        ? metrics.global.avg_net_point_swing_per_turn.toFixed(2)
                        : "—"}
                    </span>
                  </div>
                  <div
                    className={styles.globalCard}
                    title="Variação do placar do momento do uso da carta até o fim do turno"
                  >
                    <span className={styles.globalLabel}>Point swing médio por carta</span>
                    <span className={styles.globalValue}>
                      {metrics.global.avg_net_point_swing_per_card != null
                        ? metrics.global.avg_net_point_swing_per_card.toFixed(2)
                        : "—"}
                    </span>
                  </div>
                  <div className={styles.globalCard}>
                    <span className={styles.globalLabel}>Cartas por turno (média)</span>
                    <span className={styles.globalValue}>
                      {metrics.global.cards_per_turn_avg.toFixed(2)}
                    </span>
                  </div>
                  <div className={styles.globalCard}>
                    <span className={styles.globalLabel}>Cartas por turno (máx)</span>
                    <span className={styles.globalValue}>{metrics.global.cards_per_turn_max}</span>
                  </div>
                </div>
              </section>

              <section className={styles.section}>
                <h2 className={styles.sectionTitle}>Métricas por carta</h2>
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
                <h2 className={styles.sectionTitle}>Métricas por combo</h2>
                <ul className={styles.comboList}>
                  {metrics.by_combo.length === 0 && (
                    <li className={styles.comboEmpty}>Nenhum combo com 6 cartas registrado.</li>
                  )}
                  {metrics.by_combo.map((combo, i) => (
                    <li key={combo.combo_key} className={styles.comboItem}>
                      <span className={styles.comboKey}>{combo.combo_key}</span>
                      <span className={styles.comboStats}>
                        {combo.total_matches} partidas, {combo.wins} vitórias (
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
  const icon = display?.icon ?? "?";

  return (
    <button
      type="button"
      className={styles.arcanaCard}
      onClick={onClick}
    >
      <div className={styles.arcanaCardHeader}>
        <span className={styles.arcanaCardIcon}>{icon}</span>
        <span className={styles.arcanaCardName}>{label}</span>
      </div>
      <div className={styles.arcanaCardStats}>
        <span className={styles.arcanaStat}>
          <span className={styles.arcanaStatLabel}>Win rate</span>{" "}
          {card.total_matches > 0 ? card.win_rate_pct.toFixed(1) : "—"}%
        </span>
        <span
          className={styles.arcanaStat}
          title="Variação líquida (jogador − oponente) do momento do uso da carta até o fim do turno; mesma métrica do global"
        >
          <span className={styles.arcanaStatLabel}>Point swing</span>{" "}
          {(card.avg_point_swing_player - card.avg_point_swing_opponent).toFixed(1)}
        </span>
        <span className={styles.arcanaStat}>
          <span className={styles.arcanaStatLabel}>Usos</span> {card.use_count}
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
          <button type="button" className={styles.modalClose} onClick={onClose} aria-label="Fechar">
            ×
          </button>
        </div>
        <p className={styles.modalDescription}>{description}</p>
        <div className={styles.modalMetrics}>
          <h3 className={styles.modalSubtitle}>Métricas principais</h3>
          <p className={styles.modalHelper}>
            Point swing = variação do placar do momento do uso até o fim do turno (impacto direto e indireto da carta). O valor líquido (jogador − oponente) é a mesma métrica do global.
          </p>
          <div className={styles.modalMetricsGrid}>
            <span className={styles.modalMetric}>
              Win rate: {card.total_matches > 0 ? card.win_rate_pct.toFixed(1) : "—"}%
            </span>
            <span className={styles.modalMetric}>
              Point swing líquido: {(card.avg_point_swing_player - card.avg_point_swing_opponent).toFixed(1)}
            </span>
            <span className={styles.modalMetric}>
              Point swing (jogador): {card.avg_point_swing_player.toFixed(1)}
            </span>
            <span className={styles.modalMetric}>
              Point swing (oponente): {card.avg_point_swing_opponent.toFixed(1)}
            </span>
            <span className={styles.modalMetric}>Usos: {card.use_count}</span>
          </div>
        </div>
        <div className={styles.modalExtra}>
          <h3 className={styles.modalSubtitle}>Turno em que a carta é usada</h3>
          {card.turn_histogram.length === 0 ? (
            <p className={styles.modalEmpty}>Sem dados.</p>
          ) : (
            <div className={styles.histogram}>
              {card.turn_histogram.map((b) => (
                <div key={b.round} className={styles.histogramBar}>
                  <span className={styles.histogramLabel}>Turno {b.round}</span>
                  <span className={styles.histogramCount}>{b.count}</span>
                </div>
              ))}
            </div>
          )}
        </div>
        <div className={styles.modalExtra}>
          <h3 className={styles.modalSubtitle}>Matches após uso</h3>
          <p className={styles.modalText}>
            Média de pares já feitos no momento do uso:{" "}
            {card.use_count > 0 ? card.avg_pairs_matched_before.toFixed(1) : "—"}
          </p>
        </div>
      </div>
    </div>
  );
}
