import { test, expect } from '@playwright/test'

/**
 * Authenticated: validate the alert events surface renders its terminal
 * states. The dev user typically has no firing events, so we assert
 * either the skeleton resolves to an empty-state, the error path, or a
 * populated table — and that the per-row ack button is wired where rows exist.
 */
test('alert events page settles to a known terminal state', async ({ page }) => {
  const errors: string[] = []
  page.on('pageerror', e => errors.push(e.message))

  await page.goto('/app/alerts')
  await page.waitForLoadState('domcontentloaded')

  // The events surface is on the default tab; if there's an explicit "事件/Events" tab, switch.
  // The tab text in zh-CN is "事件历史"; English is "Events".
  const eventsTab = page.getByRole('tab', { name: /事件历史|事件|events/i }).first()
  if (await eventsTab.isVisible().catch(() => false)) {
    await eventsTab.click()
  }

  // Wait for the events skeleton to resolve into one of the terminal states.
  await expect(async () => {
    const states = await Promise.all([
      page.locator('[data-testid="events-empty"]').isVisible().catch(() => false),
      page.locator('[data-testid^="event-row-"]').first().isVisible().catch(() => false),
      page.locator('[data-testid="events-error"]').isVisible().catch(() => false),
    ])
    expect(states.filter(Boolean).length).toBeGreaterThan(0)
  }).toPass({ timeout: 10_000 })

  // If there are rows, an ack button must exist on at least the first row.
  const rows = page.locator('[data-testid^="event-row-"]')
  if ((await rows.count()) > 0) {
    const ackBtn = page.locator('[data-testid^="ack-btn-"]').first()
    await expect(ackBtn).toBeAttached({ timeout: 3_000 })
  }

  expect(errors, `pageerror: ${errors.join(' | ')}`).toHaveLength(0)
})

test('alert channels tab exposes "add channel" entry', async ({ page }) => {
  const errors: string[] = []
  page.on('pageerror', e => errors.push(e.message))

  await page.goto('/app/alerts')
  await page.waitForLoadState('domcontentloaded')

  // The tab text in zh-CN is "告警通道"; English is "Channels".
  const channelsTab = page.getByRole('tab', { name: /告警通道|channel/i }).first()
  if (await channelsTab.isVisible().catch(() => false)) {
    await channelsTab.click()
  }

  const addChannelBtn = page.locator('[data-testid="add-channel-btn"]')
  await expect(addChannelBtn).toBeVisible({ timeout: 10_000 })

  expect(errors, `pageerror: ${errors.join(' | ')}`).toHaveLength(0)
})
