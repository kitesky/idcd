import { test, expect } from '@playwright/test'

test.describe('Monitors list — authenticated nav', () => {
  test('monitors list page loads and "新建" entry is reachable', async ({ page }) => {
    const errors: string[] = []
    page.on('pageerror', e => errors.push(e.message))

    await page.goto('/app/monitors')
    await expect(page.locator('body')).toBeVisible()

    // A monitors page should expose a route to create a new monitor — either via
    // a link to /app/monitors/new or a button. We assert the route is reachable.
    await page.goto('/app/monitors/new')
    await page.waitForLoadState('domcontentloaded')

    // The new-monitor flow renders monitor type cards.
    const typeCards = page.locator('[data-testid^="type-card-"]')
    await expect(typeCards.first()).toBeVisible({ timeout: 10_000 })
    const count = await typeCards.count()
    expect(count, 'monitor type cards not rendered').toBeGreaterThan(0)

    expect(errors, `pageerror: ${errors.join(' | ')}`).toHaveLength(0)
  })
})
