import { test, expect } from '@playwright/test'

/**
 * Authenticated: this spec runs under the chromium-app project which uses
 * storageState. Walks the full create-api-key dialog flow and asserts that
 * the new key appears in the dialog reveal.
 */
test('user can create an API key via the settings dialog', async ({ page }) => {
  const errors: string[] = []
  page.on('pageerror', e => errors.push(e.message))

  await page.goto('/app/settings/api-keys')
  await expect(page.locator('[data-testid="api-keys-page"]')).toBeVisible({ timeout: 10_000 })

  // Open the create dialog.
  await page.locator('[data-testid="btn-create-key"]').click()
  await expect(page.locator('[data-testid="create-key-dialog"]')).toBeVisible()

  // Fill the name with a unique suffix so re-runs don't collide.
  const keyName = `e2e-test-${Date.now().toString(36)}`
  await page.locator('[data-testid="input-key-name"]').fill(keyName)

  // Submit. Wait for the POST to complete.
  const createPromise = page.waitForResponse(
    r => r.url().includes('/v1/account/api-keys') && r.request().method() === 'POST',
    { timeout: 15_000 },
  )
  await page.locator('[data-testid="btn-submit-create"]').click()
  const resp = await createPromise
  expect(resp.status(), `create key status: ${resp.status()}`).toBeLessThan(400)

  // After success, the reveal panel must show the newly-issued key.
  await expect(page.locator('[data-testid="new-key-reveal"]')).toBeVisible({ timeout: 10_000 })

  expect(errors, `pageerror: ${errors.join(' | ')}`).toHaveLength(0)

  // Cleanup: close dialog and revoke the key we just created so re-runs don't
  // hit the per-account API key quota (5–6 keys is enough to start getting 403).
  // The dialog has a close button after reveal; the row gets a revoke control.
  await page.keyboard.press('Escape').catch(() => undefined)
  await page.waitForTimeout(300)
  const revokeBtn = page.locator(`[data-testid^="btn-revoke-"]`).first()
  if (await revokeBtn.isVisible().catch(() => false)) {
    await revokeBtn.click()
    const confirmBtn = page.locator('[data-testid^="btn-confirm-revoke-"]').first()
    if (await confirmBtn.isVisible().catch(() => false)) {
      await confirmBtn.click()
    }
  }
})
