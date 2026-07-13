"use client";

// Shared motion vocabulary for the marketing surfaces.
//
// One rhythm for the whole site: every reveal, stagger and press uses the tokens
// below, so nothing feels like it came from a different product. Two rules are
// baked in rather than left to each component:
//
//   1. Only `transform` and `opacity` are animated. Animating width/height/top
//      triggers layout on every frame; transform/opacity stay on the compositor.
//   2. Exits are faster than entrances (~65%), which is what makes a UI feel
//      responsive rather than sluggish.
//
// Reduced motion is handled at the source: `useRevealVariants()` collapses every
// reveal to a plain fade when the user asks for less motion, so callers cannot
// forget. Never import the raw variants directly for a reveal — call the hook.

import { useReducedMotion, type Transition, type Variants } from "framer-motion";

/** Durations, in seconds. Micro-interactions stay in the 150-300ms band. */
export const DUR = {
  micro: 0.18,
  base: 0.32,
  slow: 0.6,
} as const;

/** Spring used for anything the pointer drives — physics reads as "alive". */
export const SPRING: Transition = {
  type: "spring",
  stiffness: 260,
  damping: 30,
  mass: 0.6,
};

/** A softer spring for large surfaces (cards, panels) so they settle, not snap. */
export const SPRING_SOFT: Transition = {
  type: "spring",
  stiffness: 120,
  damping: 20,
  mass: 0.8,
};

/** ease-out for entrances: fast start, gentle landing. */
export const EASE_OUT = [0.16, 1, 0.3, 1] as const;

/**
 * Reveal-on-scroll variants. Content rises 16px and fades in.
 *
 * Returns a plain fade (no travel) when the user prefers reduced motion — the
 * content still animates enough to signal "this is new", without vestibular
 * motion. Returning variants (rather than disabling animation entirely) keeps
 * the DOM identical in both modes, so there is no layout difference to test.
 */
export function useRevealVariants(): Variants {
  const reduce = useReducedMotion();
  return {
    hidden: { opacity: 0, y: reduce ? 0 : 16 },
    show: {
      opacity: 1,
      y: 0,
      transition: { duration: reduce ? DUR.micro : DUR.base, ease: EASE_OUT },
    },
  };
}

/**
 * Parent for a staggered group. Children reveal 45ms apart — enough to read as a
 * sequence, not so slow that the last item feels late.
 */
export function useStaggerVariants(stagger = 0.045): Variants {
  const reduce = useReducedMotion();
  return {
    hidden: {},
    show: {
      transition: {
        staggerChildren: reduce ? 0 : stagger,
        delayChildren: reduce ? 0 : 0.04,
      },
    },
  };
}

/** Standard viewport config: fire once, slightly before the element is centred. */
export const IN_VIEW = { once: true, margin: "-80px" } as const;
