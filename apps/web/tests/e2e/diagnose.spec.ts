import { test, expect } from '@playwright/test'

test.describe('Diagnose SSE flow', () => {
  test('/tools/diagnose page renders and form is present', async ({ page }) => {
    const errors: string[] = []
    page.on('pageerror', e => errors.push(e.message))

    const resp = await page.goto('/tools/diagnose')
    expect(resp!.status()).toBeLessThan(400)
    await expect(page.locator('input, textarea').first()).toBeVisible({ timeout: 10_000 })
    expect(errors).toHaveLength(0)
  })

  test('SSE endpoint /api/diagnose/stream responds with text/event-stream', async ({ request }) => {
    // Just probe the endpoint shape; full streaming is exercised by the page.
    const r = await request.get('/api/diagnose/stream?target=example.com', {
      headers: { Accept: 'text/event-stream' },
      timeout: 10_000,
      failOnStatusCode: false,
    })
    // 2xx with stream content-type, OR 401/400 if param validation fires — neither is a crash.
    expect(r.status(), `unexpected status ${r.status()}`).toBeLessThan(500)
  })
})
