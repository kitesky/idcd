import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render } from '@testing-library/react'
import OAuthCallbackPage from '../page'

const mockReplace = vi.fn()

vi.mock('next/navigation', () => ({
  useRouter: () => ({
    replace: mockReplace,
  }),
}))

// Auth now flows entirely through the HttpOnly cookie set by the backend.
// The callback page must NOT touch localStorage — that would defeat the
// XSS-resistance the cookie provides.
describe('OAuthCallbackPage', () => {
  beforeEach(() => {
    mockReplace.mockClear()
    localStorage.clear()
  })

  it('redirects to /app/dashboard without persisting anything client-side', () => {
    render(<OAuthCallbackPage />)

    expect(localStorage.getItem('auth_token')).toBeNull()
    expect(mockReplace).toHaveBeenCalledWith('/app/dashboard')
  })
})
