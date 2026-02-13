import { useCallback, useEffect, useRef, useState } from "react";
import type { ClientMessage, ServerMessage } from "../types/messages";

interface UseGameSocketOptions {
  /** Called for every server message so none are skipped by React batching. */
  onMessage?: (msg: ServerMessage) => void;
}

interface UseGameSocket {
  connected: boolean;
  send: (msg: ClientMessage) => void;
  lastMessage: ServerMessage | null;
}

const MESSAGE_TYPES: ReadonlySet<ServerMessage["type"]> = new Set([
  "error",
  "waiting_for_match",
  "match_found",
  "game_state",
  "game_over",
  "opponent_disconnected",
]);

const BASE_RECONNECT_DELAY_MS = 500;
const MAX_RECONNECT_DELAY_MS = 5000;

function isServerMessage(value: unknown): value is ServerMessage {
  if (typeof value !== "object" || value === null) {
    return false;
  }

  const maybeType = (value as { type?: unknown }).type;
  return typeof maybeType === "string" && MESSAGE_TYPES.has(maybeType as ServerMessage["type"]);
}

export function useGameSocket(url: string, options: UseGameSocketOptions = {}): UseGameSocket {
  const { onMessage: onMessageOption } = options;
  const [connected, setConnected] = useState(false);
  const [lastMessage, setLastMessage] = useState<ServerMessage | null>(null);

  const socketRef = useRef<WebSocket | null>(null);
  const reconnectTimerRef = useRef<number | null>(null);
  const reconnectAttemptsRef = useRef(0);
  const shouldReconnectRef = useRef(true);
  const onMessageRef = useRef(onMessageOption);
  onMessageRef.current = onMessageOption;

  const clearReconnectTimer = useCallback((): void => {
    if (reconnectTimerRef.current !== null) {
      window.clearTimeout(reconnectTimerRef.current);
      reconnectTimerRef.current = null;
    }
  }, []);

  const connect = useCallback((): void => {
    const socket = new WebSocket(url);
    socketRef.current = socket;

    socket.onopen = () => {
      setConnected(true);
      reconnectAttemptsRef.current = 0;
      clearReconnectTimer();
    };

    socket.onmessage = (event) => {
      try {
        const parsed: unknown = JSON.parse(event.data as string);
        if (isServerMessage(parsed)) {
          setLastMessage(parsed);
          onMessageRef.current?.(parsed);
        }
      } catch (_error) {
        // Ignore invalid JSON payloads.
      }
    };

    socket.onerror = () => {
      socket.close();
    };

    socket.onclose = () => {
      setConnected(false);
      socketRef.current = null;

      if (!shouldReconnectRef.current) {
        return;
      }

      const backoffDelay = Math.min(
        MAX_RECONNECT_DELAY_MS,
        BASE_RECONNECT_DELAY_MS * 2 ** reconnectAttemptsRef.current,
      );
      reconnectAttemptsRef.current += 1;
      reconnectTimerRef.current = window.setTimeout(() => {
        connect();
      }, backoffDelay);
    };
  }, [clearReconnectTimer, url]);

  useEffect(() => {
    shouldReconnectRef.current = true;
    // Defer connect so React Strict Mode's double-mount runs cleanup before connect runs.
    // First mount: schedule connect(0); cleanup clears timer and closes socket; second mount: schedule connect(0).
    // Only the second connect runs, so we get one connection per tab instead of connect→close→connect.
    const connectTimer = window.setTimeout(() => {
      connect();
    }, 0);

    return () => {
      window.clearTimeout(connectTimer);
      shouldReconnectRef.current = false;
      clearReconnectTimer();
      socketRef.current?.close();
      socketRef.current = null;
    };
  }, [clearReconnectTimer, connect]);

  const send = useCallback((msg: ClientMessage): void => {
    if (socketRef.current?.readyState !== WebSocket.OPEN) {
      return;
    }

    socketRef.current.send(JSON.stringify(msg));
  }, []);

  return {
    connected,
    send,
    lastMessage,
  };
}
