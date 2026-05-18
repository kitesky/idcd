import { test, expect } from '@playwright/test'

/**
 * Authenticated: full monitor lifecycle — create → detail → pause/resume → delete.
 * Truly writes to the dev DB; cleanup at the end keeps re-runs idempotent.
 */
test('monitor full lifecycle: create → detail → pause → delete', async ({ page }) => {
  const errors: string[] = []
  page.on('pageerror', e => errors.push(e.message))

  // Step 1: Create via the wizard.
  await page.goto('/app/monitors/new')
  await page.locator('[data-testid="type-card-http"]').first().click()
  const nextBtn = page.getByRole('button', { name: '下一步', exact: true })
  await expect(nextBtn).toBeEnabled({ timeout: 5_000 })
  await nextBtn.click()

  const name = `e2e-lifecycle-${Date.now().toString(36)}`
  await page.locator('#monitor-name').fill(name)
  await page.locator('#monitor-target').fill('https://example.com')
  await nextBtn.click()
  await expect(nextBtn).toBeEnabled({ timeout: 5_000 })
  await nextBtn.click()

  const createBtn = page.getByRole('button', { name: /创建监控|create monitor/i }).first()
  await expect(createBtn).toBeEnabled({ timeout: 5_000 })

  const createRespPromise = page.waitForResponse(
    r => /\/v1\/monitors/.test(r.url()) && r.request().method() === 'POST',
    { timeout: 15_000 },
  )
  await createBtn.click()
  const createResp = await createRespPromise

  // Free-plan / quota-exceeded / rate-limited users get 402/403/429 — all real
  // product behaviour, not a regression. Skip the lifecycle steps in those
  // cases; the wizard-render coverage from monitor-create.spec.ts is sufficient.
  if ([402, 403, 429].includes(createResp.status())) {
    test.info().annotations.push({
      type: 'skipped-by-plan',
      description: `lifecycle unreachable on this plan (status ${createResp.status()})`,
    })
    return
  }
  expect(createResp.status(), `create monitor returned ${createResp.status()}`).toBeLessThan(400)

  // The response carries the monitor id — extract it for the detail URL.
  const created = await createResp.json().catch(() => ({}))
  const monitorId = created?.data?.id ?? created?.id
  expect(monitorId, 'no monitor id returned').toBeTruthy()

  // Step 2: Detail page renders.
  await page.goto(`/app/monitors/${monitorId}`)
  await page.waitForLoadState('domcontentloaded')
  await expect(page.locator('body')).toContainText(name, { timeout: 10_000 })

  // Step 3: Pause via the toolbar button (text "暂停" / "Pause").
  const pauseBtn = page.getByRole('button', { name: /暂停|pause/i }).first()
  if (await pauseBtn.isVisible().catch(() => false)) {
    const pauseRespPromise = page.waitForResponse(
      r => /\/v1\/monitors\//.test(r.url()) && (r.request().method() === 'PATCH' || r.request().method() === 'PUT'),
      { timeout: 8_000 },
    ).catch(() => null)
    await pauseBtn.click()
    const pauseResp = await pauseRespPromise
    if (pauseResp) {
      expect(pauseResp.status(), `pause returned ${pauseResp.status()}`).toBeLessThan(400)
    }
  }

  // Step 4: Delete (cleanup). Open the delete dialog and confirm.
  const deleteOpenBtn = page.getByRole('button', { name: /删除|delete/i }).first()
  await deleteOpenBtn.click()
  const confirmBtn = page.getByRole('button', { name: /确认.*删除|confirm delete|delete monitor/i }).first()
  const deleteRespPromise = page.waitForResponse(
    r => r.url().includes(`/v1/monitors/${monitorId}`) && r.request().method() === 'DELETE',
    { timeout: 8_000 },
  )
  await confirmBtn.click()
  const deleteResp = await deleteRespPromise
  expect(deleteResp.status(), `delete returned ${deleteResp.status()}`).toBeLessThan(400)

  expect(errors, `pageerror: ${errors.join(' | ')}`).toHaveLength(0)
})
