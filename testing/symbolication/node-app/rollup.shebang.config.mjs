import terser from "@rollup/plugin-terser";
import { tracewayDebugIdsRollup } from "@tracewayapp/bundler-plugin";

export default {
  input: "src/index.js",
  external: (id) => !id.startsWith(".") && !id.startsWith("/"),
  output: {
    file: "dist-shebang/cli.mjs",
    format: "esm",
    sourcemap: "hidden",
    banner: "#!/usr/bin/env node",
  },
  plugins: [terser(), tracewayDebugIdsRollup()],
};
