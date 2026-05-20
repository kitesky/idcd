import { render, screen, fireEvent, act } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import DiagnoseClient from '../diagnose-client'

// Mock next/navigation
const mockPush = vi.fn()
vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: mockPush }),
}))

// EventSource mock factory — recreated for each test
type MessageHandler = (event: { data: string }) => void
type ErrorHandler = () => void

interface MockES {
  url: string
  onmessage: MessageHandler | null
  onerror: ErrorHandler | null
  close: ReturnType<typeof vi.fn>
  /** Test helper: emit a raw SSE data payload */
  emit: (data: object) => void
}

let currentES: MockES | null = null

class FakeEventSource {
  url: string
  onmessage: MessageHandler | null = null
  onerror: ErrorHandler | null = null
  close = vi.fn()

  constructor(url: string) {
    this.url = url
    currentES = this as unknown as MockES
  }

  emit(data: object) {
    this.onmessage?.({ data: JSON.stringify(data) })
  }
}

describe('DiagnoseClient', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    currentES = null
    // @ts-expect-error — browser API not in Node
    global.EventSource = FakeEventSource
  })

  afterEach(() => {
    currentES = null
  })

  it('renders heading and domain input', () => {
    render(<DiagnoseClient />)
    expect(screen.getByRole('heading', { level: 1 })).toBeTruthy()
    expect(screen.getByText('一键网络诊断')).toBeTruthy()
    expect(screen.getByLabelText('目标域名')).toBeTruthy()
  })

  it('has start diagnose button', () => {
    render(<DiagnoseClient />)
    expect(screen.getByRole('button', { name: /开始诊断/i })).toBeTruthy()
  })

  it('button is disabled when domain is empty', () => {
    render(<DiagnoseClient />)
    const button = screen.getByRole('button', { name: /开始诊断/i })
    expect(button.hasAttribute('disabled')).toBe(true)
  })

  it('button is enabled when domain is entered', () => {
    render(<DiagnoseClient />)
    const input = screen.getByLabelText('目标域名')
    fireEvent.change(input, { target: { value: 'example.com' } })
    expect(screen.getByRole('button', { name: /开始诊断/i }).hasAttribute('disabled')).toBe(false)
  })

  it('opens EventSource on diagnose click', () => {
    render(<DiagnoseClient />)
    fireEvent.change(screen.getByLabelText('目标域名'), { target: { value: 'example.com' } })
    fireEvent.click(screen.getByRole('button', { name: /开始诊断/i }))
    expect(currentES).not.toBeNull()
    expect(currentES!.url).toContain('/api/diagnose/stream')
    expect(currentES!.url).toContain('example.com')
  })

  it('shows all 7 check items after diagnosis starts', async () => {
    render(<DiagnoseClient />)
    fireEvent.change(screen.getByLabelText('目标域名'), { target: { value: 'example.com' } })

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /开始诊断/i }))
      currentES!.emit({ type: 'check_start', key: 'dns' })
    })

    const labels = ['DNS 解析', 'HTTP 可达性', 'Ping 延迟', '路由追踪', 'SSL 证书', 'ICP 备案', 'WHOIS']
    for (const label of labels) {
      // getAllByText: same text appears in checklist and usage-tips section
      expect(screen.getAllByText(label).length).toBeGreaterThan(0)
    }
  })

  it('updates check status from running to done via SSE events', async () => {
    render(<DiagnoseClient />)
    fireEvent.change(screen.getByLabelText('目标域名'), { target: { value: 'test.com' } })

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /开始诊断/i }))
      currentES!.emit({ type: 'check_start', key: 'dns' })
    })

    // Badge "检测中" should appear
    expect(screen.getAllByText('检测中').length).toBeGreaterThan(0)

    await act(async () => {
      currentES!.emit({ type: 'check_done', key: 'dns', summary: '解析到 2 条 A 记录' })
    })

    // Summary appears and "完成" badge
    expect(screen.getByText('解析到 2 条 A 记录')).toBeTruthy()
    expect(screen.getAllByText('完成').length).toBeGreaterThan(0)
  })

  it('redirects to report page on complete event', async () => {
    render(<DiagnoseClient />)
    fireEvent.change(screen.getByLabelText('目标域名'), { target: { value: 'example.com' } })

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /开始诊断/i }))
      currentES!.emit({ type: 'complete', reportId: 'abc-123' })
    })

    expect(mockPush).toHaveBeenCalledWith('/r/abc-123')
  })

  it('strips https:// prefix from domain before sending', () => {
    render(<DiagnoseClient />)
    fireEvent.change(screen.getByLabelText('目标域名'), {
      target: { value: 'https://example.com/' },
    })
    fireEvent.click(screen.getByRole('button', { name: /开始诊断/i }))

    expect(currentES!.url).toContain('example.com')
    expect(currentES!.url).not.toContain('https%3A%2F%2F')
  })

  it('renders without crashing', () => {
    const { container } = render(<DiagnoseClient />)
    expect(container.firstChild).toBeTruthy()
  })
})
