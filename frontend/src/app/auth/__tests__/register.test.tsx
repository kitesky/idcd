import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import RegisterPage from '../register/page'

// Mock Next.js router
vi.mock('next/navigation', () => ({
  useRouter: () => ({
    push: vi.fn(),
  }),
}))

describe('RegisterPage', () => {
  it('renders register form', () => {
    render(<RegisterPage />)

    expect(screen.getByText('注册账号')).toBeDefined()
    expect(screen.getByLabelText('邮箱')).toBeDefined()
    expect(screen.getByLabelText('密码')).toBeDefined()
    expect(screen.getByLabelText('确认密码')).toBeDefined()
    expect(screen.getByRole('button', { name: /注册/ })).toBeDefined()
  })

  it('shows link to login page', () => {
    render(<RegisterPage />)

    const loginLink = screen.getByText('立即登录')
    expect(loginLink).toBeDefined()
    expect(loginLink.closest('a')).toHaveProperty('href')
  })
})
