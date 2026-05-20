import { render, screen } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'

// Stub probe API to avoid real HTTP calls.
vi.mock('@/lib/api', () => ({
  probeSpeedtest: vi.fn().mockResolvedValue({ task_id: 'test-task-123', status: 'queued' }),
  getProbeTask: vi.fn().mockResolvedValue({ task_id: 'test-task-123', status: 'queued' }),
  getNodes: vi.fn().mockResolvedValue({ data: [] }),
}))

// Stub polling hook so the component does not hit the network.
vi.mock('@/hooks/useProbePolling', () => ({
  useProbePolling: vi.fn().mockReturnValue({ result: null, loading: false, error: null }),
}))

import SpeedtestProbeClient from '../speedtest-probe-client'

describe('SpeedtestProbeClient — smoke test', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders the page heading', () => {
    render(<SpeedtestProbeClient />)
    expect(screen.getByText('网速测试')).toBeTruthy()
  })

  it('renders the target URL input', () => {
    render(<SpeedtestProbeClient />)
    const input = screen.getByLabelText('目标 URL')
    expect(input).toBeTruthy()
  })

  it('renders the submit button disabled when input is empty', () => {
    render(<SpeedtestProbeClient />)
    const btn = screen.getByRole('button', { name: '开始测速' })
    expect((btn as HTMLButtonElement).disabled).toBe(true)
  })

  it('renders usage instructions card', () => {
    render(<SpeedtestProbeClient />)
    expect(screen.getByText('使用说明')).toBeTruthy()
  })
})
