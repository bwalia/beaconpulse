import { pwaIcon } from "@/lib/pwa-icon";

// iOS "Add to Home Screen" icon. Next links it as apple-touch-icon automatically.
export const size = { width: 180, height: 180 };
export const contentType = "image/png";

export default function AppleIcon() {
  return pwaIcon(180, 0.12);
}
