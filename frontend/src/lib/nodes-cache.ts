import { getNodes, type Node } from "@/lib/api"

// Module-level cache of GET /v1/nodes shared across every consumer in the
// browser tab. Three earlier copies of this (usePollingProbeResult,
// SnapshotResultPanel, TracerouteResultPanel) silently had *independent*
// caches — the comment "mirrors X" was wrong, each module saw its own
// nodesPromise reset on error. Centralizing here means one in-flight request,
// one shared list, and consistent node names across panels.
//
// Node names are effectively static — stale data after a rename is acceptable
// (the panel falls back to raw id when a node isn't in the map).
let nodesCache: Node[] | null = null
let nodesPromise: Promise<Node[]> | null = null

export function fetchNodesOnce(): Promise<Node[]> {
  if (nodesCache) return Promise.resolve(nodesCache)
  if (!nodesPromise) {
    nodesPromise = getNodes()
      .then((n) => { nodesCache = n; return n })
      .catch((err) => { nodesPromise = null; throw err })
  }
  return nodesPromise
}
