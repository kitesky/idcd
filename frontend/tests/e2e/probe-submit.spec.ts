import { test, expect, type Page } from '@playwright/test'

/**
 * Probe tools that follow the ProbeForm pattern: target input + "开始拨测".
 * For each tool, we fill a known target, submit, and assert the API responded.
 */

const PROBE_TOOLS: Array<{ slug: string; target: string }> = [
  { slug: 'http', target: 'https://example.com' },
  { slug: 'dns', target: 'example.com' },
  { slug: 'tcp', target: 'example.com:443' },
  { slug: 'traceroute', target: '1.1.1.1' },
  { slug: 'mtr', target: '1.1.1.1' },
]

async function fillProbeTarget(page: Page, target: string) {
  // ProbeForm renders a single Input for the target. The Input has no explicit
  // type — Radix Input defaults to type="text". Use placeholder-based selection
  // anchored on common substrings to disambiguate.
  const input = page.getByPlaceholder(/example\.com|1\.1\.1\.1|输入目标|https?:/).first()
  await input.pressSequentially(target)
}

for (const { slug, target } of PROBE_TOOLS) {
  test(`probe /tools/${slug} submits and gets an API response`, async ({ page }) => {
    const errors: string[] = []
    page.on('pageerror', e => errors.push(e.message))

    await page.goto(`/tools/${slug}`)
    await fillProbeTarget(page, target)

    const submitBtn = page.locator('button[type="submit"]', { hasText: /开始拨测|start/i })
    await expect(submitBtn).toBeEnabled({ timeout: 5_000 })

    const submitPromise = page.waitForResponse(
      r => r.url().includes('/v1/probe/') && r.request().method() === 'POST',
      { timeout: 15_000 },
    )
    await submitBtn.click()
    const resp = await submitPromise
    expect(
      resp.status(),
      `probe submit for ${slug} returned ${resp.status()}`,
    ).toBeLessThan(500)

    expect(errors, `pageerror on ${slug}: ${errors.join(' | ')}`).toHaveLength(0)
  })
}
