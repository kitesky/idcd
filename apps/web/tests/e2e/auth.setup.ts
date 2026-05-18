import { test as setup } from '@playwright/test'
import { E2E_USER } from './_constants'

const authFile = 'tests/e2e/.auth/user.json'

setup('authenticate', async ({ page }) => {
  await page.goto('/auth/login')
  await page.locator('input[type="email"]').fill(E2E_USER.email)
  await page.locator('input[type="password"]').fill(E2E_USER.password)
  await Promise.all([
    page.waitForURL(/\/app\/dashboard/, { timeout: 15_000 }),
    page.locator('button[type="submit"]').first().click(),
  ])
  await page.context().storageState({ path: authFile })
})
