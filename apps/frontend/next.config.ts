import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  reactStrictMode: true,
  // standalone keeps the production image small: only the traced runtime files are copied.
  output: "standalone",
  poweredByHeader: false,
};

export default nextConfig;
