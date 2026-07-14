// Terminal-styled 404 for an unknown or unpublished status slug. Kept intentionally
// vague — an unpublished org and a non-existent one look identical, so this page is
// never an oracle for which orgs exist.

const CRT: React.CSSProperties = {
  backgroundImage: [
    "repeating-linear-gradient(0deg, rgba(255,255,255,0.035) 0px, rgba(255,255,255,0.035) 1px, transparent 1px, transparent 3px)",
    "radial-gradient(120% 90% at 50% 0%, transparent 55%, rgba(0,0,0,0.55) 100%)",
  ].join(","),
};

export default function StatusNotFound() {
  return (
    <div
      className="relative flex min-h-dvh items-center justify-center overflow-hidden bg-[#080a0f] p-6 text-slate-300"
      style={{ fontFamily: "var(--font-departure), ui-monospace, SFMono-Regular, Menlo, monospace" }}
    >
      <div aria-hidden className="pointer-events-none fixed inset-0" style={CRT} />
      <div className="relative w-full max-w-lg border border-slate-700/70 bg-[#0b0d13] p-6 shadow-[0_0_40px_-12px_rgba(255,90,30,0.25)]">
        <p className="flex items-center gap-2 text-xs uppercase tracking-[0.25em] text-orange-400">
          <span aria-hidden className="inline-block h-3 w-3 rotate-45 border border-orange-400" />
          BEACON // STATUS
        </p>
        <p className="mt-5 text-lg text-red-400">
          <span className="text-slate-600">&gt;</span> ERROR 404 — NO STATUS PAGE AT THIS ADDRESS
        </p>
        <p className="mt-2 text-sm text-slate-500">
          The page may not exist, or its owner has not published one. Check the address and try again.
        </p>
        <a
          href="/"
          className="mt-6 inline-block text-xs uppercase tracking-[0.25em] text-slate-500 underline-offset-4 hover:text-orange-400 hover:underline focus:outline-none focus-visible:ring-1 focus-visible:ring-orange-400"
        >
          ▮ RETURN TO BEACON
        </a>
      </div>
    </div>
  );
}
