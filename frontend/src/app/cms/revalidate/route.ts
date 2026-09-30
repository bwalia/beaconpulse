import crypto from "node:crypto";

import { revalidateTag } from "next/cache";
import { headers } from "next/headers";

import { CMS_CACHE_TAG } from "@/lib/cms";

// force-dynamic is required: under `output: standalone` + Next 16 a statically rendered
// POST handler is invoked with an EMPTY header set, so every signed delivery would fail
// the signature check. Reading the signature through next/headers keeps it per-request.
export const dynamic = "force-dynamic";

/**
 * OpsAPI CMS webhook receiver.
 *
 * Lives at /cms/revalidate rather than under /api/ because the gateway routes /api/* to
 * the Go API. OpsAPI POSTs a signed payload on a cms_post change; we verify the
 * HMAC-SHA256 signature (`X-Opsapi-Signature-256`) against BEACON_CMS_REVALIDATE_SECRET
 * — the same secret registered on the OpsAPI webhook for this namespace — then bust the
 * `cms-posts` tag so the next request re-fetches. Content goes live in seconds, no
 * rebuild.
 */
export async function POST(req: Request) {
  const secret = process.env.BEACON_CMS_REVALIDATE_SECRET;
  if (!secret) {
    return Response.json({ ok: false, error: "revalidate secret not configured" }, { status: 503 });
  }

  let raw: string;
  try {
    raw = await req.text();
  } catch {
    return Response.json({ ok: false, error: "could not read request body" }, { status: 400 });
  }
  const provided = (await headers()).get("x-opsapi-signature-256") || "";
  const expected = "sha256=" + crypto.createHmac("sha256", secret).update(raw).digest("hex");

  // Constant-time compare; timingSafeEqual throws on a length mismatch, so guard.
  const a = Buffer.from(provided);
  const b = Buffer.from(expected);
  if (a.length !== b.length || !crypto.timingSafeEqual(a, b)) {
    return Response.json({ ok: false }, { status: 401 });
  }

  // expire: 0 — drop the cached entries now, so the very next request sees the edit
  // (a "max" profile would serve the stale copy once more first).
  revalidateTag(CMS_CACHE_TAG, { expire: 0 });
  return Response.json({ revalidated: true, at: new Date().toISOString() });
}
