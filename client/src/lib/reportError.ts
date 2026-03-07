import { getApiBase } from "./api";

export interface FrontendErrorPayload {
  message: string;
  stack?: string;
  componentStack?: string;
  url?: string;
  userAgent?: string;
  timestamp?: string;
  userId?: string;
  [key: string]: unknown;
}

const ENDPOINT = "/api/log/frontend-error";

/**
 * Sends a frontend error report to the backend for structured logging (Grafana).
 * Uses fetch with sendBeacon fallback. Does not throw.
 */
export function reportFrontendError(payload: FrontendErrorPayload): void {
  const body: FrontendErrorPayload = {
    ...payload,
    url: payload.url ?? (typeof window !== "undefined" ? window.location.href : ""),
    userAgent: payload.userAgent ?? (typeof navigator !== "undefined" ? navigator.userAgent : ""),
    timestamp: payload.timestamp ?? new Date().toISOString(),
  };

  if (!body.message) return;

  const url = `${getApiBase()}${ENDPOINT}`;
  const blob = new Blob([JSON.stringify(body)], { type: "application/json" });

  try {
    if (navigator.sendBeacon) {
      const sent = navigator.sendBeacon(url, blob);
      if (sent) return;
    }
  } catch {
    // ignore beacon errors
  }

  try {
    fetch(url, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
      keepalive: true,
    }).catch(() => {
      // avoid unhandled rejection; no retry
    });
  } catch {
    // avoid throwing from report
  }
}

const recentlyReported = new Set<string>();
const RECENT_MS = 2000;
const MAX_RECENT = 50;

function wasRecentlyReported(key: string): boolean {
  return recentlyReported.has(key);
}

function markReported(key: string): void {
  recentlyReported.add(key);
  if (recentlyReported.size > MAX_RECENT) {
    const first = recentlyReported.values().next().value;
    if (first !== undefined) recentlyReported.delete(first);
  }
  setTimeout(() => recentlyReported.delete(key), RECENT_MS);
}

/**
 * Wrapper that deduplicates by message so the same error is not sent twice
 * (e.g. when both Error Boundary and window.onerror fire).
 */
export function reportFrontendErrorDedup(payload: FrontendErrorPayload): void {
  const key = `${payload.message ?? ""}\n${payload.stack ?? ""}`.slice(0, 500);
  if (wasRecentlyReported(key)) return;
  markReported(key);
  reportFrontendError(payload);
}

/**
 * Registers global handlers for uncaught errors and unhandled rejections.
 * Call once after app mount (e.g. in main.tsx). Errors already caught by
 * ErrorBoundary may also trigger window.onerror; use reportFrontendErrorDedup
 * in the boundary to avoid duplicate reports.
 */
export function registerGlobalErrorHandlers(): void {
  if (typeof window === "undefined") return;

  window.addEventListener("error", (event: ErrorEvent) => {
    const payload: FrontendErrorPayload = {
      message: event.message ?? String(event.error),
      stack: event.error instanceof Error ? event.error.stack : undefined,
      url: event.filename ?? window.location.href,
      userAgent: navigator.userAgent,
    };
    reportFrontendErrorDedup(payload);
  });

  window.addEventListener("unhandledrejection", (event: PromiseRejectionEvent) => {
    const reason = event.reason;
    const message =
      reason instanceof Error ? reason.message : String(reason);
    const stack = reason instanceof Error ? reason.stack : undefined;
    reportFrontendErrorDedup({ message, stack, url: window.location.href, userAgent: navigator.userAgent });
  });
}
