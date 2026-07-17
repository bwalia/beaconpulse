/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  // Standalone output produces a minimal self-contained server for the Docker image.
  output: "standalone",
  // The React Compiler memoizes components and values automatically, which is why
  // there is almost no hand-written memo/useCallback/useMemo in here: hand-memoizing
  // is easy to get subtly wrong (a stale dep freezes part of the UI — the one failure
  // a monitoring product cannot afford) and it rots as the code changes. The compiler
  // re-derives it from scratch every build instead.
  //
  // It is only sound because render is pure. It memoizes on the inputs it can SEE, so
  // anything reading a hidden one — Date.now() during render, most of all — risks
  // being frozen at its first value. Those reads go through the clock store in
  // lib/time.ts, and the react-hooks lint rules (purity, immutability,
  // set-state-in-effect) are what keep it true. Keep them passing.
  reactCompiler: true,
};

export default nextConfig;
