import { defineConfig } from "vite";
import { tracewayDebugIdsVite } from "@tracewayapp/bundler-plugin";

export default defineConfig({
  build: {
    sourcemap: "hidden",
    minify: "esbuild",
  },
  plugins: process.env.TW_DEBUG_IDS ? [tracewayDebugIdsVite()] : [],
});
