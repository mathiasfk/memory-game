import { Route, Routes } from "react-router-dom";
import { Account } from "./pages/Account";
import { Auth } from "./pages/Auth";
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
      <Route path="/auth/:pathname" element={<Auth />} />
      <Route path="/account/:pathname" element={<Account />} />
    </Routes>
  );
}
