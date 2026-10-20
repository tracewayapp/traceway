import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { tracewayDebugIds } from "@tracewayapp/bundler-plugin/vite";

export default defineConfig({
  build: { sourcemap: true },
  plugins: [react(), tracewayDebugIds()],
});
