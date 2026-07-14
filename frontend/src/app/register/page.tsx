import type { Metadata } from "next";

import { AuthScreen } from "@/components/auth/auth-screen";

// A real route rather than /login?mode=register: it prerenders with real content
// (no blank flash), it is a clean URL to point a campaign or ad at, and it can
// carry its own metadata.
export const metadata: Metadata = {
  title: "Start monitoring free — Beacon Pulse",
  description:
    "Create your Beacon Pulse account. Self-hosted, multi-tenant infrastructure monitoring. No credit card.",
};

export default function RegisterPage() {
  return <AuthScreen initialMode="register" />;
}
