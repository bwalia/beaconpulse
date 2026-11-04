import { brand } from "@/brand";

// The site → app half of the Android app link check (see brand.android). Static: the
// brand is fixed at build time.
export const dynamic = "force-static";

export function GET() {
  const app = brand.android;
  if (!app?.sha256.length) return new Response("Not found", { status: 404 });
  return Response.json([
    {
      relation: ["delegate_permission/common.handle_all_urls"],
      target: { namespace: "android_app", package_name: app.package, sha256_cert_fingerprints: app.sha256 },
    },
  ]);
}
