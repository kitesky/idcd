import { test, expect } from '@playwright/test'
import { PUBLIC_PAGES } from './_constants'

test.describe('Public pages smoke', () => {
  for (const path of PUBLIC_PAGES) {
    test(`renders ${path}`, async ({ page }) => {
      const errors: string[] = []
      page.on('pageerror', e => errors.push(e.message))

      const resp = await page.goto(path)
      expect(resp, `no response for ${path}`).not.toBeNull()
      // Accept 200 (page) or 404 (acceptable for not-yet-implemented routes; we just assert no JS crash).
      const status = resp!.status()
      expect([200, 304, 404]).toContain(status)

      // The app should render some body text regardless of status.
      await expect(page.locator('body')).toBeVisible()
      expect(errors, `pageerror on ${path}: ${errors.join(' | ')}`).toHaveLength(0)
    })
  }
})
