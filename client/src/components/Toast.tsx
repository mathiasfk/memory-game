import { useEffect, useState } from "react";
import styles from "../styles/Toast.module.css";

const DEFAULT_TOAST_DURATION_MS = 5000;

export interface ToastItem {
  id: string;
  message: string;
  /** Duration in ms before auto-dismiss; default 5000. Use a larger value for persistent notices (e.g. reconnecting). */
  durationMs?: number;
  /** "error" shows "Error:" prefix and error styling; "info" neutral/warning; "success" green. Default "error". */
  variant?: "error" | "info" | "success";
  /** When set (e.g. opponent reconnecting), show countdown in toast. Unix ms. */
  reconnectionDeadlineUnixMs?: number;
}

function formatCountdown(seconds: number): string {
  const m = Math.floor(seconds / 60);
  const s = seconds % 60;
  return `${m}:${s.toString().padStart(2, "0")}`;
}

function getSecondsRemaining(deadlineUnixMs: number): number {
  return Math.max(0, Math.ceil((deadlineUnixMs - Date.now()) / 1000));
}

interface ToastProps {
  toast: ToastItem;
  onDismiss: (id: string) => void;
}

function Toast({ toast, onDismiss }: ToastProps) {
  const duration = toast.durationMs ?? DEFAULT_TOAST_DURATION_MS;
  const deadline = toast.reconnectionDeadlineUnixMs;
  const [secondsRemaining, setSecondsRemaining] = useState<number | null>(
    deadline != null && deadline > 0 ? getSecondsRemaining(deadline) : null,
  );

  useEffect(() => {
    const id = setTimeout(() => onDismiss(toast.id), duration);
    return () => clearTimeout(id);
  }, [toast.id, duration, onDismiss]);

  useEffect(() => {
    if (deadline == null || deadline <= 0) return;
    const update = () => setSecondsRemaining(getSecondsRemaining(deadline));
    update();
    const interval = setInterval(update, 1000);
    return () => clearInterval(interval);
  }, [deadline]);

  const isError = toast.variant === "error" || toast.variant === undefined;
  const className =
    isError ? styles.toast : toast.variant === "success" ? styles.toastSuccess : styles.toastInfo;

  const text =
    isError ? `Error: ${toast.message}` : toast.message;
  const countdown =
    secondsRemaining !== null ? ` (${formatCountdown(secondsRemaining)})` : "";

  return (
    <div className={className} role="alert">
      {text}{countdown}
    </div>
  );
}

interface ToastContainerProps {
  toasts: ToastItem[];
  onDismiss: (id: string) => void;
}

export function ToastContainer({ toasts, onDismiss }: ToastContainerProps) {
  if (toasts.length === 0) return null;
  return (
    <div className={styles.container} aria-live="polite">
      {toasts.map((t) => (
        <Toast key={t.id} toast={t} onDismiss={onDismiss} />
      ))}
    </div>
  );
}
