import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { TracewayProvider } from "@tracewayapp/react";
import App from "./App.jsx";

// The provider sits outside StrictMode so development does not start two
// SDK clients and report every error twice. Without a token it stays off.
createRoot(document.getElementById("root")).render(
  <TracewayProvider token={import.meta.env.VITE_TRACEWAY_TOKEN} baseUrl={import.meta.env.VITE_TRACEWAY_URL}>
    <StrictMode>
      <App />
    </StrictMode>
  </TracewayProvider>
);
