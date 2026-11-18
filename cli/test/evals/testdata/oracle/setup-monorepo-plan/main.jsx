import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { TracewayProvider } from "@tracewayapp/react";
import App from "./App.jsx";

const connectionString = import.meta.env.VITE_TRACEWAY_CONNECTION_STRING;

const app = (
  <StrictMode>
    <App />
  </StrictMode>
);

createRoot(document.getElementById("root")).render(
  connectionString ? <TracewayProvider connectionString={connectionString}>{app}</TracewayProvider> : app
);
