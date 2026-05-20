// Next.js 16 instrumentation hook — runs once when next-server (or any Node
// runtime worker) boots, *before* it starts serving requests.
//
// We use this purely as a dev diagnostic for the OOM/leak in next-server:
//   - the `--import` path via NODE_OPTIONS does NOT work because Next.js
//     filters NODE_OPTIONS when forking next-server, so the v8 heap limit
//     and our heap-dump hook never reach the leaking process.
//   - this file is the only thing Next.js guarantees to load *inside*
//     next-server itself, so the hook lives here.
//
// Production: this function still gets called once at server boot, but
// `startDevHeapDump` is a no-op outside development.

export async function register(): Promise<void> {
  if (process.env.NODE_ENV === 'production') return
  if (process.env.NEXT_RUNTIME !== 'nodejs') return // skip edge runtime
  if (process.env.IDCD_DEV_HEAP_DUMP !== '1') return // opt-in only

  // Dynamic import so the bundler doesn't try to resolve node:v8 / node:fs
  // for the edge runtime build. Also keeps the dev-only payload out of the
  // production server.
  const { startDevHeapDump } = await import('./instrumentation-dev-heap')
  startDevHeapDump()
}
