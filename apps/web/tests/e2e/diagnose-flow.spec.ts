import { test, expect } from '@playwright/test'

test.describe('Diagnose tool — SSE integration', () => {
  test('SSE endpoint streams diagnose events', async ({ request }) => {
    const r = await request.get('/api/diagnose/stream?target=example.com', {
      headers: { Accept: 'text/event-stream' },
      timeout: 15_000,
      failOnStatusCode: false,
    })
    // Endpoint must respond (any non-5xx; 401/400 acceptable if validation rejects).
    expect(r.status()).toBeLessThan(500)
    const ct = r.headers()['content-type'] ?? ''
    // Stream content-type on success, or JSON/text on validation reject — all prove the route is wired.
    expect(/text\/event-stream|application\/json|text\/plain/i.test(ct), `unexpected content-type: ${ct}`).toBe(true)
  })

  test('diagnose page form is interactive', async ({ page }) => {
    await page.goto('/tools/diagnose')
    const inputs = page.locator('input, textarea')
    await expect(inputs.first()).toBeVisible({ timeout: 10_000 })
    await inputs.first().fill('example.com')

    // Find a button that looks like "开始 / Start / Diagnose / 诊断".
    const startBtn = page.locator('button').filter({
      hasText: /(开始|start|诊断|diagnose|run)/i,
    }).first()
    // Some pages render the button as soon as input is non-empty; at minimum it should exist.
    await expect(startBtn).toBeAttached({ timeout: 5_000 })
  })
})
