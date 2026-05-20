import { test, expect } from '@playwright/test'

/**
 * Authenticated: walks the create-status-page entry. The button behaviour
 * depends on plan:
 *   - Free plan → opens an "upgrade to Pro" dialog (real product behaviour)
 *   - Pro+      → opens the create sheet, which we fill and submit
 * We detect which path opened AFTER click and assert the right thing.
 */
test('status page create entry handles free vs paid plan correctly', async ({ page }) => {
  const errors: string[] = []
  page.on('pageerror', e => errors.push(e.message))

  await page.goto('/app/status-pages')
  await expect(page.locator('[data-testid="status-pages-page"]')).toBeVisible({ timeout: 10_000 })

  await page.locator('[data-testid="new-page-button"]').click()
  // Whichever appears first wins. Both selectors take up to 5s.
  const sheetInput = page.locator('[data-testid="sp-name-input"]')
  const upgradeHint = page.locator('text=/升级|upgrade to Pro|升级到 Pro/i').first()

  await Promise.race([
    sheetInput.waitFor({ state: 'visible', timeout: 6_000 }),
    upgradeHint.waitFor({ state: 'visible', timeout: 6_000 }),
  ])

  const sheetOpened = await sheetInput.isVisible().catch(() => false)
  const upgradeOpened = await upgradeHint.isVisible().catch(() => false)
  expect(sheetOpened || upgradeOpened, 'neither create sheet nor upgrade dialog appeared').toBe(true)

  if (sheetOpened) {
    // Paid plan: complete the create flow.
    const ts = Date.now().toString(36)
    const name = `e2e-status-${ts}`
    const slug = `e2e-${ts}`
    await sheetInput.fill(name)
    await page.locator('[data-testid="sp-slug-input"]').fill(slug)
    await page.locator('[data-testid="sp-desc-input"]').fill('e2e test status page')

    const respPromise = page.waitForResponse(
      r => /\/v1\/status-pages/.test(r.url()) && r.request().method() === 'POST',
      { timeout: 15_000 },
    )
    await page.locator('[data-testid="create-sp-button"]').click()
    const resp = await respPromise
    expect(resp.status(), `create status page returned ${resp.status()}`).toBeLessThan(400)

    await expect(
      page.locator('[data-testid="status-pages-list"]').getByText(name),
    ).toBeVisible({ timeout: 10_000 })
  }
  // Free-plan branch: nothing more to do — the dialog appearing is the assertion.

  expect(errors, `pageerror: ${errors.join(' | ')}`).toHaveLength(0)
})
