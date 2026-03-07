import { Analytics } from "@vercel/analytics/react";
import { SpeedInsights } from "@vercel/speed-insights/react";
import { NeonAuthUIProvider } from "@neondatabase/neon-js/auth/react";
import "@neondatabase/neon-js/ui/css";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import App from "./App";
import { ErrorBoundary } from "./components/ErrorBoundary";
import { authClient } from "./lib/auth";
import { prefetchCardImages } from "./lib/prefetchCardImages";
import { registerGlobalErrorHandlers } from "./lib/reportError";
import "./styles/global.css";

prefetchCardImages();
registerGlobalErrorHandlers();

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <NeonAuthUIProvider authClient={authClient} social={{ providers: ["google"] }}>
      <BrowserRouter>
        <ErrorBoundary>
          <App />
          <Analytics />
          <SpeedInsights />
        </ErrorBoundary>
      </BrowserRouter>
    </NeonAuthUIProvider>
  </StrictMode>,
);
