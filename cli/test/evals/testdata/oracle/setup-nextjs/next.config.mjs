/** @type {import('next').NextConfig} */
const nextConfig = {
  poweredByHeader: false,
  serverExternalPackages: ["@opentelemetry/auto-instrumentations-node"],
  experimental: { serverSourceMaps: true },
};

export default nextConfig;
