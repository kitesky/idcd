import { test, expect } from '@playwright/test'
import { TOOL_SLUGS } from './_constants'

test.describe('Tool pages smoke', () => {
  test('tool index lists tools', async ({ page }) => {
    const resp = await page.goto('/tools')
    expect(resp!.status()).toBeLessThan(400)
    await expect(page.locator('body')).toContainText(/[a-z]/i)
  })

  for (const slug of TOOL_SLUGS) {
    test(`tool /tools/${slug} renders without JS errors`, async ({ page }) => {
      const errors: string[] = []
      const consoleErrors: string[] = []
      page.on('pageerror', e => errors.push(e.message))
      page.on('console', msg => {
        if (msg.type() === 'error') consoleErrors.push(msg.text())
      })

      const resp = await page.goto(`/tools/${slug}`)
      expect(resp, `null response for /tools/${slug}`).not.toBeNull()
      expect([200, 304], `bad status for /tools/${slug}: ${resp!.status()}`).toContain(resp!.status())

      // Wait for the page to settle (no longer than navigationTimeout in config).
      await page.waitForLoadState('domcontentloaded')

      // The page must render a main heading or at least non-empty body.
      const bodyText = await page.locator('body').innerText()
      expect(bodyText.trim().length, `empty body for ${slug}`).toBeGreaterThan(20)

      expect(errors, `runtime error on /tools/${slug}: ${errors.join(' | ')}`).toHaveLength(0)
      // Filter out known-benign console warnings (favicon 404 etc.).
      const realErrors = consoleErrors.filter(
        e => !/(favicon|manifest|404|Failed to load resource)/i.test(e),
      )
      expect(realErrors, `console error on /tools/${slug}: ${realErrors.join(' | ')}`).toHaveLength(0)
    })
  }
})
