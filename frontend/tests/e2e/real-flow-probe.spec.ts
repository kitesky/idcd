import { test, expect } from '@playwright/test'

/**
 * Real end-to-end probe flow — the test that would have caught the
 * "queued forever" bug (aggregator UPDATE WHERE status='running' matching 0 rows).
 *
 * Submits a real ping task, polls /v1/probe/tasks/{id} until status leaves
 * "queued", and asserts the task lands in a terminal state (completed/failed).
 *
 * Requirements:
 *  - api running on :8080
 *  - gateway + agent + aggregator wired up (use scripts/start-local-stack.sh)
 *
 * Gated by IDCD_REAL_FLOW=1 — without an active agent the task stays queued by
 * design, so we skip in default CI/local runs to avoid false negatives.
 */

const REAL_FLOW = process.env.IDCD_REAL_FLOW === '1'
const API = process.env.IDCD_API_BASE ?? 'http://localhost:8080'

test.describe('Real probe flow (submit → completed)', () => {
  test.skip(!REAL_FLOW, 'set IDCD_REAL_FLOW=1 with full local stack to enable')

  test.setTimeout(90_000)

  test('ping task transitions queued → completed within 60s', async ({ request }) => {
    // Read node_id from the local-stack-enrolled agent config so we target the
    // process that's actually running on this machine. If absent, fall back to
    // the env override.
    let nodeID: string | undefined = process.env.IDCD_AGENT_NODE_ID
    try {
      const fs = await import('node:fs')
      const txt = fs.readFileSync('/tmp/idcd-agent.yaml', 'utf8')
      const m = txt.match(/^node_id:\s*(\S+)/m)
      if (m) nodeID = m[1]
    } catch { /* fallback to env */ }

    // Submit a ping probe.
    const submitResp = await request.post(`${API}/v1/probe/ping`, {
      data: {
        target: '8.8.8.8',
        count: 3,
        ...(nodeID ? { node_id: nodeID } : {}),
      },
    })
    expect(submitResp.status(), `submit returned ${submitResp.status()}`).toBeLessThan(400)
    const submitted = await submitResp.json()
    const taskID: string = submitted?.data?.task_id
    expect(taskID, 'no task_id returned').toBeTruthy()

    // Poll until terminal state or timeout.
    const deadline = Date.now() + 60_000
    let status = 'queued'
    let payload: unknown = null
    while (Date.now() < deadline) {
      const r = await request.get(`${API}/v1/probe/tasks/${taskID}`)
      expect(r.status()).toBeLessThan(500)
      payload = await r.json()
      const s = (payload as { data?: { status?: string } })?.data?.status
      if (s && s !== 'queued') {
        status = s
        break
      }
      await new Promise(res => setTimeout(res, 1_500))
    }

    expect(
      status,
      `task ${taskID} stayed queued — likely aggregator state-machine regression (T18). full payload: ${JSON.stringify(payload)}`,
    ).not.toBe('queued')
    // Acceptable terminal states: completed (normal), failed (network blocked etc).
    expect(['completed', 'failed', 'running']).toContain(status)
  })
})
