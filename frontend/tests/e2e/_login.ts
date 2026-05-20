import type { Page } from '@playwright/test'
import { E2E_USER } from './_constants'

/**
 * UI login: fills the form on /auth/login and waits for the dashboard nav.
 * Uses HttpOnly cookies, so storage state is captured automatically by Playwright.
 */
export async function loginViaUi(page: Page): Promise<void> {
  await page.goto('/auth/login')
  // Form field labels are i18n'd; rely on input types to stay locale-agnostic.
  await page.locator('input[type="email"]').fill(E2E_USER.email)
  await page.locator('input[type="password"]').fill(E2E_USER.password)
  await Promise.all([
    page.waitForURL(/\/app\/dashboard/, { timeout: 15_000 }),
    page.locator('button[type="submit"]').first().click(),
  ])
}
