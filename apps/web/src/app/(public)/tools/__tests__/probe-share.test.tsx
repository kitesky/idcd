import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'

// Mock next/navigation before importing the component
const mockGet = vi.fn()
vi.mock('next/navigation', () => ({
  useSearchParams: () => ({ get: mockGet }),
}))

// Import after mock is set up
import ProbeToolClient from '../[slug]/probe-client'

describe('ProbeToolClient — 分享链接功能', () => {
  beforeEach(() => {
    mockGet.mockReturnValue(null)

    // Mock clipboard API
    Object.assign(navigator, {
      clipboard: {
        writeText: vi.fn().mockResolvedValue(undefined),
      },
    })

    // Mock window.location
    Object.defineProperty(window, 'location', {
      value: {
        origin: 'https://idcd.com',
        pathname: '/tools/ssl',
        href: 'https://idcd.com/tools/ssl',
      },
      writable: true,
    })
  })

  it('renders the copy-link button', () => {
    render(<ProbeToolClient slug="ssl" />)
    const btn = screen.getByTestId('copy-link-button')
    expect(btn).toBeTruthy()
    expect(btn.textContent).toContain('复制链接')
  })

  it('pre-fills target input from ?target= query param', () => {
    mockGet.mockImplementation((key: string) =>
      key === 'target' ? 'example.com' : null
    )
    render(<ProbeToolClient slug="ssl" />)
    const input = screen.getByRole('textbox', { name: /域名/i })
    expect((input as HTMLInputElement).value).toBe('example.com')
  })

  it('copy-link button is disabled when target is empty', () => {
    render(<ProbeToolClient slug="ssl" />)
    const btn = screen.getByTestId('copy-link-button')
    expect((btn as HTMLButtonElement).disabled).toBe(true)
  })

  it('copy-link button is enabled when target is filled', () => {
    render(<ProbeToolClient slug="ssl" />)
    const input = screen.getByRole('textbox', { name: /域名/i })
    fireEvent.change(input, { target: { value: 'example.com' } })
    const btn = screen.getByTestId('copy-link-button')
    expect((btn as HTMLButtonElement).disabled).toBe(false)
  })

  it('clicking copy-link writes correct URL to clipboard', async () => {
    render(<ProbeToolClient slug="ssl" />)
    const input = screen.getByRole('textbox', { name: /域名/i })
    fireEvent.change(input, { target: { value: 'example.com' } })
    const btn = screen.getByTestId('copy-link-button')
    fireEvent.click(btn)
    expect(navigator.clipboard.writeText).toHaveBeenCalledWith(
      'https://idcd.com/tools/ssl?target=example.com'
    )
  })

  it('button text changes to 已复制！ after click', async () => {
    render(<ProbeToolClient slug="ssl" />)
    const input = screen.getByRole('textbox', { name: /域名/i })
    fireEvent.change(input, { target: { value: 'example.com' } })
    const btn = screen.getByTestId('copy-link-button')
    fireEvent.click(btn)
    expect(btn.textContent).toContain('已复制！')
  })
})
