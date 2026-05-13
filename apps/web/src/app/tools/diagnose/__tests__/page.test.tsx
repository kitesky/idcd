import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import DiagnoseClient from '../diagnose-client'
import * as api from '@/lib/api'

// Mock API functions
vi.mock('@/lib/api', () => ({
  probeDns: vi.fn(),
  probeHttp: vi.fn(),
  probePing: vi.fn(),
  probeTraceroute: vi.fn(),
  getSSLInfo: vi.fn(),
  getWhoisInfo: vi.fn(),
}))

describe('Diagnose Tool', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders diagnose tool heading', () => {
    render(<DiagnoseClient />)
    expect(screen.getByRole('heading', { level: 1 })).toBeTruthy()
    expect(screen.getByText('一键网络诊断')).toBeTruthy()
  })

  it('has domain input field', () => {
    render(<DiagnoseClient />)
    const input = screen.getByLabelText('目标域名')
    expect(input).toBeTruthy()
    expect(input.getAttribute('placeholder')).toBe('example.com')
  })

  it('has start diagnose button', () => {
    render(<DiagnoseClient />)
    const button = screen.getByRole('button', { name: /开始诊断/i })
    expect(button).toBeTruthy()
  })

  it('button is disabled when domain is empty', () => {
    render(<DiagnoseClient />)
    const button = screen.getByRole('button', { name: /开始诊断/i })
    expect(button.hasAttribute('disabled')).toBe(true)
  })

  it('button is enabled when domain is entered', () => {
    render(<DiagnoseClient />)
    const input = screen.getByLabelText('目标域名')
    const button = screen.getByRole('button', { name: /开始诊断/i })

    fireEvent.change(input, { target: { value: 'example.com' } })
    expect(button.hasAttribute('disabled')).toBe(false)
  })

  it('shows check items after diagnosis starts', async () => {
    const mockProbeResult = {
      task_id: 'test-task',
      status: 'completed',
      results: []
    }

    vi.mocked(api.probeDns).mockResolvedValue(mockProbeResult)
    vi.mocked(api.probeHttp).mockResolvedValue(mockProbeResult)
    vi.mocked(api.probePing).mockResolvedValue(mockProbeResult)
    vi.mocked(api.probeTraceroute).mockResolvedValue(mockProbeResult)
    vi.mocked(api.getSSLInfo).mockResolvedValue({
      domain: 'example.com',
      issuer: 'Test CA',
      valid_from: '2024-01-01',
      valid_to: '2025-01-01',
      days_remaining: 100,
      is_valid: true
    })
    vi.mocked(api.getWhoisInfo).mockResolvedValue({
      domain: 'example.com',
      registrar: 'Test Registrar'
    })

    render(<DiagnoseClient />)
    const input = screen.getByLabelText('目标域名')
    const button = screen.getByRole('button', { name: /开始诊断/i })

    fireEvent.change(input, { target: { value: 'example.com' } })
    fireEvent.click(button)

    await waitFor(() => {
      expect(screen.getByText('DNS 解析')).toBeTruthy()
      expect(screen.getByText('HTTPS 可达性')).toBeTruthy()
      expect(screen.getByText('Ping 延迟')).toBeTruthy()
      expect(screen.getByText('Traceroute')).toBeTruthy()
      expect(screen.getByText('SSL 证书')).toBeTruthy()
      expect(screen.getByText('WHOIS 信息')).toBeTruthy()
    })
  })

  it('handles API errors gracefully', async () => {
    vi.mocked(api.probeDns).mockRejectedValue(new Error('DNS 查询失败'))
    vi.mocked(api.probeHttp).mockRejectedValue(new Error('HTTP 请求失败'))
    vi.mocked(api.probePing).mockRejectedValue(new Error('Ping 失败'))
    vi.mocked(api.probeTraceroute).mockRejectedValue(new Error('Traceroute 失败'))
    vi.mocked(api.getSSLInfo).mockRejectedValue(new Error('SSL 查询失败'))
    vi.mocked(api.getWhoisInfo).mockRejectedValue(new Error('WHOIS 查询失败'))

    render(<DiagnoseClient />)
    const input = screen.getByLabelText('目标域名')
    const button = screen.getByRole('button', { name: /开始诊断/i })

    fireEvent.change(input, { target: { value: 'example.com' } })
    fireEvent.click(button)

    await waitFor(() => {
      expect(screen.getByText(/错误/i)).toBeTruthy()
    }, { timeout: 3000 })
  })

  it('renders without crashing', () => {
    const { container } = render(<DiagnoseClient />)
    expect(container.firstChild).toBeTruthy()
  })
})
