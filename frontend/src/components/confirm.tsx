"use client";

// A promise-based confirmation dialog to replace window.confirm(). Mount
// <ConfirmProvider> once (in the dashboard layout); call const confirm =
// useConfirm() and `if (await confirm({ ... }))` anywhere beneath it.
//
// Why a real modal: window.confirm blocks the main thread, can't be themed or
// made dark-mode/aria consistent, and looks like a browser popup rather than the
// product. This one is focus-managed, Escape/backdrop dismissable, and styled with
// the app's design system.

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";

import { Button } from "@/components/ui";

type ConfirmOptions = {
  title: string;
  body?: ReactNode;
  confirmLabel?: string;
  cancelLabel?: string;
  /** Style the confirm button as destructive and focus Cancel by default. */
  danger?: boolean;
};

const ConfirmContext = createContext<(o: ConfirmOptions) => Promise<boolean>>(
  async () => false,
);

/** Returns an async confirm(opts) that resolves true if the user confirms. */
export function useConfirm() {
  return useContext(ConfirmContext);
}

export function ConfirmProvider({ children }: { children: ReactNode }) {
  const [opts, setOpts] = useState<ConfirmOptions | null>(null);
  const resolver = useRef<((ok: boolean) => void) | null>(null);

  const confirm = useCallback((o: ConfirmOptions) => {
    setOpts(o);
    return new Promise<boolean>((resolve) => {
      resolver.current = resolve;
    });
  }, []);

  const resolve = useCallback((ok: boolean) => {
    resolver.current?.(ok);
    resolver.current = null;
    setOpts(null);
  }, []);

  return (
    <ConfirmContext.Provider value={confirm}>
      {children}
      {opts && <ConfirmDialog opts={opts} onResolve={resolve} />}
    </ConfirmContext.Provider>
  );
}

function ConfirmDialog({
  opts,
  onResolve,
}: {
  opts: ConfirmOptions;
  onResolve: (ok: boolean) => void;
}) {
  const confirmRef = useRef<HTMLButtonElement>(null);
  const cancelRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    // Focus the safe default: Cancel for destructive actions, otherwise Confirm.
    (opts.danger ? cancelRef : confirmRef).current?.focus();
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onResolve(false);
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [opts.danger, onResolve]);

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      <div
        className="absolute inset-0 bg-slate-900/50 backdrop-blur-sm"
        onClick={() => onResolve(false)}
        aria-hidden
      />
      <div
        role="alertdialog"
        aria-modal="true"
        aria-labelledby="confirm-title"
        aria-describedby={opts.body ? "confirm-body" : undefined}
        className="relative w-full max-w-md rounded-xl border border-slate-200 bg-white p-5 shadow-2xl dark:border-slate-800 dark:bg-slate-900"
      >
        <h2 id="confirm-title" className="text-lg font-semibold text-slate-900 dark:text-slate-100">
          {opts.title}
        </h2>
        {opts.body ? (
          <div id="confirm-body" className="mt-2 text-sm text-slate-600 dark:text-slate-300">
            {opts.body}
          </div>
        ) : null}
        <div className="mt-5 flex justify-end gap-2">
          <Button ref={cancelRef} variant="secondary" onClick={() => onResolve(false)}>
            {opts.cancelLabel ?? "Cancel"}
          </Button>
          <Button
            ref={confirmRef}
            variant={opts.danger ? "danger" : "primary"}
            onClick={() => onResolve(true)}
          >
            {opts.confirmLabel ?? "Confirm"}
          </Button>
        </div>
      </div>
    </div>
  );
}
