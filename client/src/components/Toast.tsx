import { useEffect } from "react";
import styles from "../styles/Toast.module.css";

const TOAST_DURATION_MS = 5000;

export interface ToastItem {
  id: string;
  message: string;
}

interface ToastProps {
  toast: ToastItem;
  onDismiss: (id: string) => void;
}

function Toast({ toast, onDismiss }: ToastProps) {
  useEffect(() => {
    const id = setTimeout(() => onDismiss(toast.id), TOAST_DURATION_MS);
    return () => clearTimeout(id);
  }, [toast.id, onDismiss]);

  return (
    <div className={styles.toast} role="alert">
      Error: {toast.message}
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
