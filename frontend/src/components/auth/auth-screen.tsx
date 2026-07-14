"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { motion, AnimatePresence } from "framer-motion";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { forwardRef, useEffect, useId, useState, type InputHTMLAttributes } from "react";
import { useForm, type UseFormRegisterReturn } from "react-hook-form";
import { z } from "zod";

import {
  ArrowRightIcon,
  BeaconMark,
  CheckCircleIcon,
  EyeIcon,
  EyeOffIcon,
  LockIcon,
} from "@/components/icons";
import { Spotlight } from "@/components/marketing/pointer";
import { Button, Field } from "@/components/ui";
import { ApiRequestError } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { DUR, EASE_OUT, useRevealVariants, useStaggerVariants } from "@/lib/motion";
import { ThemeToggle } from "@/lib/theme";

const loginSchema = z.object({
  email: z.string().email("Enter a valid email address"),
  password: z.string().min(1, "Enter your password"),
});

const registerSchema = z.object({
  org_name: z.string().min(1, "Give your organization a name"),
  name: z.string().min(1, "Enter your name"),
  email: z.string().email("Enter a valid email address"),
  password: z.string().min(8, "Use at least 8 characters"),
});

type LoginValues = z.infer<typeof loginSchema>;
type RegisterValues = z.infer<typeof registerSchema>;
type Mode = "login" | "register";

const inputBase =
  "w-full rounded-xl border border-slate-300 bg-white px-4 py-3.5 text-lg text-slate-900 placeholder:text-slate-400 focus:border-blue-600 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 disabled:opacity-50 aria-[invalid=true]:border-red-500 dark:border-slate-700 dark:bg-slate-900 dark:text-white dark:placeholder:text-slate-500";

/**
 * Shared input. 44px+ tall (touch target) and 16px+ text (no iOS auto-zoom).
 *
 * MUST forward its ref. react-hook-form's register() returns a `ref`, and React
 * does NOT pass refs to plain function components — it silently drops them. With
 * the ref lost, RHF has no handle on the DOM node, reads `undefined` on submit,
 * and zod reports its default "Required" even though the field visibly contains
 * text. It breaks browser autofill hardest of all, since autofill never fires
 * React's onChange, so the DOM value is the ONLY place the value exists.
 */
const TextInput = forwardRef<HTMLInputElement, InputHTMLAttributes<HTMLInputElement>>(
  function TextInput({ className = "", ...props }, ref) {
    return <input ref={ref} className={`${inputBase} ${className}`} {...props} />;
  },
);

/**
 * Password input with a show/hide toggle.
 *
 * A masked field with no reveal is a well-known source of failed logins — people
 * mistype and cannot see why. The toggle is a real button (focusable, labelled),
 * not an icon glued on top, and its label announces the ACTION not the state.
 */
function PasswordInput({
  register,
  autoComplete,
  placeholder,
  ...field
}: {
  // Typed as the real RHF return value rather than a loose record: that keeps the
  // `ref` in the type, so it cannot be quietly dropped the way TextInput's was.
  register: UseFormRegisterReturn;
  autoComplete: "current-password" | "new-password";
  placeholder: string;
} & InputHTMLAttributes<HTMLInputElement>) {
  const [shown, setShown] = useState(false);
  return (
    <div className="relative">
      {/* `field` carries the id / aria-invalid / aria-describedby that <Field>
          clones onto its child. Without spreading them onto the real <input>,
          the <label htmlFor> would point at nothing and the field would be
          unlabelled for screen readers. */}
      <input
        {...field}
        {...register}
        type={shown ? "text" : "password"}
        autoComplete={autoComplete}
        placeholder={placeholder}
        className={`${inputBase} pr-14`}
      />
      <button
        type="button"
        onClick={() => setShown((v) => !v)}
        aria-label={shown ? "Hide password" : "Show password"}
        className="absolute right-2 top-1/2 grid h-10 w-10 -translate-y-1/2 place-items-center rounded-lg text-slate-500 transition-colors hover:text-slate-900 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 motion-reduce:transition-none dark:text-slate-400 dark:hover:text-white"
      >
        {shown ? <EyeOffIcon className="h-5 w-5" /> : <EyeIcon className="h-5 w-5" />}
      </button>
    </div>
  );
}

/** Server error banner. role=alert so a screen reader announces it immediately. */
function ServerError({ message }: { message: string | null }) {
  return (
    <AnimatePresence>
      {message && (
        <motion.p
          role="alert"
          initial={{ opacity: 0, y: -6 }}
          animate={{ opacity: 1, y: 0 }}
          exit={{ opacity: 0, y: -6 }}
          transition={{ duration: DUR.micro, ease: EASE_OUT }}
          className="rounded-xl bg-red-500/10 px-4 py-3 text-base font-medium text-red-700 dark:text-red-400"
        >
          {message}
        </motion.p>
      )}
    </AnimatePresence>
  );
}

function LoginForm() {
  const { login } = useAuth();
  const router = useRouter();
  const [serverError, setServerError] = useState<string | null>(null);
  const {
    register,
    handleSubmit,
    setFocus,
    formState: { errors, isSubmitting },
  } = useForm<LoginValues>({ resolver: zodResolver(loginSchema), mode: "onBlur" });

  const onSubmit = async (values: LoginValues) => {
    setServerError(null);
    try {
      await login(values.email, values.password);
      router.replace("/dashboard");
    } catch (err) {
      setServerError(err instanceof ApiRequestError ? err.message : "Something went wrong. Try again.");
      // Send focus back to the field they will need to correct, rather than
      // leaving it on a disabled button they cannot see the error from.
      setFocus("password");
    }
  };

  return (
    <form onSubmit={handleSubmit(onSubmit)} className="space-y-5" noValidate>
      <ServerError message={serverError} />
      <Field label="Email" error={errors.email?.message}>
        <TextInput type="email" inputMode="email" autoComplete="email" placeholder="you@company.com" {...register("email")} />
      </Field>
      <Field label="Password" error={errors.password?.message}>
        <PasswordInput register={register("password")} autoComplete="current-password" placeholder="Your password" />
      </Field>
      <Button type="submit" size="lg" className="w-full text-lg" disabled={isSubmitting}>
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
    setFocus,
    formState: { errors, isSubmitting },
  } = useForm<RegisterValues>({ resolver: zodResolver(registerSchema), mode: "onBlur" });

  const onSubmit = async (values: RegisterValues) => {
    setServerError(null);
    try {
      await registerAccount(values);
      router.replace("/dashboard");
    } catch (err) {
      setServerError(err instanceof ApiRequestError ? err.message : "Something went wrong. Try again.");
      setFocus("email");
    }
  };

  return (
    <form onSubmit={handleSubmit(onSubmit)} className="space-y-5" noValidate>
      <ServerError message={serverError} />
      <Field label="Organization name" error={errors.org_name?.message} hint="Your team or company. You can rename it later.">
        <TextInput autoComplete="organization" placeholder="Acme Inc." {...register("org_name")} />
      </Field>
      <Field label="Your name" error={errors.name?.message}>
        <TextInput autoComplete="name" placeholder="Jane Doe" {...register("name")} />
      </Field>
      <Field label="Email" error={errors.email?.message}>
        <TextInput type="email" inputMode="email" autoComplete="email" placeholder="you@company.com" {...register("email")} />
      </Field>
      <Field label="Password" error={errors.password?.message} hint="At least 8 characters.">
        <PasswordInput register={register("password")} autoComplete="new-password" placeholder="Create a password" />
      </Field>
      <Button type="submit" size="lg" className="w-full text-lg" disabled={isSubmitting}>
        {isSubmitting ? "Creating your account…" : "Create account"}
      </Button>
    </form>
  );
}

const SELLING_POINTS = [
  "Twelve monitor types — HTTP, DNS, SSL, TCP, Kubernetes and more",
  "Alerts that reach a human, with AI incident summaries",
  "Public status pages your customers actually trust",
];

/**
 * The auth screen, in either mode.
 *
 * `initialMode` is a PROP, not a query param read with useSearchParams. That
 * matters: useSearchParams forces the whole route to bail out of prerendering
 * (Next emits BAILOUT_TO_CLIENT_SIDE_RENDERING), so every visitor would get a
 * blank white screen until hydration. With two real routes — /login and
 * /register — both are statically prerendered with real content, there is no
 * flash, and /register is a proper URL you can point a campaign at.
 *
 * Switching tabs still animates client-side and rewrites the URL, so the
 * interaction is unchanged; only the entry point is now static.
 */
export function AuthScreen({ initialMode }: { initialMode: Mode }) {
  const [mode, setMode] = useState<Mode>(initialMode);
  const groupId = useId();

  const reveal = useRevealVariants();
  const stagger = useStaggerVariants(0.07);

  // Keep the URL honest as the user switches, without a navigation (which would
  // re-mount the form and lose anything they had typed).
  useEffect(() => {
    window.history.replaceState(null, "", mode === "register" ? "/register" : "/login");
  }, [mode]);

  return (
    <div className="grid min-h-dvh lg:grid-cols-[1.1fr_1fr]">
      {/* ---- Brand panel. Hidden on small screens: on a phone the form IS the
              page, and a decorative half would just push it below the fold. ---- */}
      <aside className="relative hidden overflow-hidden bg-slate-950 p-12 text-white lg:flex lg:flex-col lg:justify-between xl:p-16">
        <Spotlight />
        <div
          aria-hidden
          className="pointer-events-none absolute inset-0 bg-[linear-gradient(to_right,rgba(148,163,184,0.08)_1px,transparent_1px),linear-gradient(to_bottom,rgba(148,163,184,0.08)_1px,transparent_1px)] bg-[size:56px_56px] [mask-image:radial-gradient(ellipse_at_center,black_30%,transparent_75%)]"
        />
        <div
          aria-hidden
          className="pointer-events-none absolute -left-32 top-1/3 h-[460px] w-[460px] rounded-full bg-blue-500/20 blur-3xl"
        />

        <Link
          href="/"
          className="relative inline-flex items-center gap-3 rounded-lg focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-400"
        >
          <BeaconMark className="h-9 w-9 text-blue-400" />
          <span className="text-2xl font-semibold tracking-tight">Beacon Pulse</span>
        </Link>

        <motion.div initial="hidden" animate="show" variants={stagger} className="relative max-w-xl">
          <motion.h1
            variants={reveal}
            className="text-balance text-5xl font-semibold leading-[1.1] tracking-tight xl:text-6xl"
          >
            Know it&apos;s down
            <br />
            <span className="bg-gradient-to-r from-blue-400 to-emerald-400 bg-clip-text text-transparent">
              before they do.
            </span>
          </motion.h1>

          <motion.ul variants={stagger} className="mt-10 space-y-4">
            {SELLING_POINTS.map((p) => (
              <motion.li key={p} variants={reveal} className="flex items-start gap-3 text-lg text-slate-300">
                <CheckCircleIcon className="mt-0.5 h-6 w-6 shrink-0 text-emerald-400" />
                <span>{p}</span>
              </motion.li>
            ))}
          </motion.ul>
        </motion.div>

        <p className="relative flex items-center gap-2 text-base text-slate-400">
          <LockIcon className="h-5 w-5" />
          Self-hosted. Your data never leaves your infrastructure.
        </p>
      </aside>

      {/* ---- Form panel ---- */}
      <main className="relative flex items-center justify-center bg-white px-6 py-12 dark:bg-slate-950">
        <div className="absolute right-6 top-6">
          <ThemeToggle />
        </div>

        <motion.div
          initial={{ opacity: 0, y: 16 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: DUR.base, ease: EASE_OUT }}
          className="w-full max-w-lg"
        >
          {/* Brand shows on mobile, where the aside is hidden. */}
          <Link href="/" className="mb-8 inline-flex items-center gap-2.5 lg:hidden">
            <BeaconMark className="h-8 w-8 text-blue-600 dark:text-blue-400" />
            <span className="text-xl font-semibold tracking-tight text-slate-900 dark:text-white">
              Beacon Pulse
            </span>
          </Link>

          <h2 className="text-4xl font-semibold tracking-tight text-slate-900 dark:text-white">
            {mode === "login" ? "Welcome back" : "Start monitoring free"}
          </h2>
          <p className="mt-2.5 text-lg text-slate-600 dark:text-slate-300">
            {mode === "login"
              ? "Sign in to your Beacon Pulse dashboard."
              : "No credit card. Add your first domain in under a minute."}
          </p>

          {/* Mode switch. aria-pressed carries the state — styling alone would not. */}
          <div
            role="group"
            aria-label="Authentication mode"
            id={groupId}
            className="mt-8 flex rounded-xl bg-slate-100 p-1.5 dark:bg-slate-900"
          >
            {(["login", "register"] as const).map((m) => (
              <button
                key={m}
                type="button"
                onClick={() => setMode(m)}
                aria-pressed={mode === m}
                className={`relative flex-1 rounded-lg px-4 py-2.5 text-base font-medium transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 motion-reduce:transition-none ${
                  mode === m
                    ? "text-slate-900 dark:text-white"
                    : "text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white"
                }`}
              >
                {/* The pill slides between tabs via a shared layoutId — the motion
                    itself explains that these two are one control, not two. */}
                {mode === m && (
                  <motion.span
                    layoutId="auth-tab"
                    transition={{ type: "spring", stiffness: 320, damping: 30 }}
                    className="absolute inset-0 rounded-lg bg-white shadow-sm dark:bg-slate-800"
                  />
                )}
                <span className="relative">{m === "login" ? "Sign in" : "Create account"}</span>
              </button>
            ))}
          </div>

          <div className="mt-8">
            {/* Crossfade between forms: same container, replaced content. */}
            <AnimatePresence mode="wait">
              <motion.div
                key={mode}
                initial={{ opacity: 0, y: 8 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0, y: -8 }}
                transition={{ duration: DUR.base, ease: EASE_OUT }}
              >
                {mode === "login" ? <LoginForm /> : <RegisterForm />}
              </motion.div>
            </AnimatePresence>
          </div>

          <p className="mt-8 text-center text-base text-slate-600 dark:text-slate-400">
            {mode === "login" ? "New to Beacon Pulse?" : "Already have an account?"}{" "}
            <button
              type="button"
              onClick={() => setMode(mode === "login" ? "register" : "login")}
              className="group inline-flex items-center gap-1 rounded font-medium text-blue-700 underline-offset-4 hover:underline focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 dark:text-blue-400"
            >
              {mode === "login" ? "Create an account" : "Sign in"}
              <ArrowRightIcon className="h-4 w-4 transition-transform group-hover:translate-x-0.5 motion-reduce:transition-none" />
            </button>
          </p>
        </motion.div>
      </main>
    </div>
  );
}
