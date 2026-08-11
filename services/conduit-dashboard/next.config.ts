import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // A minimal, self-contained server bundle (its own node_modules subset,
  // no monorepo-wide install needed) — what the Dockerfile's runtime stage
  // actually copies, keeping the final image small.
  output: "standalone",
};

export default nextConfig;
