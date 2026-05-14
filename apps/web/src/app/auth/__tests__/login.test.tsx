import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import LoginPage from '../login/page'

// Mock Next.js router
vi.mock('next/navigation', () => ({
  useRouter: () => ({
    push: vi.fn(),
  }),
}))

describe('LoginPage', () => {
  it('renders login form', () => {
    render(<LoginPage />)

    expect(screen.getByRole('heading', { name: '登录' })).toBeDefined()
    expect(screen.getByLabelText('邮箱')).toBeDefined()
    expect(screen.getByLabelText('密码')).toBeDefined()
    expect(screen.getByRole('button', { name: /登录/ })).toBeDefined()
  })

  it('shows links to register and forgot password pages', () => {
    render(<LoginPage />)

    expect(screen.getByText('忘记密码？')).toBeDefined()
    expect(screen.getByText('立即注册')).toBeDefined()
  })

  it('renders DingTalk login button', () => {
    render(<LoginPage />)
    expect(screen.getByText('钉钉登录')).toBeDefined()
  })

  it('renders Feishu login button', () => {
    render(<LoginPage />)
    expect(screen.getByText('飞书登录')).toBeDefined()
  })
})
