/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  // Standalone output produces a minimal self-contained server for the Docker image.
  output: "standalone",
};

export default nextConfig;
