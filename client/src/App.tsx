import { Route, Routes } from "react-router-dom";
import { Account } from "./pages/Account";
import { Auth } from "./pages/Auth";
import { HistoryPage } from "./pages/History";
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
      <Route path="/auth/:pathname" element={<Auth />} />
      <Route path="/account/:pathname" element={<Account />} />
    </Routes>
  );
}
