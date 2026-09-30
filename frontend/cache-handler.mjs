/**
 * Redis-backed ISR / data cache handler for the standalone server (same approach as
 * workstation-website).
 *
 * Why: Next's default cache is per-pod on local disk. With more than one frontend
 * replica (prod runs 2), `revalidateTag("cms-posts")` from the OpsAPI webhook would only
 * bust the pod that received it and replicas would serve different versions of the
 * blog. A shared Redis cache makes tag revalidation global.
 *
 * Safety: an in-process LRU tier sits behind Redis, so a Redis outage degrades to
 * per-pod caching rather than a hard failure, and an unset BEACON_ISR_REDIS_URL falls
 * back to local-only (correct for a single replica, local prod builds and `next build`).
 */
import { CacheHandler } from "@fortedigital/nextjs-cache-handler";
import createLruHandler from "@fortedigital/nextjs-cache-handler/local-lru";
import createRedisHandler from "@fortedigital/nextjs-cache-handler/redis-strings";
import { createClient } from "@redis/client";

CacheHandler.onCreation(async () => {
  const localHandler = createLruHandler();

  const url = process.env.BEACON_ISR_REDIS_URL;
  if (!url) {
    return { handlers: [localHandler] };
  }

  let client;
  try {
    // connectTimeout bounds boot: a black-hole URL must not stall the first render
    // for the OS default (~minutes) — fall back to LRU quickly.
    client = createClient({ url, socket: { connectTimeout: 1000 } });
    // Never let a Redis error crash the server; the LRU tier keeps serving.
    client.on("error", (e) => console.warn("cache-handler: redis error:", e?.message || e));
    await client.connect();

    const redisHandler = createRedisHandler({
      client,
      // The Redis instance is shared with the API, so namespace every key.
      keyPrefix: "beacon:isr:",
      timeoutMs: 1000,
    });

    // Redis first (shared across replicas), LRU as an in-process fallback.
    return { handlers: [redisHandler, localHandler] };
  } catch (e) {
    console.warn("cache-handler: Redis unavailable, using local LRU only:", e?.message || e);
    // Don't leave a client auto-reconnecting in the background.
    try {
      await client?.quit();
    } catch {
      /* already down */
    }
    return { handlers: [localHandler] };
  }
});

export default CacheHandler;
