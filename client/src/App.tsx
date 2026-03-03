import { Route, Routes, useLocation } from "react-router-dom";
import { Account } from "./pages/Account";
import { Auth } from "./pages/Auth";
import { HistoryPage } from "./pages/History";
import { LeaderboardPage } from "./pages/Leaderboard";
import { TelemetryPage } from "./pages/Telemetry";
import { Home } from "./pages/Home";

const LIST_PAGE_PATHS = ["/history", "/leaderboard", "/telemetry"];

export default function App() {
  const { pathname } = useLocation();
  const isListPage = LIST_PAGE_PATHS.includes(pathname);
  const appThemeClass = isListPage ? "appTheme appTheme--listPage" : "appTheme";

  return (
    <Routes>
      <Route
        path="/"
        element={
          <div className="appTheme">
            <Home />
          </div>
        }
      />
      <Route
        path="/history"
        element={
          <div className={appThemeClass}>
            <HistoryPage />
          </div>
        }
      />
      <Route
        path="/leaderboard"
        element={
          <div className={appThemeClass}>
            <LeaderboardPage />
          </div>
        }
      />
      <Route
        path="/telemetry"
        element={
          <div className={appThemeClass}>
            <TelemetryPage />
          </div>
        }
      />
      <Route path="/auth/:pathname" element={<Auth />} />
      <Route path="/account/:pathname" element={<Account />} />
    </Routes>
  );
}
