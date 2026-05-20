import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { Suspense } from 'react'
import VerifyEmailPage from '../verify-email/page'

// Mock Next.js navigation
vi.mock('next/navigation', () => ({
  useRouter: () => ({
    push: vi.fn(),
  }),
  useSearchParams: () => ({
    get: (key: string) => {
      const params: Record<string, string> = {
        email: 'test@example.com',
        otp_id: 'test-otp-uuid',
      }
      return params[key] ?? null
    },
  }),
}))

// Mock apiRequest
vi.mock('@/lib/api', () => ({
  apiRequest: vi.fn().mockResolvedValue({ data: {} }),
}))

function renderPage() {
  return render(
    <Suspense fallback={<div>加载中...</div>}>
      <VerifyEmailPage />
    </Suspense>
  )
}

describe('VerifyEmailPage', () => {
  it('renders the verify-email-page container', () => {
    renderPage()
    expect(screen.getByTestId('verify-email-page')).toBeDefined()
  })

  it('renders the 6-digit code input', () => {
    renderPage()
    const input = screen.getByTestId('verify-code-input')
    expect(input).toBeDefined()
  })

  it('renders the verify submit button', () => {
    renderPage()
    expect(screen.getByTestId('verify-submit-btn')).toBeDefined()
  })

  it('renders the resend button', () => {
    renderPage()
    expect(screen.getByTestId('resend-btn')).toBeDefined()
  })

  it('renders the skip-verify link pointing to /app/dashboard', () => {
    renderPage()
    const link = screen.getByTestId('skip-verify-link')
    expect(link).toBeDefined()
    expect(link.getAttribute('href')).toContain('/app/dashboard')
  })

  it('displays the email in the page description', () => {
    renderPage()
    expect(screen.getByText(/test@example\.com/)).toBeDefined()
  })

  it('code input has numeric inputMode and maxLength of 6', () => {
    renderPage()
    const input = screen.getByTestId('verify-code-input') as HTMLInputElement
    expect(input.getAttribute('inputmode')).toBe('numeric')
    expect(input.getAttribute('maxlength')).toBe('6')
  })
})
