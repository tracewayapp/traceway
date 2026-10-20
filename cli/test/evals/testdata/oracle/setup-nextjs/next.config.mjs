/** @type {import('next').NextConfig} */
const nextConfig = {
  poweredByHeader: false,
  serverExternalPackages: ["@opentelemetry/auto-instrumentations-node"],
};

export default nextConfig;
