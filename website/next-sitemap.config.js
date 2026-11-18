/** @type {import('next-sitemap').IConfig} */
module.exports = {
  siteUrl: "https://tracewayapp.com",
  generateRobotsTxt: true,
  exclude: ["/product/session-replay"],
  outDir: "./out",
};
