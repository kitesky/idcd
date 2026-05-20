// Dev-only heap dump + soft-limit watchdog for next-server.
//
// Activated by setting IDCD_DEV_HEAP_DUMP=1 in the `dev:trace-oom` script.
// Lives in a separate file so the bundler can tree-shake it out of edge /
// production builds — instrumentation.ts dynamic-imports this only when the
// gate matches.

import { writeHeapSnapshot } from 'node:v8'
import { mkdirSync, readdirSync, statSync, unlinkSync } from 'node:fs'
import { join } from 'node:path'

// Resolve absolute path to frontend/.heap-profile/ — next dev's cwd is the
// directory containing next.config.ts, which is `frontend/` after the refactor.
const PROFILE_DIR = resolveProfileDir()

// Soft heap ceiling: when RSS crosses this, write a final dump and exit.
// V8 heap limit (--max-old-space-size) is filtered out by Next.js when it
// forks next-server, so we cannot trust the V8 ceiling — enforce in JS.
//
// 3500 MB chosen because RSS = V8 heap + native + arrayBuffers. With a 4 GB
// heap fully utilized, RSS often runs 5–6 GB. The earlier leaking sessions
// crossed 8 GB / 14 GB before the user could intervene, which is exactly
// what kills the remote SSH session. Bail early.
const SOFT_RSS_LIMIT_MB = 3500

export function startDevHeapDump(): void {
  let started: boolean
  try {
    mkdirSync(PROFILE_DIR, { recursive: true })
    started = true
  } catch (err) {
    log(`init failed: ${(err as Error).message}`)
    return
  }
  if (!started) return

  const startedAt = Date.now()
  const pid = process.pid
  log(`enabled pid=${pid} runtime=${process.env.NEXT_RUNTIME ?? '?'} dir=${PROFILE_DIR}`)

  // Take an early baseline once the server has settled.
  setTimeout(() => takeSnapshot('baseline'), 8_000).unref()

  // Progress log every 30s.
  setInterval(() => {
    const mem = process.memoryUsage()
    const elapsed = Math.round((Date.now() - startedAt) / 1000)
    log(
      `+${elapsed}s rss=${mb(mem.rss)} heapUsed=${mb(mem.heapUsed)} ` +
        `heapTotal=${mb(mem.heapTotal)} ext=${mb(mem.external)} ` +
        `arr=${mb(mem.arrayBuffers)}`,
    )
  }, 30_000).unref()

  // Periodic full snapshot every 2 min.
  setInterval(() => takeSnapshot('tick'), 120_000).unref()

  // Milestone snapshots: heap crossings at 1G / 2G / 3G (one-shot each).
  const milestonesMB = [1024, 2048, 3072]
  const milestoneTimer = setInterval(() => {
    const heapMB = process.memoryUsage().heapUsed / 1024 / 1024
    while (milestonesMB.length && heapMB >= milestonesMB[0]!) {
      const crossed = milestonesMB.shift()!
      takeSnapshot(`milestone-${crossed}M`)
    }
    if (!milestonesMB.length) clearInterval(milestoneTimer)
  }, 5_000).unref()

  // Soft RSS guard — fire-once kill switch.
  let bailing = false
  setInterval(() => {
    if (bailing) return
    const rssMB = process.memoryUsage().rss / 1024 / 1024
    if (rssMB < SOFT_RSS_LIMIT_MB) return
    bailing = true
    log(`!! RSS ${Math.round(rssMB)}M > ${SOFT_RSS_LIMIT_MB}M, dumping + exiting`)
    takeSnapshot('softlimit-bail')
    log('!! exit(137)')
    // SIGKILL-equivalent code; the parent next dev will detect and report.
    setTimeout(() => process.exit(137), 200).unref()
  }, 2_000).unref()
}

function takeSnapshot(label: string): void {
  try {
    const stamp = new Date()
      .toISOString()
      .replace(/[:.]/g, '-')
      .replace('T', '_')
      .slice(0, 19)
    const rssMB = Math.round(process.memoryUsage().rss / 1024 / 1024)
    const file = join(
      PROFILE_DIR,
      `heap_${stamp}_next-server_${label}_rss${rssMB}M_pid${process.pid}.heapsnapshot`,
    )
    log(`writing ${file} ...`)
    const written = writeHeapSnapshot(file)
    log(`wrote ${written}`)
    pruneOldSnapshots()
  } catch (err) {
    log(`snapshot failed: ${(err as Error).message}`)
  }
}

function pruneOldSnapshots(): void {
  const KEEP = 8
  try {
    const entries = readdirSync(PROFILE_DIR)
      .filter((f) => f.endsWith('.heapsnapshot'))
      .map((f) => {
        const full = join(PROFILE_DIR, f)
        return { full, mtime: statSync(full).mtimeMs }
      })
      .sort((a, b) => b.mtime - a.mtime)
    for (const entry of entries.slice(KEEP)) {
      try {
        unlinkSync(entry.full)
      } catch {
        // ignore — old snapshot already gone
      }
    }
  } catch {
    // ignore — pruning is best-effort
  }
}

function resolveProfileDir(): string {
  // next dev sets cwd to the directory containing next.config.ts, i.e. the
  // frontend root, so a direct child .heap-profile/ is correct. .gitignore
  // covers both 'frontend/.heap-profile/' and the generic '.heap-profile/'.
  return join(process.cwd(), '.heap-profile')
}

function mb(bytes: number): string {
  return `${(bytes / 1024 / 1024).toFixed(0)}M`
}

function log(msg: string): void {
  process.stderr.write(`[heap-dump] ${msg}\n`)
}
