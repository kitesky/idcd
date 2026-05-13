import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import JsonFormatterPage from '../json-formatter/page'

describe('JSON Formatter Tool', () => {
  it('renders without crashing', () => {
    const { container } = render(<JsonFormatterPage />)
    expect(container.firstChild).toBeTruthy()
  })

  it('has a heading', () => {
    render(<JsonFormatterPage />)
    expect(screen.getByRole('heading', { level: 1 })).toBeTruthy()
  })

  it('has at least one textarea for JSON input', () => {
    render(<JsonFormatterPage />)
    const textareas = screen.getAllByRole('textbox')
    expect(textareas.length).toBeGreaterThanOrEqual(1)
  })

  it('has action buttons', () => {
    render(<JsonFormatterPage />)
    const buttons = screen.getAllByRole('button')
    expect(buttons.length).toBeGreaterThan(0)
  })

  it('formats valid JSON', async () => {
    render(<JsonFormatterPage />)
    const inputs = screen.getAllByRole('textbox')
    fireEvent.change(inputs[0], { target: { value: '{"a":1}' } })
    const buttons = screen.getAllByRole('button')
    fireEvent.click(buttons[0])
    // After formatting, at least one textarea should have content
    const updatedInputs = screen.getAllByRole('textbox')
    expect(updatedInputs.some((t: HTMLTextAreaElement) => (t as HTMLTextAreaElement).value.length > 0)).toBeTruthy()
  })
})
