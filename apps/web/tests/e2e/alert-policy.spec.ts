import { test, expect } from '@playwright/test'

/**
 * Authenticated: navigate to alerts, open the policy form, verify the form
 * fields exist. Creating a policy requires an existing monitor and channels
 * we don't seed in dev, so we stop at form render — enough to catch wiring
 * bugs without committing test data.
 */
test('alert policy form opens with monitor selector', async ({ page }) => {
  const errors: string[] = []
  page.on('pageerror', e => errors.push(e.message))

  await page.goto('/app/alerts')
  await page.waitForLoadState('domcontentloaded')

  // The page has tabs — find the policies tab. Text might be "策略" / "Policies"
  // or a tab with name matching that. Switch only if not already on it.
  const policiesTab = page.getByRole('tab', { name: /策略|polic/i }).first()
  if (await policiesTab.isVisible().catch(() => false)) {
    await policiesTab.click()
  }

  // The add-policy button surfaces in the policies tab.
  const addBtn = page.locator('[data-testid="add-policy-btn"]')
  await expect(addBtn).toBeVisible({ timeout: 10_000 })
  await addBtn.click()

  // The form has a name input (id="policy-name") and monitor select.
  await expect(page.locator('#policy-name')).toBeVisible({ timeout: 5_000 })
  await expect(page.locator('[data-testid="policy-monitor-select"]')).toBeVisible()

  expect(errors, `pageerror: ${errors.join(' | ')}`).toHaveLength(0)
})
