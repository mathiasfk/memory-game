import { Route, Routes } from "react-router-dom";
import { Account } from "./pages/Account";
import { Auth } from "./pages/Auth";
import { HistoryPage } from "./pages/History";
import { LeaderboardPage } from "./pages/Leaderboard";
import { TelemetryPage } from "./pages/Telemetry";
import { Home } from "./pages/Home";

export default function App() {
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
          <div className="appTheme">
            <HistoryPage />
          </div>
        }
      />
      <Route
        path="/leaderboard"
        element={
          <div className="appTheme">
            <LeaderboardPage />
          </div>
        }
      />
      <Route
        path="/telemetry"
        element={
          <div className="appTheme">
            <TelemetryPage />
          </div>
        }
      />
      <Route path="/auth/:pathname" element={<Auth />} />
      <Route path="/account/:pathname" element={<Account />} />
    </Routes>
  );
}
