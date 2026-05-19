import { renderHook, act } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

// Mock the api module before importing the hook
vi.mock('@/lib/api', () => ({
  getProbeTask: vi.fn(),
}))

import { useProbePolling } from '../useProbePolling'
import { getProbeTask } from '@/lib/api'

const mockGetProbeTask = vi.mocked(getProbeTask)

describe('useProbePolling', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    mockGetProbeTask.mockReset()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('taskId=null 时不触发 poll，isPolling 为 false', () => {
    const { result } = renderHook(() => useProbePolling(null))
    expect(result.current.isPolling).toBe(false)
    expect(result.current.taskResult).toBeNull()
    expect(result.current.error).toBe('')
    expect(mockGetProbeTask).not.toHaveBeenCalled()
  })

  it('初始状态：isPolling=false，taskResult=null，error 为空', () => {
    const { result } = renderHook(() => useProbePolling(null))
    expect(result.current.isPolling).toBe(false)
    expect(result.current.taskResult).toBeNull()
    expect(result.current.error).toBe('')
  })

  it('传入 taskId 后 isPolling 变为 true 并调用 getProbeTask', async () => {
    // 返回一个 pending promise，不会 resolve，以观察 isPolling 状态
    mockGetProbeTask.mockImplementation(() => new Promise(() => { /* intentionally never resolves */ }))

    const { result } = renderHook(() => useProbePolling('pt_123'))

    // isPolling 立即为 true
    expect(result.current.isPolling).toBe(true)
    expect(mockGetProbeTask).toHaveBeenCalledWith('pt_123')
  })

  it('status=completed 时停止轮询，isPolling 变为 false', async () => {
    mockGetProbeTask.mockResolvedValue({
      task_id: 'pt_456',
      status: 'completed',
      result: { success: true, duration_ms: 42 },
      created_at: '2026-05-15T00:00:00Z',
      completed_at: '2026-05-15T00:00:01Z',
    })

    const { result } = renderHook(() => useProbePolling('pt_456'))

    // 等待 promise resolve
    await act(async () => {
      await Promise.resolve()
    })

    expect(result.current.isPolling).toBe(false)
    expect(result.current.taskResult?.status).toBe('completed')
    expect(result.current.error).toBe('')
    // 只调用了一次，因为第一次就返回 completed
    expect(mockGetProbeTask).toHaveBeenCalledTimes(1)
  })

  it('status=failed 时停止轮询', async () => {
    mockGetProbeTask.mockResolvedValue({
      task_id: 'pt_789',
      status: 'failed',
      created_at: '2026-05-15T00:00:00Z',
    })

    const { result } = renderHook(() => useProbePolling('pt_789'))

    await act(async () => {
      await Promise.resolve()
    })

    expect(result.current.isPolling).toBe(false)
    expect(result.current.taskResult?.status).toBe('failed')
  })

  it('status=cancelled 时停止轮询', async () => {
    mockGetProbeTask.mockResolvedValue({
      task_id: 'pt_can',
      status: 'cancelled',
      created_at: '2026-05-15T00:00:00Z',
    })

    const { result } = renderHook(() => useProbePolling('pt_can'))

    await act(async () => {
      await Promise.resolve()
    })

    expect(result.current.isPolling).toBe(false)
    expect(result.current.taskResult?.status).toBe('cancelled')
  })

  it('API 抛错时停止轮询并设置 error', async () => {
    mockGetProbeTask.mockRejectedValue(new Error('网络错误'))

    const { result } = renderHook(() => useProbePolling('pt_err'))

    await act(async () => {
      await Promise.resolve()
    })

    expect(result.current.error).toBe('网络错误')
    expect(result.current.isPolling).toBe(false)
  })

  it('running 状态下继续轮询，2s 后再次请求', async () => {
    // 第一次返回 running，第二次返回 completed
    mockGetProbeTask
      .mockResolvedValueOnce({
        task_id: 'pt_poll',
        status: 'running',
        created_at: '2026-05-15T00:00:00Z',
      })
      .mockResolvedValueOnce({
        task_id: 'pt_poll',
        status: 'completed',
        result: { success: true, duration_ms: 100 },
        created_at: '2026-05-15T00:00:00Z',
        completed_at: '2026-05-15T00:00:02Z',
      })

    const { result } = renderHook(() => useProbePolling('pt_poll'))

    // 等待第一次 poll 完成（running 状态）
    await act(async () => {
      await Promise.resolve()
    })

    expect(mockGetProbeTask).toHaveBeenCalledTimes(1)
    // 还在 polling（running 状态尚未停止）
    expect(result.current.isPolling).toBe(true)

    // 推进 2s 定时器触发第二次 poll
    await act(async () => {
      vi.advanceTimersByTime(2000)
      await Promise.resolve()
    })

    expect(mockGetProbeTask).toHaveBeenCalledTimes(2)
    expect(result.current.isPolling).toBe(false)
    expect(result.current.taskResult?.status).toBe('completed')
  })

  it('超过 120s 超时后停止轮询并设置超时错误', async () => {
    // 一直返回 running
    mockGetProbeTask.mockResolvedValue({
      task_id: 'pt_timeout',
      status: 'running',
      created_at: '2026-05-15T00:00:00Z',
    })

    // 固定起始时间
    const startTime = 1_000_000
    vi.setSystemTime(startTime)

    const { result } = renderHook(() => useProbePolling('pt_timeout'))

    // 等待第一次 poll
    await act(async () => {
      await Promise.resolve()
    })

    // 推进系统时间超过 120s，使得超时检查生效
    vi.setSystemTime(startTime + 121_000)

    // 推进 2s 定时器，让下一次 poll 尝试触发（超时检查会拦截）
    await act(async () => {
      vi.advanceTimersByTime(2_000)
      await Promise.resolve()
    })

    expect(result.current.error).toBe('拨测超时（120s），请重试')
    expect(result.current.isPolling).toBe(false)
  })
})
