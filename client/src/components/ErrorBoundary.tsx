import type { ErrorInfo, ReactNode } from "react";
import { Component } from "react";
import { reportFrontendErrorDedup } from "../lib/reportError";

export interface ErrorBoundaryProps {
  children: ReactNode;
  fallback?: ReactNode;
  onError?: (error: Error, errorInfo: ErrorInfo) => void;
}

interface ErrorBoundaryState {
  hasError: boolean;
  error: Error | null;
}

export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  constructor(props: ErrorBoundaryProps) {
    super(props);
    this.state = { hasError: false, error: null };
  }

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo): void {
    reportFrontendErrorDedup({
      message: error.message,
      stack: error.stack,
      componentStack: errorInfo.componentStack ?? undefined,
      url: typeof window !== "undefined" ? window.location.href : "",
      userAgent: typeof navigator !== "undefined" ? navigator.userAgent : "",
    });
    this.props.onError?.(error, errorInfo);
  }

  render(): ReactNode {
    if (this.state.hasError && this.state.error) {
      if (this.props.fallback) return this.props.fallback;
      return (
        <div
          className="appTheme"
          style={{
            minHeight: "100vh",
            display: "flex",
            flexDirection: "column",
            alignItems: "center",
            justifyContent: "center",
            padding: "1.5rem",
            fontFamily: "var(--font-body)",
            color: "var(--color-text)",
            textAlign: "center",
          }}
        >
          <h1 style={{ fontSize: "var(--text-xl)", marginBottom: "0.5rem" }}>
            Something went wrong
          </h1>
          <p style={{ color: "var(--color-text-muted)", marginBottom: "1.5rem" }}>
            The error has been reported. Try reloading the page.
          </p>
          <button
            type="button"
            onClick={() => window.location.reload()}
            style={{
              padding: "0.5rem 1rem",
              fontFamily: "var(--font-body)",
              fontSize: "var(--text-base)",
              backgroundColor: "var(--color-primary-accent)",
              color: "var(--color-primary-accent-fg)",
              border: "none",
              borderRadius: "4px",
              cursor: "pointer",
            }}
          >
            Reload
          </button>
        </div>
      );
    }
    return this.props.children;
  }
}
