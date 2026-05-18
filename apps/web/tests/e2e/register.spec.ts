import { test, expect } from '@playwright/test'

test.describe('Register flow', () => {
  test('new user can register and is redirected to verify-email', async ({ page }) => {
    const errors: string[] = []
    page.on('pageerror', e => errors.push(e.message))

    const email = `e2e-reg-${Date.now().toString(36)}@idcd.local`
    const password = 'E2eTest!2026'

    await page.goto('/auth/register')
    await page.locator('input[type="email"]').fill(email)
    const pwInputs = page.locator('input[type="password"]')
    await pwInputs.nth(0).fill(password)
    await pwInputs.nth(1).fill(password)

    const submitPromise = page.waitForResponse(
      r => r.url().includes('/v1/auth/register') && r.request().method() === 'POST',
      { timeout: 15_000 },
    )
    await page.locator('button[type="submit"]').first().click()
    const resp = await submitPromise
    expect(resp.status(), `register status: ${resp.status()}`).toBeLessThan(400)

    // The page redirects to /auth/verify-email?email=...
    await page.waitForURL(/\/auth\/verify-email/, { timeout: 10_000 })
    expect(page.url()).toContain(encodeURIComponent(email))

    expect(errors, `pageerror: ${errors.join(' | ')}`).toHaveLength(0)
  })

  test('duplicate email surfaces an error', async ({ page }) => {
    await page.goto('/auth/register')
    // e2e-test@idcd.local already exists from auth.setup.ts.
    await page.locator('input[type="email"]').fill('e2e-test@idcd.local')
    const pwInputs = page.locator('input[type="password"]')
    await pwInputs.nth(0).fill('E2eTest!2026')
    await pwInputs.nth(1).fill('E2eTest!2026')
    await page.locator('button[type="submit"]').first().click()

    // Server returns 4xx — the form should render an error alert.
    const errorBox = page.locator('.bg-destructive\\/15, [role="alert"]').first()
    await expect(errorBox).toBeVisible({ timeout: 10_000 })
  })

  test('password confirmation mismatch is blocked client-side', async ({ page }) => {
    await page.goto('/auth/register')
    await page.locator('input[type="email"]').fill('mismatch@idcd.local')
    const pwInputs = page.locator('input[type="password"]')
    await pwInputs.nth(0).fill('Password!2026')
    await pwInputs.nth(1).fill('Different!2026')
    await page.locator('button[type="submit"]').first().click()
    // No /v1/auth/register POST should fire when client validation fails.
    await page.waitForTimeout(800)
    // Form should still be on the register page.
    expect(page.url()).toContain('/auth/register')
  })
})
