import { test, expect } from '@playwright/test'

/**
 * Authenticated: opens the DNS credentials page, opens the new-cred sheet,
 * verifies form renders. We don't actually submit a real cred — that would
 * write provider secrets to the dev DB and require a valid API key/secret.
 */
test('cert DNS credentials page opens new-cred form', async ({ page }) => {
  const errors: string[] = []
  page.on('pageerror', e => errors.push(e.message))

  await page.goto('/app/cert/dns-credentials')
  // The page should render — find the "open-new-cred" button or its sheet.
  const openBtn = page.locator('[data-testid="open-new-cred"]')
  await expect(openBtn).toBeVisible({ timeout: 10_000 })
  await openBtn.click()

  // The credential name input has placeholder "例如：Cloudflare 主账号".
  const nameInput = page.getByPlaceholder(/Cloudflare|凭据|主账号/)
  await expect(nameInput.first()).toBeVisible({ timeout: 5_000 })

  expect(errors, `pageerror: ${errors.join(' | ')}`).toHaveLength(0)
})
