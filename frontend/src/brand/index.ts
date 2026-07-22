import { beacon } from "./beacon";
import { sysops } from "./sysops";
import type { Brand } from "./types";

export type { Brand } from "./types";

// Every brand the build knows about. Add a file, import it, add it here — the one edit
// beyond writing the brand itself.
const BRANDS: Record<string, Brand> = {
  beacon,
  sysops,
};

// Selected at BUILD time by NEXT_PUBLIC_BRAND. Next inlines NEXT_PUBLIC_* into the
// bundle, so this resolves to a single brand with no runtime branching and no way for
// one brand's assets to leak into another's build. Unknown or unset falls back to
// beacon, so a typo degrades to the default rather than a blank product.
const selected = process.env.NEXT_PUBLIC_BRAND?.toLowerCase() ?? "beacon";

/** The active brand. Everything user-facing reads from this. */
export const brand: Brand = BRANDS[selected] ?? beacon;

// hexToChannels turns "#328cff" into "50 140 255" — space-separated RGB channels with
// no rgb() wrapper, which is the exact form Tailwind needs to keep opacity modifiers
// working: `rgb(var(--brand-500) / <alpha-value>)` lets `bg-brand-500/50` still mean
// half-opacity. Commas or a wrapped rgb() would silently break every `/opacity` class.
function hexToChannels(hex: string): string {
  const h = hex.replace("#", "");
  const full = h.length === 3 ? h.split("").map((c) => c + c).join("") : h;
  const r = parseInt(full.slice(0, 2), 16);
  const g = parseInt(full.slice(2, 4), 16);
  const b = parseInt(full.slice(4, 6), 16);
  return `${r} ${g} ${b}`;
}

/**
 * brandCSSVariables renders the active brand's ramp as a `:root { --brand-N: r g b }`
 * block, injected once in the root layout before paint.
 *
 * This is the hinge the whole feature turns on: the Tailwind `brand-*` palette points at
 * these variables, so defining them here re-tints every `brand-*` class in the product
 * from one place. No component references a hex value, and nothing has to be rebuilt per
 * class — the colour lives in exactly one CSS rule.
 */
export function brandCSSVariables(): string {
  const lines = Object.entries(brand.primary)
    .map(([shade, hex]) => `--brand-${shade}: ${hexToChannels(hex)};`)
    .join("");
  return `:root{${lines}}`;
}
