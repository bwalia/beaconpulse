import { ImageResponse } from "next/og";
import { cloneElement } from "react";

import { brand } from "@/brand";

/**
 * A home-screen / install icon rendered from the active brand: its mark in white on
 * its accent colour. Generated, so every white-label installs with its own icon and
 * there is no per-brand PNG to keep in sync. `inset` is the margin around the mark
 * as a fraction of the size — maskable icons need more (the OS crops to a circle).
 */
export function pwaIcon(px: number, inset: number) {
  const markPx = Math.round(px * (1 - 2 * inset));
  const mark = cloneElement(brand.Mark({}), { width: markPx, height: markPx, stroke: "white" } as object);
  return new ImageResponse(
    (
      <div
        style={{
          width: "100%",
          height: "100%",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          background: brand.primary[600],
          color: "white",
        }}
      >
        {mark}
      </div>
    ),
    { width: px, height: px },
  );
}
