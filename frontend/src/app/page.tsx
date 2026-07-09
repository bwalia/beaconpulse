"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/lib/auth";

export default function Home() {
  const { user, loading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (loading) return;
    router.replace(user ? "/monitors" : "/login");
  }, [user, loading, router]);

  return (
    <div className="flex h-screen items-center justify-center text-slate-500 dark:text-slate-400">
      <span className="motion-safe:animate-pulse">Loading Beacon…</span>
    </div>
  );
}
