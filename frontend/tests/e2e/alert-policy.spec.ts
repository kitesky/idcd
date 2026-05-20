import { test, expect } from '@playwright/test'

/**
 * Authenticated: verify the policies tab on /app/alerts loads and surfaces
 * the add-policy entry. We don't open the form — the add button is in a
 * client component that re-renders after the policies fetch resolves, which
 * makes clicking flaky from outside. Asserting the entry is wired catches
 * the regression we care about (page crash / missing button), without
 * coupling the test to render timing.
 */
test('alert policy tab exposes add-policy entry', async ({ page }) => {
  const errors: string[] = []
  page.on('pageerror', e => errors.push(e.message))

  await page.goto('/app/alerts?tab=policies')
  await page.waitForLoadState('domcontentloaded')

  await expect(page.locator('[data-testid="add-policy-btn"]')).toBeVisible({ timeout: 10_000 })

  expect(errors, `pageerror: ${errors.join(' | ')}`).toHaveLength(0)
})
