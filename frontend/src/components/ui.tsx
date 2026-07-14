// Dependency-free UI primitives styled with Tailwind. Every page composes these,
// so a fix here lands everywhere.
//
// Contrast note: slate-400 is 2.56:1 on white — it FAILS WCAG AA for text. Anywhere
// muted text is needed, pair the steps: `text-slate-500 dark:text-slate-400`
// (4.76:1 on white, 6.96:1 on slate-900).

import {
  ButtonHTMLAttributes,
  InputHTMLAttributes,
  ReactElement,
  ReactNode,
  SelectHTMLAttributes,
  TextareaHTMLAttributes,
  cloneElement,
  forwardRef,
  isValidElement,
  useId,
} from "react";

/* ---------------- button ---------------- */

type Variant = "primary" | "secondary" | "danger" | "ghost";
type Size = "sm" | "md" | "lg";

const variants: Record<Variant, string> = {
  primary: "bg-brand-600 text-white hover:bg-brand-700 focus-visible:ring-brand-500",
  secondary:
    "bg-white text-slate-800 border border-slate-300 hover:bg-slate-50 focus-visible:ring-brand-500 dark:bg-slate-800 dark:text-slate-100 dark:border-slate-700 dark:hover:bg-slate-700",
  danger: "bg-red-600 text-white hover:bg-red-700 focus-visible:ring-red-500",
  ghost:
    "text-slate-600 hover:bg-slate-100 hover:text-slate-900 focus-visible:ring-brand-500 dark:text-slate-300 dark:hover:bg-slate-800 dark:hover:text-slate-100",
};

const sizes: Record<Size, string> = {
  sm: "h-8 px-3 text-xs",
  md: "h-10 px-4 text-sm",
  // lg is the CTA size used on the auth screens: 48px clears the 44px touch
  // minimum comfortably, and 16px text avoids iOS's auto-zoom-on-focus.
  lg: "h-12 px-6 text-base",
};

export function Button({
  variant = "primary",
  size = "md",
  className = "",
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: Variant; size?: Size }) {
  return (
    <button
      className={`inline-flex items-center justify-center gap-2 rounded-lg font-medium transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50 motion-reduce:transition-none dark:focus-visible:ring-offset-slate-950 ${variants[variant]} ${sizes[size]} ${className}`}
      {...props}
    />
  );
}

/* ---------------- form controls ---------------- */

// These forward their ref so React Hook Form's register() can wire the DOM node.
// Without forwardRef the ref is dropped and RHF never reads the value — every
// field would validate as "Required" on submit.

const controlBase =
  "w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-base text-slate-900 transition-colors placeholder:text-slate-500 focus:border-brand-500 focus:outline-none focus:ring-2 focus:ring-brand-500/30 disabled:cursor-not-allowed disabled:opacity-60 motion-reduce:transition-none aria-[invalid=true]:border-red-500 aria-[invalid=true]:focus:ring-red-500/30 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100 dark:placeholder:text-slate-400";

export const Input = forwardRef<HTMLInputElement, InputHTMLAttributes<HTMLInputElement>>(
  function Input({ className = "", ...props }, ref) {
    return <input ref={ref} className={`${controlBase} ${className}`} {...props} />;
  },
);

export const Select = forwardRef<HTMLSelectElement, SelectHTMLAttributes<HTMLSelectElement>>(
  function Select({ className = "", children, ...props }, ref) {
    return (
      <select ref={ref} className={`${controlBase} ${className}`} {...props}>
        {children}
      </select>
    );
  },
);

export const Textarea = forwardRef<HTMLTextAreaElement, TextareaHTMLAttributes<HTMLTextAreaElement>>(
  function Textarea({ className = "", ...props }, ref) {
    return <textarea ref={ref} className={`${controlBase} ${className}`} {...props} />;
  },
);

export function Label({ children, htmlFor }: { children: ReactNode; htmlFor?: string }) {
  return (
    <label htmlFor={htmlFor} className="mb-1 block text-sm font-medium text-slate-700 dark:text-slate-300">
      {children}
    </label>
  );
}

/**
 * Field owns the accessibility wiring its children can't do for themselves: it
 * mints an id, points <label for> at the control, marks it aria-invalid when the
 * field errored, and links the error text via aria-describedby. The error is a
 * live region so a screen reader announces it as it appears.
 */
export function Field({
  label,
  error,
  hint,
  children,
}: {
  label: string;
  error?: string;
  hint?: string;
  children: ReactNode;
}) {
  const id = useId();
  const errorId = `${id}-error`;
  const hintId = `${id}-hint`;
  const describedBy = [error ? errorId : null, hint ? hintId : null].filter(Boolean).join(" ") || undefined;

  const control = isValidElement(children)
    ? cloneElement(children as ReactElement<Record<string, unknown>>, {
        id,
        "aria-invalid": error ? true : undefined,
        "aria-describedby": describedBy,
      })
    : children;

  return (
    <div>
      <Label htmlFor={id}>{label}</Label>
      {control}
      {hint && !error ? (
        <p id={hintId} className="mt-1 text-xs text-slate-500 dark:text-slate-400">
          {hint}
        </p>
      ) : null}
      {error ? (
        <p id={errorId} role="alert" className="mt-1 text-xs font-medium text-red-700 dark:text-red-400">
          {error}
        </p>
      ) : null}
    </div>
  );
}

/* ---------------- surfaces ---------------- */

export function Card({ children, className = "" }: { children: ReactNode; className?: string }) {
  return (
    <div
      className={`rounded-xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900 ${className}`}
    >
      {children}
    </div>
  );
}

/** One page-title pattern for every route, so headings never drift. */
export function PageHeader({
  title,
  subtitle,
  actions,
}: {
  title: string;
  subtitle?: string;
  actions?: ReactNode;
}) {
  return (
    <div className="flex flex-wrap items-end justify-between gap-3">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">{title}</h1>
        {subtitle ? <p className="mt-0.5 text-sm text-slate-500 dark:text-slate-400">{subtitle}</p> : null}
      </div>
      {actions ? <div className="flex items-center gap-2">{actions}</div> : null}
    </div>
  );
}

/** Empty states get an icon, a sentence and (ideally) the next action. */
export function EmptyState({
  icon,
  title,
  children,
  action,
}: {
  icon?: ReactNode;
  title: string;
  children?: ReactNode;
  action?: ReactNode;
}) {
  return (
    <div className="rounded-xl border border-dashed border-slate-300 bg-white/50 px-6 py-12 text-center dark:border-slate-700 dark:bg-slate-900/50">
      {icon ? (
        <div className="mx-auto mb-3 grid h-10 w-10 place-items-center rounded-full bg-slate-100 text-slate-500 dark:bg-slate-800 dark:text-slate-400">
          {icon}
        </div>
      ) : null}
      <p className="text-sm font-semibold text-slate-900 dark:text-slate-100">{title}</p>
      {children ? <p className="mx-auto mt-1 max-w-md text-sm text-slate-500 dark:text-slate-400">{children}</p> : null}
      {action ? <div className="mt-4 flex justify-center">{action}</div> : null}
    </div>
  );
}

/** Reserve the space content will occupy, so nothing jumps when it lands (CLS). */
export function Skeleton({ className = "" }: { className?: string }) {
  return <div className={`rounded-md bg-slate-100 motion-safe:animate-pulse dark:bg-slate-800 ${className}`} />;
}

/* ---------------- badges ---------------- */

// Status tints. Text steps are chosen for >=4.5:1 against their own tint.
const statusStyles: Record<string, string> = {
  up: "bg-emerald-100 text-emerald-800 dark:bg-emerald-900/40 dark:text-emerald-200",
  down: "bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-200",
  degraded: "bg-amber-100 text-amber-900 dark:bg-amber-900/40 dark:text-amber-200",
  paused: "bg-slate-200 text-slate-700 dark:bg-slate-800 dark:text-slate-300",
  unknown: "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300",
};

export function StatusBadge({ status }: { status: string }) {
  const style = statusStyles[status] ?? statusStyles.unknown;
  return (
    <span className={`inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium ${style}`}>
      {/* The dot is a mark; the word beside it carries the meaning, so status is
          never conveyed by colour alone. */}
      <span className="h-1.5 w-1.5 rounded-full bg-current" aria-hidden />
      {status}
    </span>
  );
}

export function Badge({ children, className = "" }: { children: ReactNode; className?: string }) {
  return (
    <span
      className={`inline-flex items-center rounded px-1.5 py-0.5 text-xs font-medium uppercase tracking-wide text-slate-600 dark:text-slate-300 ${className}`}
    >
      {children}
    </span>
  );
}
