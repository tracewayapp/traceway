import terser from "@rollup/plugin-terser";
import { tracewayDebugIdsRollup } from "@tracewayapp/bundler-plugin";

const plugins = [terser()];
if (process.env.TW_DEBUG_IDS) {
  plugins.push(tracewayDebugIdsRollup());
}

export default {
  input: "src/index.js",
  external: (id) => !id.startsWith(".") && !id.startsWith("/"),
  output: [
    { file: "dist/app.mjs", format: "esm", sourcemap: "hidden" },
    { file: "dist/app.cjs", format: "cjs", sourcemap: "hidden" },
  ],
  plugins,
};
