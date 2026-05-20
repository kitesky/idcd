import { test, expect } from '@playwright/test'

test.describe('Ping probe tool — real submission', () => {
  test('submits a ping task and shows result state', async ({ page }) => {
    const errors: string[] = []
    page.on('pageerror', e => errors.push(e.message))

    await page.goto('/tools/ping')
    await expect(page.locator('h1')).toContainText(/Ping/i)

    // The target input has a Chinese placeholder containing "目标"; key it that way to
    // disambiguate from any future inputs on the page.
    const targetInput = page.getByPlaceholder(/example\.com|1\.1\.1\.1/i).first()
    await targetInput.pressSequentially('1.1.1.1')

    const submitBtn = page.locator('button[type="submit"]', { hasText: /开始拨测|start/i })
    // Wait for the button to enable in response to the controlled input.
    await expect(submitBtn).toBeEnabled({ timeout: 5_000 })

    const submitPromise = page.waitForResponse(
      r => r.url().includes('/v1/probe/') && r.request().method() === 'POST',
      { timeout: 15_000 },
    )
    await submitBtn.click()
    const resp = await submitPromise
    expect(resp.status(), `probe submit status: ${resp.status()}`).toBeLessThan(500)

    // After submission the page should reflect the task — either result or error.
    await page.waitForTimeout(1500)
    const body = await page.locator('body').innerText()
    const reflected = /(任务|task|结果|节点|延迟|失败|错误|error)/i.test(body)
    expect(reflected, 'page did not reflect probe submission state').toBe(true)

    expect(errors, `pageerror: ${errors.join(' | ')}`).toHaveLength(0)
  })

  test('refuses empty target', async ({ page }) => {
    await page.goto('/tools/ping')
    const submitBtn = page.locator('button[type="submit"]', { hasText: /开始拨测|start/i })
    await expect(submitBtn).toBeDisabled()
  })
})
