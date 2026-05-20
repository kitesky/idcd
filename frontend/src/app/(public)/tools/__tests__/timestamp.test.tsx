import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import TimestampPage from '../timestamp/page'

describe('Timestamp Tool', () => {
  it('renders without crashing', () => {
    const { container } = render(<TimestampPage />)
    expect(container.firstChild).toBeTruthy()
  })

  it('has a heading', () => {
    render(<TimestampPage />)
    expect(screen.getByRole('heading', { level: 1 })).toBeTruthy()
  })

  it('has at least one input', () => {
    render(<TimestampPage />)
    const inputs = screen.getAllByRole('textbox')
    expect(inputs.length).toBeGreaterThanOrEqual(1)
  })

  it('has action buttons', () => {
    render(<TimestampPage />)
    const buttons = screen.getAllByRole('button')
    expect(buttons.length).toBeGreaterThan(0)
  })

  it('accepts a known unix timestamp', () => {
    render(<TimestampPage />)
    const inputs = screen.getAllByRole('textbox')
    fireEvent.change(inputs[0]!, { target: { value: '1000000000' } })
    // Just verify it doesn't throw
    expect(inputs[0]).toBeTruthy()
  })
})
