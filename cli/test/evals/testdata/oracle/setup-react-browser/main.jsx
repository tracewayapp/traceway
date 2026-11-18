import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { TracewayProvider } from "@tracewayapp/react";
import App from "./App.jsx";

const tracewayToken = import.meta.env.VITE_TRACEWAY_TOKEN;
const tracewayUrl = (import.meta.env.VITE_TRACEWAY_URL ?? "").replace(/\/+$/, "");

const app = (
  <StrictMode>
    <App />
  </StrictMode>
);

// The provider sits outside StrictMode so development does not start two
// SDK clients and report every error twice.
createRoot(document.getElementById("root")).render(
  tracewayToken && tracewayUrl ? (
    <TracewayProvider connectionString={`${tracewayToken}@${tracewayUrl}/api/report`}>{app}</TracewayProvider>
  ) : (
    app
  )
);
