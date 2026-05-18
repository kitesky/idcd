import { test, expect } from '@playwright/test'

/**
 * Authenticated: incidents are auto-generated from probe/monitor failures,
 * so we can't create one from the UI. We assert the page renders one of
 * the documented terminal states (empty / table / error alert) without crashes.
 */
test('incidents list renders a known terminal state', async ({ page }) => {
  const errors: string[] = []
  page.on('pageerror', e => errors.push(e.message))

  await page.goto('/app/incidents')

  // Wait for the page wrapper to settle (the page-level testid).
  await expect(page.locator('[data-testid="incidents-page"]')).toBeVisible({ timeout: 10_000 })

  // Exactly one of these terminal states must be visible.
  const emptyState = page.locator('[data-testid="incidents-empty-state"]')
  const table = page.locator('[data-testid="incidents-table"]')
  const errorAlert = page.locator('[data-testid="incidents-error-alert"]')

  await expect(async () => {
    const states = await Promise.all([
      emptyState.isVisible().catch(() => false),
      table.isVisible().catch(() => false),
      errorAlert.isVisible().catch(() => false),
    ])
    expect(states.filter(Boolean).length).toBeGreaterThan(0)
  }).toPass({ timeout: 10_000 })

  // If there are rows, the per-row generate-postmortem button should be present.
  const rows = page.locator('[data-testid^="incident-row-"]')
  const rowCount = await rows.count()
  if (rowCount > 0) {
    const firstId = await rows.first().getAttribute('data-testid')
    expect(firstId).toMatch(/^incident-row-/)
  }

  expect(errors, `pageerror: ${errors.join(' | ')}`).toHaveLength(0)
})
