"use client";

// Pointer-driven effects: a cursor spotlight and a 3D tilt card.
//
// Both are deliberately built on motion values rather than React state. A
// pointermove handler that calls setState re-renders the subtree on every mouse
// event — at 120Hz that is a guaranteed frame drop. Motion values write straight
// to the element's transform off the React render path, so the cursor stays
// smooth no matter how heavy the page is.
//
// Everything here is decoration, so it is the FIRST thing to switch off under
// prefers-reduced-motion: the spotlight renders nothing and the tilt stays flat.
// A pointer effect is also meaningless on touch, so both are gated behind a
// fine-pointer check rather than shipping dead listeners to phones.

import {
  motion,
  useMotionTemplate,
  useMotionValue,
  useReducedMotion,
  useSpring,
} from "framer-motion";
import { useEffect, useRef, useState, type ReactNode } from "react";

import { SPRING } from "@/lib/motion";

/** True only for devices with a precise pointer (mouse/trackpad). */
function useFinePointer(): boolean {
  const [fine, setFine] = useState(false);
  useEffect(() => {
    const mq = window.matchMedia("(pointer: fine)");
    const sync = () => setFine(mq.matches);
    sync();
    mq.addEventListener("change", sync);
    return () => mq.removeEventListener("change", sync);
  }, []);
  return fine;
}

/**
 * A soft light that follows the cursor across the section it wraps.
 *
 * Renders nothing at all for reduced-motion or touch users, so they never pay
 * for the listener or the extra paint layer.
 */
export function Spotlight({ className = "" }: { className?: string }) {
  const reduce = useReducedMotion();
  const fine = useFinePointer();

  // Start off-screen so the light does not flash at 0,0 before the first move.
  const x = useMotionValue(-9999);
  const y = useMotionValue(-9999);
  // Spring the position so the light trails the cursor slightly — an exactly
  // pinned light reads as a cheap CSS trick; a trailing one reads as a material.
  const sx = useSpring(x, SPRING);
  const sy = useSpring(y, SPRING);

  const enabled = fine && !reduce;

  useEffect(() => {
    if (!enabled) return;
    const onMove = (e: PointerEvent) => {
      x.set(e.clientX);
      y.set(e.clientY);
    };
    window.addEventListener("pointermove", onMove, { passive: true });
    return () => window.removeEventListener("pointermove", onMove);
  }, [enabled, x, y]);

  const background = useMotionTemplate`radial-gradient(420px circle at ${sx}px ${sy}px, rgba(42,120,214,0.16), transparent 78%)`;

  if (!enabled) return null;

  return (
    <motion.div
      aria-hidden
      className={`pointer-events-none fixed inset-0 z-0 ${className}`}
      style={{ background }}
    />
  );
}

/**
 * Tilts its children toward the cursor. The rotation is small on purpose —
 * beyond ~8deg the text on the card starts to distort and it stops looking like
 * a solid object.
 */
export function TiltCard({
  children,
  className = "",
  max = 7,
}: {
  children: ReactNode;
  className?: string;
  max?: number;
}) {
  const reduce = useReducedMotion();
  const fine = useFinePointer();
  const ref = useRef<HTMLDivElement>(null);

  const rx = useMotionValue(0);
  const ry = useMotionValue(0);
  const srx = useSpring(rx, SPRING);
  const sry = useSpring(ry, SPRING);

  const enabled = fine && !reduce;

  function onPointerMove(e: React.PointerEvent<HTMLDivElement>) {
    if (!enabled || !ref.current) return;
    const r = ref.current.getBoundingClientRect();
    // Normalise the pointer to -0.5..0.5 within the card, then map to degrees.
    const px = (e.clientX - r.left) / r.width - 0.5;
    const py = (e.clientY - r.top) / r.height - 0.5;
    rx.set(-py * max * 2); // pointer below centre => tilt away from viewer
    ry.set(px * max * 2);
  }

  function reset() {
    rx.set(0);
    ry.set(0);
  }

  return (
    <motion.div
      ref={ref}
      onPointerMove={onPointerMove}
      onPointerLeave={reset}
      style={
        enabled
          ? { rotateX: srx, rotateY: sry, transformPerspective: 1000 }
          : undefined
      }
      className={className}
    >
      {children}
    </motion.div>
  );
}

/**
 * A card that lights up under the cursor. Unlike Spotlight this is local to the
 * element, which is what makes a grid of cards feel individually "alive".
 */
export function GlowCard({
  children,
  className = "",
}: {
  children: ReactNode;
  className?: string;
}) {
  const reduce = useReducedMotion();
  const fine = useFinePointer();
  const ref = useRef<HTMLDivElement>(null);
  const x = useMotionValue(-9999);
  const y = useMotionValue(-9999);

  const enabled = fine && !reduce;

  function onPointerMove(e: React.PointerEvent<HTMLDivElement>) {
    if (!enabled || !ref.current) return;
    const r = ref.current.getBoundingClientRect();
    x.set(e.clientX - r.left);
    y.set(e.clientY - r.top);
  }

  const background = useMotionTemplate`radial-gradient(220px circle at ${x}px ${y}px, rgba(42,120,214,0.10), transparent 80%)`;

  return (
    <div
      ref={ref}
      onPointerMove={onPointerMove}
      onPointerLeave={() => {
        x.set(-9999);
        y.set(-9999);
      }}
      className={`group relative overflow-hidden ${className}`}
    >
      {enabled && (
        <motion.div
          aria-hidden
          className="pointer-events-none absolute inset-0 opacity-0 transition-opacity duration-300 group-hover:opacity-100 motion-reduce:transition-none"
          style={{ background }}
        />
      )}
      {children}
    </div>
  );
}
