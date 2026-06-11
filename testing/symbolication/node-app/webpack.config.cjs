const path = require("path");
const { TracewayDebugIdsWebpackPlugin } = require("@tracewayapp/bundler-plugin");

module.exports = {
  mode: "production",
  target: "node",
  entry: "./src/index.js",
  devtool: "hidden-source-map",
  output: {
    path: path.resolve(__dirname, "dist-webpack"),
    filename: "app.webpack.cjs",
    library: { type: "commonjs2" },
  },
  externals: ({ request }, callback) => {
    if (request && !request.startsWith(".") && !request.startsWith("/")) {
      return callback(null, "node-commonjs " + request);
    }
    callback();
  },
  plugins: [new TracewayDebugIdsWebpackPlugin()],
};
