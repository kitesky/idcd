import { describe, it, expect } from 'vitest'
import { cn } from './utils'

describe('cn utility function', () => {
  it('should merge class names correctly', () => {
    const result = cn('px-2 py-1', 'text-sm')
    expect(result).toBe('px-2 py-1 text-sm')
  })

  it('should handle conditional classes', () => {
    const result = cn('base-class', true && 'conditional-class', false && 'hidden-class')
    expect(result).toBe('base-class conditional-class')
  })

  it('should override conflicting Tailwind classes', () => {
    const result = cn('px-2', 'px-4')
    expect(result).toBe('px-4')
  })

  it('should handle arrays and objects', () => {
    const result = cn(['px-2', 'py-1'], { 'bg-blue-500': true, 'text-white': false })
    expect(result).toBe('px-2 py-1 bg-blue-500')
  })

  it('should handle undefined and null values', () => {
    const result = cn('px-2', undefined, null, 'py-1')
    expect(result).toBe('px-2 py-1')
  })

  it('should handle empty input', () => {
    const result = cn()
    expect(result).toBe('')
  })
})