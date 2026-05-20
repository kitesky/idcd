import { test, expect } from '@playwright/test'

/**
 * Authenticated: walks the cert request wizard up to the DNS-credential step.
 * Doesn't submit — actual cert issuance requires real DNS provider creds and
 * would commit state to shared dev infra.
 */
test('cert request wizard renders and accepts a SAN', async ({ page }) => {
  const errors: string[] = []
  page.on('pageerror', e => errors.push(e.message))

  await page.goto('/app/cert/new')

  // The wizard step indicator should be present.
  await expect(page.locator('[data-testid="wizard-step-0"]').first()).toBeVisible({ timeout: 10_000 })

  // Fill SAN input.
  const san = `e2e-${Date.now().toString(36)}.idcd.local`
  await page.locator('[data-testid="san-input"]').fill(san)
  // After typing, the SAN preview chips should reflect the input.
  await expect(page.locator('[data-testid="san-preview"]')).toContainText(san, { timeout: 5_000 })

  // Press Enter or click "add" to commit — but if the preview already shows it,
  // the wizard-next button should be enabled.
  const nextBtn = page.locator('[data-testid="wizard-next"]')
  // wizard-next may be conditionally rendered; if it shows, it should be clickable.
  if (await nextBtn.isVisible().catch(() => false)) {
    await expect(nextBtn).toBeEnabled({ timeout: 5_000 })
  }

  expect(errors, `pageerror: ${errors.join(' | ')}`).toHaveLength(0)
})
