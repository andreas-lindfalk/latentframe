import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Serve /renders + /reels unoptimized so the ?v= cache-bust on regenerated files
  // works (Next's image optimizer rejects a query on local images). Full quality;
  // re-enable optimization + versioned filenames in the pre-public perf pass.
  images: { unoptimized: true },
};

export default nextConfig;
