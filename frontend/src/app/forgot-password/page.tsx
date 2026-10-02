import type { Metadata } from "next";

import { brand } from "@/brand";
import { AuthCard } from "@/components/auth/auth-card";
import { ForgotPasswordForm } from "@/components/auth/password-reset-forms";

export const metadata: Metadata = {
  title: "Forgot password",
  description: `Reset the password for your ${brand.name} account.`,
  robots: { index: false, follow: true },
};

export default function ForgotPasswordPage() {
  return (
    <AuthCard title="Forgot your password?" subtitle="Enter your account email and we'll send you a link to choose a new one.">
      <ForgotPasswordForm />
    </AuthCard>
  );
}
