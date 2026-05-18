import { test, expect } from '@playwright/test'

/**
 * Authenticated: walk the major account-management surface — profile,
 * security (2FA / passkey), sessions. None of these mutate; we only verify
 * the pages render their key controls so future regressions surface here.
 */

test.describe('Account & security surface', () => {
  test('profile page exposes display-name input', async ({ page }) => {
    const errors: string[] = []
    page.on('pageerror', e => errors.push(e.message))

    await page.goto('/app/settings/profile')
    // Display name input — semantic role + a placeholder containing "name".
    const displayName = page.getByPlaceholder(/display name|显示|nickname|昵称/i).first()
    await expect(displayName).toBeVisible({ timeout: 10_000 })

    expect(errors, `pageerror: ${errors.join(' | ')}`).toHaveLength(0)
  })

  test('security page renders 2FA + passkey cards', async ({ page }) => {
    const errors: string[] = []
    page.on('pageerror', e => errors.push(e.message))

    await page.goto('/app/settings/security')
    await expect(page.locator('[data-testid="security-page"]')).toBeVisible({ timeout: 10_000 })
    await expect(page.locator('[data-testid="2fa-card"]')).toBeVisible()
    await expect(page.locator('[data-testid="passkey-card"]')).toBeVisible()
    // At least one of the action buttons must be present.
    const enableBtn = page.locator('[data-testid="btn-enable-2fa"], [data-testid="btn-disable-2fa"]').first()
    await expect(enableBtn).toBeVisible()

    expect(errors, `pageerror: ${errors.join(' | ')}`).toHaveLength(0)
  })

  test('sessions page renders session list', async ({ page }) => {
    const errors: string[] = []
    page.on('pageerror', e => errors.push(e.message))

    await page.goto('/app/settings/sessions')
    await expect(page.locator('[data-testid="sessions-page"]')).toBeVisible({ timeout: 10_000 })
    await expect(page.locator('[data-testid="sessions-card"]')).toBeVisible()

    expect(errors, `pageerror: ${errors.join(' | ')}`).toHaveLength(0)
  })
})

test.describe('Logout flow', () => {
  // Uses a freshly-registered throwaway user so logout doesn't invalidate the
  // shared session token from auth.setup.ts (which is bound to the same JWT jti
  // and would be deny-listed for the rest of the run).
  test('logout clears session and redirects to login', async ({ browser, request }) => {
    const email = `e2e-logout-${Date.now().toString(36)}@idcd.local`
    const password = 'E2eLogout!2026'
    // Register and capture the resulting access_token cookie.
    const reg = await request.post('http://localhost:8080/v1/auth/register', {
      data: { email, password, username: `e2elo${Date.now().toString(36).slice(-6)}` },
      failOnStatusCode: false,
    })
    expect(reg.status(), 'throwaway register failed').toBeLessThan(400)

    const context = await browser.newContext()
    const page = await context.newPage()

    // Login through the UI so HttpOnly cookie is set on the page context.
    await page.goto('/auth/login')
    await page.locator('input[type="email"]').fill(email)
    await page.locator('input[type="password"]').fill(password)
    await Promise.all([
      page.waitForURL(/\/app\/dashboard/, { timeout: 15_000 }),
      page.locator('button[type="submit"]').first().click(),
    ])

    const logoutPromise = page.waitForResponse(
      r => r.url().includes('/v1/auth/logout') && r.request().method() === 'POST',
      { timeout: 10_000 },
    )
    await page.goto('/auth/logout')
    const resp = await logoutPromise
    expect(resp.status(), `logout status: ${resp.status()}`).toBeLessThan(400)

    // After logout, accessing /app/dashboard should redirect to /auth/login.
    await page.goto('/app/dashboard')
    await page.waitForURL(/\/auth\/login/, { timeout: 10_000 })

    await context.close()
  })
})
