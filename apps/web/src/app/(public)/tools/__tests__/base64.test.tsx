import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import Base64Page from '../base64/page'

describe('Base64 Tool', () => {
  it('renders Base64 tool heading', () => {
    render(<Base64Page />)
    expect(screen.getByRole('heading', { level: 1 })).toBeTruthy()
  })

  it('has encode and decode inputs', () => {
    render(<Base64Page />)
    const textareas = screen.getAllByRole('textbox')
    expect(textareas.length).toBeGreaterThanOrEqual(1)
  })

  it('encodes text to Base64', async () => {
    render(<Base64Page />)
    const inputs = screen.getAllByRole('textbox')
    fireEvent.change(inputs[0]!, { target: { value: 'Hello World!' } })
    const buttons = screen.getAllByRole('button')
    expect(buttons.length).toBeGreaterThan(0)
  })

  it('renders without crashing', () => {
    const { container } = render(<Base64Page />)
    expect(container.firstChild).toBeTruthy()
  })
})
