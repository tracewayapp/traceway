module.exports = {
  apps: [
    {
      name: "devtesting-nestjs",
      script: "dist/main.js",
      instances: 4,
      exec_mode: "cluster",
    },
  ],
};
