import { test, expect } from '@playwright/test'

/**
 * Probe tools that don't use the shared ProbeForm — smtp/ntp/speedtest each
 * have their own client. Migrations 00034 (smtp/ntp) and 00037 (speedtest)
 * must be applied for these to not 500 with a check-constraint violation.
 */

const EXTENDED_PROBES: Array<{ slug: string; inputId: string; target: string }> = [
  { slug: 'smtp', inputId: '#smtp-target', target: 'smtp.gmail.com' },
  { slug: 'ntp', inputId: '#ntp-target', target: 'pool.ntp.org' },
  { slug: 'speedtest', inputId: '#speedtest-target', target: 'https://speed.cloudflare.com/__down?bytes=1000000' },
]

for (const { slug, inputId, target } of EXTENDED_PROBES) {
  test(`probe /tools/${slug} submits via its custom client`, async ({ page }) => {
    const errors: string[] = []
    page.on('pageerror', e => errors.push(e.message))

    await page.goto(`/tools/${slug}`)
    await page.locator(inputId).fill(target)

    const submitBtn = page.locator('button[type="submit"]').filter({
      hasText: /开始(检测|测速|拨测)|start/i,
    })
    await expect(submitBtn).toBeEnabled({ timeout: 5_000 })

    const respPromise = page.waitForResponse(
      r => r.url().includes('/v1/probe/') && r.request().method() === 'POST',
      { timeout: 15_000 },
    )
    await submitBtn.click()
    const resp = await respPromise
    expect(
      resp.status(),
      `${slug} submit returned ${resp.status()} — check probe_task_type_check constraint`,
    ).toBeLessThan(500)

    expect(errors, `pageerror on ${slug}: ${errors.join(' | ')}`).toHaveLength(0)
  })
}
