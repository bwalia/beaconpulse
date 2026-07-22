# Branding

The whole product's identity — name, colours, logo, docs host — lives in this folder.
Nothing elsewhere hardcodes it, so shipping the same product under a different brand is
editing one file and rebuilding, not hunting a word through the codebase.

## How it fits together

- **`types.ts`** — the `Brand` contract every brand fills in.
- **`beacon.ts`** — the default brand (Beacon Pulse).
- **`sysops.ts`** — a complete second brand, kept as a working example.
- **`index.ts`** — picks the active brand from `NEXT_PUBLIC_BRAND` (default `beacon`),
  and turns its colour ramp into the CSS variables the whole `brand-*` palette reads.

Colours are wired as CSS variables: `tailwind.config.ts` points `brand-*` at
`--brand-*`, and the root layout fills those from the active brand. So one ramp re-tints
every `brand-*` class in the product — buttons, links, focus rings, the logo — with no
per-class change.

## Add a brand

1. Copy `beacon.ts` to `yourbrand.ts` and change the values — name, tagline, the 50–900
   colour ramp, and the `Mark` SVG.
2. Register it in `index.ts`:
   ```ts
   import { yourbrand } from "./yourbrand";
   const BRANDS = { beacon, sysops, yourbrand };
   ```
3. Build it:
   ```bash
   NEXT_PUBLIC_BRAND=yourbrand npm run build
   ```

That build is a fully branded artifact. To ship two brands from this codebase, build
twice with different `NEXT_PUBLIC_BRAND` values — two images, one source tree. An
unknown or unset value falls back to `beacon`, so a typo degrades to the default rather
than a blank product.

## What this does NOT cover

Backend user-facing text (email subjects, the Stripe line item, health strings) still
says "Beacon" — that's a separate, backend-side change. This folder is the frontend:
everything a user sees in the browser.
