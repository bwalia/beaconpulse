import type { Metadata } from "next";

import { AuthScreen } from "@/components/auth/auth-screen";

export const metadata: Metadata = {
  title: "Sign in — Beacon",
  description: "Sign in to your Beacon infrastructure monitoring dashboard.",
};

export default function LoginPage() {
  return <AuthScreen initialMode="login" />;
}
