import type { Metadata } from "next";
import { brand } from "@/brand";

import { AuthScreen } from "@/components/auth/auth-screen";

export const metadata: Metadata = {
  title: `Sign in — ${brand.name}`,
  description: `Sign in to your ${brand.name} infrastructure monitoring dashboard.`,
};

export default function LoginPage() {
  return <AuthScreen initialMode="login" />;
}
