import type { ReactNode } from "react";

import { departureMono } from "@/app/fonts/departure";

// Scopes the Departure Mono pixel font to the public status route (page + its 404)
// via a CSS variable the terminal styles read. Kept off the rest of the app.
export default function StatusLayout({ children }: { children: ReactNode }) {
  return <div className={departureMono.variable}>{children}</div>;
}
