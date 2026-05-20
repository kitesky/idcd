import { test, expect } from '@playwright/test'
import { APP_PAGES } from './_constants'

test.describe('User center (/app/*) — authenticated via storageState', () => {
  for (const path of APP_PAGES) {
    test(`renders ${path}`, async ({ page }) => {
      const errors: string[] = []
      page.on('pageerror', e => errors.push(e.message))

      const resp = await page.goto(path)
      expect(resp, `null response for ${path}`).not.toBeNull()
      expect(resp!.status()).toBeLessThan(500)

      await page.waitForLoadState('domcontentloaded')
      // If somehow we got redirected to login, fail loudly — storageState should have kept us in.
      expect(page.url(), `redirected to login from ${path}`).not.toContain('/auth/login')

      const bodyText = await page.locator('body').innerText()
      expect(bodyText.trim().length, `empty body on ${path}`).toBeGreaterThan(10)
      expect(errors, `pageerror on ${path}: ${errors.join(' | ')}`).toHaveLength(0)
    })
  }
})
