import { ImageResponse } from "next/og";

import { brand } from "@/brand";

// A brand-aware social card, generated at request time from the active brand — so every
// white-label gets its own name, tagline and accent with no per-brand image asset to
// maintain. Next wires this into openGraph.images and twitter.images for the whole app
// segment automatically. nodejs runtime because the app ships as a standalone server.
export const runtime = "nodejs";
export const alt = `${brand.name} — infrastructure monitoring`;
export const size = { width: 1200, height: 630 };
export const contentType = "image/png";

export default function OpengraphImage() {
  const accent = brand.primary[500];
  return new ImageResponse(
    (
      <div
        style={{
          width: "100%",
          height: "100%",
          display: "flex",
          flexDirection: "column",
          justifyContent: "space-between",
          background: "#020617",
          color: "white",
          padding: "80px",
          fontFamily: "sans-serif",
        }}
      >
        <div style={{ display: "flex", alignItems: "center", gap: "24px" }}>
          <div
            style={{
              width: "56px",
              height: "56px",
              borderRadius: "16px",
              background: accent,
              display: "flex",
            }}
          />
          <div style={{ fontSize: "44px", fontWeight: 700 }}>{brand.name}</div>
        </div>
        <div style={{ display: "flex", flexDirection: "column", gap: "24px" }}>
          <div style={{ fontSize: "76px", fontWeight: 700, lineHeight: 1.05, maxWidth: "900px" }}>
            {brand.tagline}
          </div>
          <div style={{ fontSize: "34px", color: "#94a3b8", maxWidth: "1000px", lineHeight: 1.3 }}>
            {brand.description}
          </div>
        </div>
        <div style={{ display: "flex", alignItems: "center", gap: "16px", fontSize: "28px", color: accent }}>
          <div style={{ width: "16px", height: "16px", borderRadius: "9999px", background: accent, display: "flex" }} />
          Uptime · latency · SSL · DNS · status pages
        </div>
      </div>
    ),
    size,
  );
}
