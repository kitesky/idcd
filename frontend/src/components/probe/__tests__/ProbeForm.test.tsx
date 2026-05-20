import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import '@testing-library/jest-dom'
import ProbeForm from '../ProbeForm'

describe('ProbeForm', () => {
  it('renders with correct placeholder for http type', () => {
    const mockSubmit = vi.fn()
    render(<ProbeForm type="http" onSubmit={mockSubmit} />)

    const input = screen.getByPlaceholderText(/https:\/\/example.com/)
    expect(input).toBeInTheDocument()
  })

  it('renders with correct placeholder for ping type', () => {
    const mockSubmit = vi.fn()
    render(<ProbeForm type="ping" onSubmit={mockSubmit} />)

    const input = screen.getByPlaceholderText(/example.com 或 1.1.1.1/)
    expect(input).toBeInTheDocument()
  })

  it('renders with correct placeholder for tcp type', () => {
    const mockSubmit = vi.fn()
    render(<ProbeForm type="tcp" onSubmit={mockSubmit} />)

    const input = screen.getByPlaceholderText(/example.com:443/)
    expect(input).toBeInTheDocument()
  })

  it('disables submit button when target is empty', () => {
    const mockSubmit = vi.fn()
    render(<ProbeForm type="http" onSubmit={mockSubmit} />)

    const button = screen.getByRole('button', { name: /开始拨测/ })
    expect(button).toBeDisabled()
  })

  it('enables submit button when valid target is entered', () => {
    const mockSubmit = vi.fn()
    render(<ProbeForm type="http" onSubmit={mockSubmit} />)

    const input = screen.getByPlaceholderText(/https:\/\/example.com/)
    fireEvent.change(input, { target: { value: 'https://example.com' } })

    const button = screen.getByRole('button', { name: /开始拨测/ })
    expect(button).not.toBeDisabled()
  })

  it('calls onSubmit with correct params for http', () => {
    const mockSubmit = vi.fn()
    render(<ProbeForm type="http" onSubmit={mockSubmit} />)

    const input = screen.getByPlaceholderText(/https:\/\/example.com/)
    fireEvent.change(input, { target: { value: 'https://example.com' } })

    const button = screen.getByRole('button', { name: /开始拨测/ })
    fireEvent.click(button)

    expect(mockSubmit).toHaveBeenCalledWith('https://example.com', expect.objectContaining({
      method: 'GET',
      follow_redirect: true
    }))
  })

  it('shows loading state when loading prop is true', () => {
    const mockSubmit = vi.fn()
    render(<ProbeForm type="http" onSubmit={mockSubmit} loading={true} />)

    const button = screen.getByRole('button', { name: /拨测进行中/ })
    expect(button).toBeDisabled()
  })

  it('renders method selector for http type', () => {
    const mockSubmit = vi.fn()
    render(<ProbeForm type="http" onSubmit={mockSubmit} />)

    expect(screen.getByText('请求方法')).toBeInTheDocument()
  })

  it('renders count selector for ping type', () => {
    const mockSubmit = vi.fn()
    render(<ProbeForm type="ping" onSubmit={mockSubmit} />)

    expect(screen.getByText('发送次数')).toBeInTheDocument()
  })

  it('renders record type selector for dns type', () => {
    const mockSubmit = vi.fn()
    render(<ProbeForm type="dns" onSubmit={mockSubmit} />)

    expect(screen.getByText('记录类型')).toBeInTheDocument()
  })
})
