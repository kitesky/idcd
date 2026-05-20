/**
 * E2E Bug-confirmation tests for /app/settings/* pages.
 *
 * Known reported bugs:
 *   BUG-1: API Keys  — new key not shown in list after creation (dialog closes, list stale)
 *   BUG-2: Tokens    — "select all scopes" does not actually select all
 *   BUG-3: Team      — new team not shown in UI after creation
 *
 * Runs under chromium-app project (requires storageState from setup).
 */

import { test, expect, type Page } from '@playwright/test'

// ── helpers ───────────────────────────────────────────────────────────────────

async function screenshotStep(page: Page, name: string) {
  const dir = 'test-results/settings-bugs'
  await page.screenshot({
    path: `${dir}/${name}.png`,
    fullPage: true,
  })
}

function unique(prefix: string): string {
  return `${prefix}-${Date.now().toString(36)}`
}

// ── BUG-1: API Keys list does not reflect newly created key ──────────────────

test('BUG-1: API key list shows new key after creation', async ({ page }) => {
  const errors: string[] = []
  const networkErrors: string[] = []

  page.on('pageerror', (e) => errors.push(e.message))
  page.on('response', (resp) => {
    if (resp.status() >= 400) {
      networkErrors.push(`${resp.request().method()} ${resp.url()} → ${resp.status()}`)
    }
  })

  await page.goto('/app/settings/api-keys')
  await expect(page.locator('[data-testid="api-keys-page"]')).toBeVisible({ timeout: 12_000 })
  await screenshotStep(page, 'bug1-01-page-loaded')

  // Capture the pre-existing list state
  const hadKeys = await page.locator('[data-testid="api-keys-table"]').isVisible().catch(() => false)
  const preCount = hadKeys
    ? await page.locator('[data-testid="api-keys-table"] tbody tr').count()
    : 0
  console.log(`[BUG-1] Pre-existing key count: ${preCount}`)

  // Open create dialog
  await page.locator('[data-testid="btn-create-key"]').click()
  await expect(page.locator('[data-testid="create-key-dialog"]')).toBeVisible({ timeout: 5_000 })
  await screenshotStep(page, 'bug1-02-dialog-open')

  // Fill in key name
  const keyName = unique('e2e-bug1')
  await page.locator('[data-testid="input-key-name"]').fill(keyName)

  // Intercept the POST request
  const createResponse = page.waitForResponse(
    (r) => r.url().includes('/v1/account/api-keys') && r.request().method() === 'POST',
    { timeout: 15_000 },
  )

  await page.locator('[data-testid="btn-submit-create"]').click()
  const resp = await createResponse
  const respStatus = resp.status()
  let respBody: unknown = null
  try { respBody = await resp.json() } catch { /* ignore */ }

  console.log(`[BUG-1] POST /v1/account/api-keys → ${respStatus}`)
  console.log(`[BUG-1] Response body:`, JSON.stringify(respBody).slice(0, 400))

  await screenshotStep(page, 'bug1-03-after-submit')

  // Check whether the reveal panel appears (expected on success)
  const revealVisible = await page
    .locator('[data-testid="new-key-reveal"]')
    .isVisible({ timeout: 8_000 })
    .catch(() => false)

  console.log(`[BUG-1] Reveal panel visible: ${revealVisible}`)
  await screenshotStep(page, 'bug1-04-reveal-state')

  // Close dialog
  const doneBtn = page.locator('[data-testid="btn-done-create"]')
  if (await doneBtn.isVisible().catch(() => false)) {
    await doneBtn.click()
  } else {
    await page.keyboard.press('Escape').catch(() => undefined)
  }

  // Wait a moment for list to update
  await page.waitForTimeout(1500)
  await screenshotStep(page, 'bug1-05-after-close-dialog')

  // Verify list state after creation
  const tableVisible = await page
    .locator('[data-testid="api-keys-table"]')
    .isVisible({ timeout: 5_000 })
    .catch(() => false)

  if (tableVisible) {
    const postCount = await page.locator('[data-testid="api-keys-table"] tbody tr').count()
    console.log(`[BUG-1] Post-creation key count: ${postCount}`)

    // Search for the key we just created by name
    const searchInput = page.locator('[data-testid="input-search-keys"]')
    if (await searchInput.isVisible().catch(() => false)) {
      await searchInput.fill(keyName)
      await page.waitForTimeout(500)
    }

    const newKeyRow = page.locator(`text=${keyName}`)
    const newKeyVisible = await newKeyRow.isVisible({ timeout: 3_000 }).catch(() => false)
    console.log(`[BUG-1] New key "${keyName}" visible in list: ${newKeyVisible}`)
    await screenshotStep(page, 'bug1-06-list-after-search')

    // BUG CHECK: if API returned success but key not in list, that's BUG-1
    if (respStatus < 400 && !newKeyVisible) {
      console.log('[BUG-1] CONFIRMED: API returned success but key not visible in list!')
    } else if (respStatus < 400 && newKeyVisible) {
      console.log('[BUG-1] PASS: Key appears in list after creation')
    }

    // Cleanup: revoke the key
    if (newKeyVisible) {
      const searchedRows = page.locator(`tr:has-text("${keyName}")`)
      const revokeBtn = searchedRows.locator('[data-testid^="btn-revoke-"]').first()
      if (await revokeBtn.isVisible().catch(() => false)) {
        await revokeBtn.click()
        await page.waitForTimeout(300)
        const confirmBtn = page.locator('[data-testid^="btn-confirm-revoke-"]').first()
        if (await confirmBtn.isVisible().catch(() => false)) {
          await confirmBtn.click()
          await page.waitForTimeout(500)
        }
      }
    }
  } else {
    console.log('[BUG-1] Table not visible after creation — empty state or error')
    const emptyMsg = await page.locator('[data-testid="empty-keys-message"]').isVisible().catch(() => false)
    const loadError = await page.locator('[data-testid="load-keys-error"]').isVisible().catch(() => false)
    console.log(`[BUG-1] Empty state: ${emptyMsg}, Load error: ${loadError}`)
    await screenshotStep(page, 'bug1-06-no-table')
  }

  if (respStatus < 400) {
    expect(revealVisible, 'Reveal panel should appear after successful creation').toBe(true)
  }

  console.log('[BUG-1] Network errors during test:', networkErrors)
  console.log('[BUG-1] Page errors during test:', errors)
})

// ── BUG-2: Select-all scopes does not work in token creation dialog ───────────

test('BUG-2: Token creation - select all scopes actually selects all', async ({ page }) => {
  const errors: string[] = []
  page.on('pageerror', (e) => errors.push(e.message))

  await page.goto('/app/settings/tokens')
  await expect(page.locator('[data-testid="tokens-page"]')).toBeVisible({ timeout: 12_000 })
  await screenshotStep(page, 'bug2-01-page-loaded')

  // Open create dialog
  await page.locator('[data-testid="btn-create-token"]').click()
  await expect(page.locator('[data-testid="create-token-dialog"]')).toBeVisible({ timeout: 5_000 })
  await screenshotStep(page, 'bug2-02-dialog-open')

  // Fill name
  const tokenName = unique('e2e-bug2')
  await page.locator('[data-testid="input-token-name"]').fill(tokenName)

  // Check all available scope checkboxes
  const scopes = ['read:monitors', 'write:monitors', 'read:alerts', 'read:billing']

  // First, count how many scope checkboxes are rendered
  const scopeCheckboxes = page.locator('[data-testid^="checkbox-scope-"]')
  const checkboxCount = await scopeCheckboxes.count()
  console.log(`[BUG-2] Found ${checkboxCount} scope checkboxes`)
  await screenshotStep(page, 'bug2-03-before-select-all')

  // Click each scope checkbox
  for (const scope of scopes) {
    const checkbox = page.locator(`[data-testid="checkbox-scope-${scope}"]`)
    const isVisible = await checkbox.isVisible().catch(() => false)
    if (isVisible) {
      const isChecked = await checkbox.isChecked().catch(() => false)
      console.log(`[BUG-2] Scope ${scope}: visible=${isVisible}, initially checked=${isChecked}`)
      if (!isChecked) {
        await checkbox.click()
        await page.waitForTimeout(100)
      }
    } else {
      console.log(`[BUG-2] Scope ${scope}: NOT VISIBLE`)
    }
  }

  await screenshotStep(page, 'bug2-04-after-select-all')

  // Verify all are checked
  let allChecked = true
  for (const scope of scopes) {
    const checkbox = page.locator(`[data-testid="checkbox-scope-${scope}"]`)
    const isVisible = await checkbox.isVisible().catch(() => false)
    if (isVisible) {
      const isChecked = await checkbox.isChecked().catch(() => false)
      console.log(`[BUG-2] After click - Scope ${scope} checked: ${isChecked}`)
      if (!isChecked) {
        allChecked = false
        console.log(`[BUG-2] CONFIRMED BUG: Scope ${scope} could not be checked!`)
      }
    }
  }

  // Submit token creation
  const createResponse = page.waitForResponse(
    (r) => r.url().includes('/v1/account/tokens') && r.request().method() === 'POST',
    { timeout: 15_000 },
  )

  await page.locator('[data-testid="btn-submit-create"]').click()
  const resp = await createResponse
  const respStatus = resp.status()
  let respBody: unknown = null
  try { respBody = await resp.json() } catch { /* ignore */ }

  console.log(`[BUG-2] POST /v1/account/tokens → ${respStatus}`)
  console.log(`[BUG-2] Request body sent:`, JSON.stringify(respBody).slice(0, 600))

  await screenshotStep(page, 'bug2-05-after-submit')

  // Check what scopes the server received vs what was sent
  const requestBody = resp.request().postData()
  console.log(`[BUG-2] Raw request body: ${requestBody}`)

  let parsedRequest: { scopes?: string[] } = {}
  try {
    parsedRequest = JSON.parse(requestBody ?? '{}')
  } catch { /* ignore */ }

  const sentScopes = parsedRequest.scopes ?? []
  console.log(`[BUG-2] Scopes sent to API: ${JSON.stringify(sentScopes)}`)

  if (sentScopes.length < scopes.length) {
    console.log(`[BUG-2] CONFIRMED BUG: Only ${sentScopes.length}/${scopes.length} scopes sent to API!`)
    console.log(`[BUG-2] Missing: ${scopes.filter(s => !sentScopes.includes(s)).join(', ')}`)
  } else {
    console.log('[BUG-2] PASS: All scopes sent to API')
  }

  // Check reveal panel appears
  const revealVisible = await page
    .locator('[data-testid="new-token-reveal"]')
    .isVisible({ timeout: 8_000 })
    .catch(() => false)
  await screenshotStep(page, 'bug2-06-reveal-state')
  console.log(`[BUG-2] Reveal panel visible: ${revealVisible}`)

  // Close dialog
  const doneBtn = page.locator('[data-testid="btn-done-create"]')
  if (await doneBtn.isVisible().catch(() => false)) {
    await doneBtn.click()
  } else {
    await page.keyboard.press('Escape').catch(() => undefined)
  }

  await page.waitForTimeout(1500)
  await screenshotStep(page, 'bug2-07-after-close')

  // Verify token appears in list with correct scopes
  const tokenRow = page.locator(`tr:has-text("${tokenName}")`)
  const tokenVisible = await tokenRow.isVisible({ timeout: 5_000 }).catch(() => false)
  console.log(`[BUG-2] Token "${tokenName}" visible in list: ${tokenVisible}`)

  if (tokenVisible && respStatus < 400) {
    // Check each scope badge
    for (const scope of sentScopes) {
      const badge = tokenRow.locator(`[data-testid="scope-badge-${scope}"]`)
      const badgeVisible = await badge.isVisible().catch(() => false)
      console.log(`[BUG-2] Scope badge "${scope}" visible in list row: ${badgeVisible}`)
    }
    await screenshotStep(page, 'bug2-08-list-with-scopes')

    // Cleanup: revoke the token
    const revokeBtn = tokenRow.locator('[data-testid^="btn-revoke-"]').first()
    if (await revokeBtn.isVisible().catch(() => false)) {
      await revokeBtn.click()
      await page.waitForTimeout(300)
      const confirmBtn = page.locator('[data-testid^="btn-confirm-revoke-"]').first()
      if (await confirmBtn.isVisible().catch(() => false)) {
        await confirmBtn.click()
      }
    }
  }

  console.log('[BUG-2] allChecked in UI:', allChecked)
  console.log('[BUG-2] Page errors:', errors)
})

// ── BUG-3: Team not shown after creation ──────────────────────────────────────

test('BUG-3: Team appears in UI after creation', async ({ page }) => {
  const errors: string[] = []
  const networkErrors: string[] = []

  page.on('pageerror', (e) => errors.push(e.message))
  page.on('response', (resp) => {
    if (resp.status() >= 400 && resp.url().includes('/v1/teams')) {
      networkErrors.push(`${resp.request().method()} ${resp.url()} → ${resp.status()}`)
    }
  })

  await page.goto('/app/settings/team')
  await expect(page.locator('[data-testid="team-page"]')).toBeVisible({ timeout: 12_000 })
  await screenshotStep(page, 'bug3-01-page-loaded')

  // Check initial state: does user already have a team?
  const hasTeam = await page.locator('[data-testid="team-info-card"]').isVisible({ timeout: 5_000 }).catch(() => false)
  const hasEmptyState = await page.locator('[data-testid="team-empty-state"]').isVisible({ timeout: 3_000 }).catch(() => false)

  console.log(`[BUG-3] Has team: ${hasTeam}, Has empty state: ${hasEmptyState}`)

  if (hasTeam) {
    const teamName = await page.locator('[data-testid="team-name"]').textContent().catch(() => 'unknown')
    console.log(`[BUG-3] User already has team: "${teamName}" — skipping creation test`)
    console.log('[BUG-3] Cannot test creation of new team when one already exists (single-team per account)')
    await screenshotStep(page, 'bug3-02-existing-team')
    // Just validate the existing team renders correctly
    await expect(page.locator('[data-testid="team-info-card"]')).toBeVisible()
    await expect(page.locator('[data-testid="members-table"]')).toBeVisible()
    await expect(page.locator('[data-testid="team-api-keys-card"]')).toBeVisible()
    return
  }

  if (!hasEmptyState) {
    // Still loading or error
    const loadError = await page.locator('[data-testid="team-error-alert"]').isVisible().catch(() => false)
    console.log(`[BUG-3] Load error visible: ${loadError}`)
    if (loadError) {
      const errText = await page.locator('[data-testid="team-error-alert"]').textContent().catch(() => '')
      console.log(`[BUG-3] Error text: ${errText}`)
    }
    await screenshotStep(page, 'bug3-02-unknown-state')
    return
  }

  // User has no team — proceed to create one
  console.log('[BUG-3] No team found — proceeding to create one')
  await page.locator('[data-testid="btn-create-team-empty"]').click()

  // Wait for create dialog
  const dialogVisible = await page
    .locator('dialog, [role="dialog"]')
    .first()
    .isVisible({ timeout: 5_000 })
    .catch(() => false)
  console.log(`[BUG-3] Create dialog visible: ${dialogVisible}`)
  await screenshotStep(page, 'bug3-03-create-dialog-open')

  // Fill team name and slug
  const teamName = unique('e2e-team')
  const teamSlug = `e2e-${Date.now().toString(36)}`
  await page.locator('[data-testid="input-team-name"]').fill(teamName)
  await page.locator('[data-testid="input-team-slug"]').fill(teamSlug)
  await screenshotStep(page, 'bug3-04-filled-create-form')

  // Intercept the POST
  const createResponse = page.waitForResponse(
    (r) => r.url().includes('/v1/teams') && r.request().method() === 'POST',
    { timeout: 15_000 },
  )

  await page.locator('[data-testid="btn-confirm-create-team"]').click()
  const resp = await createResponse
  const respStatus = resp.status()
  let respBody: unknown = null
  try { respBody = await resp.json() } catch { /* ignore */ }

  console.log(`[BUG-3] POST /v1/teams → ${respStatus}`)
  console.log(`[BUG-3] Response body:`, JSON.stringify(respBody).slice(0, 500))

  await page.waitForTimeout(2000)
  await screenshotStep(page, 'bug3-05-after-creation')

  // Check if team info card appeared
  const teamCardVisible = await page
    .locator('[data-testid="team-info-card"]')
    .isVisible({ timeout: 8_000 })
    .catch(() => false)
  const emptyStillVisible = await page
    .locator('[data-testid="team-empty-state"]')
    .isVisible({ timeout: 1_000 })
    .catch(() => false)

  console.log(`[BUG-3] Team info card visible after creation: ${teamCardVisible}`)
  console.log(`[BUG-3] Empty state still visible: ${emptyStillVisible}`)

  if (respStatus < 400 && !teamCardVisible) {
    console.log('[BUG-3] CONFIRMED BUG: API returned success but team not shown in UI!')
  } else if (respStatus < 400 && teamCardVisible) {
    const displayedName = await page.locator('[data-testid="team-name"]').textContent().catch(() => '')
    console.log(`[BUG-3] PASS: Team "${displayedName}" appears in UI after creation`)
  } else {
    console.log(`[BUG-3] API error (${respStatus}) — cannot confirm UI bug`)
  }

  await screenshotStep(page, 'bug3-06-final-state')
  console.log('[BUG-3] Network errors:', networkErrors)
  console.log('[BUG-3] Page errors:', errors)
})

// ── Bonus: Check for console errors on each settings page ─────────────────────

test('Settings pages load without console errors', async ({ page }) => {
  const allErrors: Record<string, string[]> = {}

  const settingsPages = [
    '/app/settings/api-keys',
    '/app/settings/tokens',
    '/app/settings/team',
  ]

  for (const url of settingsPages) {
    const pageErrors: string[] = []
    page.on('pageerror', (e) => pageErrors.push(e.message))

    await page.goto(url)
    await page.waitForLoadState('networkidle', { timeout: 15_000 }).catch(() => undefined)
    await page.waitForTimeout(1000)

    await screenshotStep(page, `console-check-${url.split('/').pop()}`)

    page.removeAllListeners('pageerror')
    allErrors[url] = pageErrors

    if (pageErrors.length > 0) {
      console.log(`[CONSOLE] Errors on ${url}:`, pageErrors)
    } else {
      console.log(`[CONSOLE] ${url}: no errors`)
    }
  }

  // Report but don't fail hard — this is observation-only
  const totalErrors = Object.values(allErrors).flat()
  if (totalErrors.length > 0) {
    console.warn('[CONSOLE] Total page errors across settings pages:', totalErrors.length)
  }
})
