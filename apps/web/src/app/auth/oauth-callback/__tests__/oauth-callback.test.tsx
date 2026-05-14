import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render } from '@testing-library/react'
import OAuthCallbackPage from '../page'

const mockReplace = vi.fn()
const mockGet = vi.fn()

vi.mock('next/navigation', () => ({
  useRouter: () => ({
    replace: mockReplace,
  }),
  useSearchParams: () => ({
    get: mockGet,
  }),
}))

describe('OAuthCallbackPage', () => {
  beforeEach(() => {
    mockReplace.mockClear()
    mockGet.mockClear()
    localStorage.clear()
  })

  it('stores token and redirects to dashboard', () => {
    mockGet.mockReturnValue('test.jwt.token')

    render(<OAuthCallbackPage />)

    expect(localStorage.getItem('auth_token')).toBe('test.jwt.token')
    expect(mockReplace).toHaveBeenCalledWith('/app/dashboard')
  })

  it('redirects to dashboard even when no token present', () => {
    mockGet.mockReturnValue(null)

    render(<OAuthCallbackPage />)

    expect(mockReplace).toHaveBeenCalledWith('/app/dashboard')
  })
})
