"use client";

// Time-aware rendering, done as an external store rather than as clock reads during
// render.
//
// Reading Date.now() while rendering is impure in two ways that both bite here:
// the server and the browser never agree on it (a hydration mismatch), and React is
// free to reuse a render whose "now" has since moved on — so a maintenance window
// that has ended goes on claiming it is running until something unrelated triggers a
// re-render. Modelling the clock as a store React subscribes to fixes both: render
// stays pure, and anything derived from time updates on its own, with no refresh.

import { useSyncExternalStore } from "react";

interface Ticker {
  subscribe: (onChange: () => void) => () => void;
  getSnapshot: () => number;
}

// One ticker per cadence, shared by every component that asks for it, so a hundred
// "2M AGO" labels cost one timer rather than a hundred. The timer only exists while
// something is subscribed.
const tickers = new Map<number, Ticker>();

function tickerFor(intervalMs: number): Ticker {
  const existing = tickers.get(intervalMs);
  if (existing) return existing;

  const listeners = new Set<() => void>();
  let now = Date.now();
  let timer: ReturnType<typeof setInterval> | null = null;

  const ticker: Ticker = {
    subscribe(onChange) {
      listeners.add(onChange);
      if (timer === null) {
        // Resync on the way in: while nothing was subscribed the timer was off, so
        // `now` is as old as the gap and would otherwise be served for a full
        // interval before the first tick corrected it.
        now = Date.now();
        timer = setInterval(() => {
          now = Date.now();
          for (const listener of listeners) listener();
        }, intervalMs);
      }
      return () => {
        listeners.delete(onChange);
        if (listeners.size === 0 && timer !== null) {
          clearInterval(timer);
          timer = null;
        }
      };
    },
    // Cached between ticks on purpose: getSnapshot has to return a value that stays
    // identical until it genuinely changes. Returning a fresh Date.now() per call
    // would never compare equal and would re-render forever.
    getSnapshot: () => now,
  };

  tickers.set(intervalMs, ticker);
  return ticker;
}

// The server has no clock worth agreeing with the browser about, so it reports null
// and callers render the time-dependent part only once hydrated.
const getServerSnapshot = () => null;

/**
 * useNow returns the current time in epoch ms, re-rendering on every tick, and null
 * on the server and during hydration. Derive time-dependent values from it during
 * render instead of calling Date.now() there.
 */
export function useNow(intervalMs = 30_000): number | null {
  const ticker = tickerFor(intervalMs);
  return useSyncExternalStore(ticker.subscribe, ticker.getSnapshot, getServerSnapshot);
}

const subscribeToNothing = () => () => {};
const clientSnapshot = () => true;
const serverSnapshot = () => false;

/**
 * useHydrated is false on the server and for the first client render, true after.
 * Gate anything the server cannot render identically — locale/timezone formatting,
 * most obviously — so the markup matches and React never has to patch it up.
 */
export function useHydrated(): boolean {
  return useSyncExternalStore(subscribeToNothing, clientSnapshot, serverSnapshot);
}
