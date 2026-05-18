import { test, expect } from '@playwright/test'

/**
 * Pure client-side utility tools — input a known value, click the action,
 * verify the deterministic output renders.
 */

test.describe('Utility tools — real interaction', () => {
  test('json-formatter pretty-prints valid JSON', async ({ page }) => {
    await page.goto('/tools/json-formatter')
    const input = page.locator('textarea').first()
    const output = page.locator('textarea[readonly], textarea').nth(1)

    await input.fill('{"a":1,"b":[2,3],"c":{"d":4}}')
    await page.getByRole('button', { name: /^格式化$/ }).click()

    // Output should now contain newlines (pretty-printed).
    const text = await output.inputValue()
    expect(text).toContain('"a": 1')
    expect(text.split('\n').length, 'pretty-print expected multi-line output').toBeGreaterThan(3)
  })

  test('json-formatter surfaces parse errors', async ({ page }) => {
    await page.goto('/tools/json-formatter')
    await page.locator('textarea').first().fill('{not valid json}')
    await page.getByRole('button', { name: /^格式化$/ }).click()
    // Error badge should appear.
    await expect(page.locator('text=/错误/').first()).toBeVisible({ timeout: 3_000 })
  })

  test('base64 encodes then decodes round-trip', async ({ page }) => {
    await page.goto('/tools/base64')
    // Encode tab is default. Enter known text.
    const input = page.locator('textarea').first()
    const output = page.locator('textarea').nth(1)

    await input.fill('hello, idcd 2026')
    // Two "编码" buttons exist: the tab pill and the action button. The action
    // button is the shadcn-styled one (has data-slot="button").
    await page.locator('button[data-slot="button"]').filter({ hasText: '编码' }).first().click()
    const encoded = await output.inputValue()
    expect(encoded.length, 'encoded value should be non-empty').toBeGreaterThan(10)
    // "hello" → "aGVsbG8" prefix in base64.
    expect(encoded.startsWith('aGVsbG')).toBe(true)
  })

  test('hash computes SHA-256 of a known input', async ({ page }) => {
    await page.goto('/tools/hash')
    await page.locator('textarea').first().fill('abc')
    await page.getByRole('button', { name: /计算|hash/i }).first().click()

    // SHA-256("abc") = ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad
    await expect(page.locator('body')).toContainText(/ba7816bf8f01cfea414140de5dae2223/i, { timeout: 5_000 })
  })

  test('jwt-decoder parses a known token', async ({ page }) => {
    await page.goto('/tools/jwt-decoder')
    const token = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c'
    await page.locator('textarea').first().fill(token)

    // Most JWT pages auto-decode on input, but a "decode/解码" button is also common.
    const decodeBtn = page.getByRole('button', { name: /解码|decode/i }).first()
    if (await decodeBtn.isVisible().catch(() => false)) {
      await decodeBtn.click()
    }

    // The decoded payload contains "John Doe" and sub "1234567890".
    await expect(page.locator('body')).toContainText(/John Doe/i, { timeout: 5_000 })
    await expect(page.locator('body')).toContainText(/1234567890/i)
  })

  test('timestamp converts unix epoch to a date', async ({ page }) => {
    await page.goto('/tools/timestamp')
    // 1703980800 = 2023-12-31 08:00:00 UTC
    const input = page.getByPlaceholder(/1703980800|时间戳/i).first()
    await input.fill('1703980800')
    await page.getByRole('button', { name: /^转换$/ }).first().click()
    // Should render "2023" somewhere in the result.
    await expect(page.locator('body')).toContainText(/2023/, { timeout: 5_000 })
  })

  test('regex-tester matches a simple pattern', async ({ page }) => {
    await page.goto('/tools/regex-tester')
    const inputs = page.locator('input, textarea')
    const count = await inputs.count()
    // The first two visible textinputs are pattern + text (the order may vary by layout).
    if (count >= 2) {
      // Pattern \d+ on text 'abc 123 def' → 1 match
      await inputs.nth(0).fill('\\d+')
      await inputs.nth(1).fill('abc 123 def')
      // Either auto-matches on change or has a test button.
      const runBtn = page.getByRole('button', { name: /测试|test|match/i }).first()
      if (await runBtn.isVisible().catch(() => false)) await runBtn.click()
      // The match "123" should appear in output / highlight.
      await expect(page.locator('body')).toContainText(/123/, { timeout: 5_000 })
    }
  })
})
