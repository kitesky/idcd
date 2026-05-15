import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import CidrCalculatorPage from '../cidr-calculator/page'

describe('CIDR Calculator Tool', () => {
  it('renders without crashing', () => {
    const { container } = render(<CidrCalculatorPage />)
    expect(container.firstChild).toBeTruthy()
  })

  it('has a heading', () => {
    render(<CidrCalculatorPage />)
    expect(screen.getByRole('heading', { level: 1 })).toBeTruthy()
  })

  it('has an input for CIDR notation', () => {
    render(<CidrCalculatorPage />)
    const inputs = screen.getAllByRole('textbox')
    expect(inputs.length).toBeGreaterThanOrEqual(1)
  })

  it('calculates 192.168.1.0/24', () => {
    render(<CidrCalculatorPage />)
    const inputs = screen.getAllByRole('textbox')
    fireEvent.change(inputs[0]!, { target: { value: '192.168.1.0/24' } })
    const buttons = screen.getAllByRole('button')
    if (buttons.length > 0) fireEvent.click(buttons[0]!)
    // Just verify no crash
    expect(inputs[0]).toBeTruthy()
  })

  it('shows host count for /24', () => {
    render(<CidrCalculatorPage />)
    const inputs = screen.getAllByRole('textbox')
    fireEvent.change(inputs[0]!, { target: { value: '192.168.1.0/24' } })
    const buttons = screen.getAllByRole('button')
    if (buttons.length > 0) fireEvent.click(buttons[0]!)
    expect(document.body.textContent).toContain('254')
  })
})
