import type { NextConfig } from "next";

// standalone output is what the Docker image needs: only the traced runtime files are copied.
// It is opt-in because a host with its own Next.js adapter (Netlify, Vercel) builds its own
// output, and forcing standalone there produces a deploy that serves nothing.
const standalone = process.env.NEXT_OUTPUT === "standalone";

const nextConfig: NextConfig = {
  reactStrictMode: true,
  output: standalone ? "standalone" : undefined,
  poweredByHeader: false,
};

export default nextConfig;
