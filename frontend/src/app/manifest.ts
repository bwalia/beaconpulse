import type { MetadataRoute } from "next";

import { brand } from "@/brand";

// The install manifest. Name, colours and icons all come from the active brand, so
// every white-label installs as itself. start_url is the dashboard: an installed
// app is for checking on monitors, not reading the landing page.
export default function manifest(): MetadataRoute.Manifest {
  return {
    id: "/",
    name: brand.name,
    short_name: brand.shortName,
    description: brand.description,
    start_url: "/dashboard",
    scope: "/",
    display: "standalone",
    background_color: "#020617",
    theme_color: brand.primary[600],
    icons: [
      { src: "/pwa-icon/192", sizes: "192x192", type: "image/png" },
      { src: "/pwa-icon/512", sizes: "512x512", type: "image/png" },
      { src: "/pwa-icon/maskable", sizes: "512x512", type: "image/png", purpose: "maskable" },
    ],
  };
}
