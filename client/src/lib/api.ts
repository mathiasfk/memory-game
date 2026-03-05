import { authClient } from "./auth";
import type { GameRecord, HistoryResponse } from "../types/history";

const NEON_AUTH_URL = import.meta.env.VITE_NEON_AUTH_URL ?? "";
const WS_URL = import.meta.env.VITE_WS_URL ?? "ws://localhost:8080/ws";

export function getApiBase(): string {
  const apiUrl = import.meta.env.VITE_API_URL;
  if (apiUrl) return apiUrl.replace(/\/$/, "");
  const base = WS_URL.replace(/^ws/, "http").replace(/\/ws\/?$/, "") || "http://localhost:8080";
  return base;
}

export async function getAuthToken(): Promise<string | null> {
  if (!NEON_AUTH_URL) return null;
  const getSessionUrl = `${NEON_AUTH_URL.replace(/\/$/, "")}/get-session`;
  const res = await fetch(getSessionUrl, { credentials: "include" });
  const jwt =
    res.headers.get("set-auth-jwt") ?? res.headers.get("Set-Auth-Jwt");
  if (jwt) return jwt;
  const result = await authClient.token();
  return result.data?.token ?? null;
}

export async function fetchLastGame(): Promise<GameRecord | null> {
  const token = await getAuthToken();
  if (!token) return null;
  const base = getApiBase();
  const res = await fetch(`${base}/api/history?limit=1`, {
    headers: { Authorization: `Bearer ${token}` },
  });
  if (!res.ok) return null;
  const data: HistoryResponse = await res.json();
  return data.games?.[0] ?? null;
}
