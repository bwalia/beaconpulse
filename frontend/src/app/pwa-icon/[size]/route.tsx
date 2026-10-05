import { pwaIcon } from "@/lib/pwa-icon";

// The install icons the manifest lists. Prerendered at build (force-static), so
// serving one is a file read, not an image render.
const VARIANTS: Record<string, { px: number; inset: number }> = {
  "192": { px: 192, inset: 0.1 },
  "512": { px: 512, inset: 0.1 },
  maskable: { px: 512, inset: 0.18 },
};

export const dynamic = "force-static";

export function generateStaticParams() {
  return Object.keys(VARIANTS).map((size) => ({ size }));
}

export async function GET(_req: Request, { params }: { params: Promise<{ size: string }> }) {
  const v = VARIANTS[(await params).size];
  if (!v) return new Response("Not found", { status: 404 });
  return pwaIcon(v.px, v.inset);
}
