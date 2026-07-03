"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useAuth } from "@/lib/auth";
import { ApiRequestError } from "@/lib/api";
import { Button, Card, Field, Input } from "@/components/ui";

const loginSchema = z.object({
  email: z.string().email("Enter a valid email"),
  password: z.string().min(1, "Password is required"),
});

const registerSchema = z.object({
  org_name: z.string().min(1, "Organization name is required"),
  name: z.string().min(1, "Your name is required"),
  email: z.string().email("Enter a valid email"),
  password: z.string().min(8, "At least 8 characters"),
});

type LoginValues = z.infer<typeof loginSchema>;
type RegisterValues = z.infer<typeof registerSchema>;

export default function LoginPage() {
  const [mode, setMode] = useState<"login" | "register">("login");

  return (
    <div className="flex min-h-screen items-center justify-center bg-gradient-to-b from-slate-50 to-slate-100 p-4 dark:from-slate-950 dark:to-slate-900">
      <div className="w-full max-w-md">
        <div className="mb-6 text-center">
          <div className="mx-auto mb-3 flex h-12 w-12 items-center justify-center rounded-xl bg-brand-600 text-2xl">
            🛰️
          </div>
          <h1 className="text-2xl font-bold">Beacon</h1>
          <p className="text-sm text-slate-500">Self-hosted infrastructure monitoring</p>
        </div>

        <Card>
          <div className="mb-4 flex rounded-lg bg-slate-100 p-1 text-sm dark:bg-slate-800">
            {(["login", "register"] as const).map((m) => (
              <button
                key={m}
                onClick={() => setMode(m)}
                className={`flex-1 rounded-md px-3 py-1.5 font-medium capitalize transition ${
                  mode === m
                    ? "bg-white text-slate-900 shadow-sm dark:bg-slate-700 dark:text-white"
                    : "text-slate-500"
                }`}
              >
                {m === "login" ? "Sign in" : "Create account"}
              </button>
            ))}
          </div>
          {mode === "login" ? <LoginForm /> : <RegisterForm />}
        </Card>
      </div>
    </div>
  );
}

function LoginForm() {
  const { login } = useAuth();
  const router = useRouter();
  const [serverError, setServerError] = useState<string | null>(null);
  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<LoginValues>({ resolver: zodResolver(loginSchema) });

  const onSubmit = async (values: LoginValues) => {
    setServerError(null);
    try {
      await login(values.email, values.password);
      router.replace("/monitors");
    } catch (err) {
      setServerError(err instanceof ApiRequestError ? err.message : "Something went wrong");
    }
  };

  return (
    <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
      <Field label="Email" error={errors.email?.message}>
        <Input type="email" placeholder="you@example.com" {...register("email")} />
      </Field>
      <Field label="Password" error={errors.password?.message}>
        <Input type="password" placeholder="••••••••" {...register("password")} />
      </Field>
      {serverError && <p className="text-sm text-red-600">{serverError}</p>}
      <Button type="submit" className="w-full" disabled={isSubmitting}>
        {isSubmitting ? "Signing in…" : "Sign in"}
      </Button>
    </form>
  );
}

function RegisterForm() {
  const { register: registerAccount } = useAuth();
  const router = useRouter();
  const [serverError, setServerError] = useState<string | null>(null);
  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<RegisterValues>({ resolver: zodResolver(registerSchema) });

  const onSubmit = async (values: RegisterValues) => {
    setServerError(null);
    try {
      await registerAccount(values);
      router.replace("/monitors");
    } catch (err) {
      setServerError(err instanceof ApiRequestError ? err.message : "Something went wrong");
    }
  };

  return (
    <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
      <Field label="Organization name" error={errors.org_name?.message}>
        <Input placeholder="Acme Inc." {...register("org_name")} />
      </Field>
      <Field label="Your name" error={errors.name?.message}>
        <Input placeholder="Jane Doe" {...register("name")} />
      </Field>
      <Field label="Email" error={errors.email?.message}>
        <Input type="email" placeholder="you@example.com" {...register("email")} />
      </Field>
      <Field label="Password" error={errors.password?.message}>
        <Input type="password" placeholder="At least 8 characters" {...register("password")} />
      </Field>
      {serverError && <p className="text-sm text-red-600">{serverError}</p>}
      <Button type="submit" className="w-full" disabled={isSubmitting}>
        {isSubmitting ? "Creating…" : "Create account"}
      </Button>
    </form>
  );
}
