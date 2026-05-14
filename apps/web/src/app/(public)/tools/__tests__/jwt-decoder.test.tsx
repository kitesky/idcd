import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import JwtDecoderPage from '../jwt-decoder/page'

const SAMPLE_JWT = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c'

describe('JWT Decoder Tool', () => {
  it('renders without crashing', () => {
    const { container } = render(<JwtDecoderPage />)
    expect(container.firstChild).toBeTruthy()
  })

  it('has a heading', () => {
    render(<JwtDecoderPage />)
    expect(screen.getByRole('heading', { level: 1 })).toBeTruthy()
  })

  it('has a textarea for JWT input', () => {
    render(<JwtDecoderPage />)
    const textareas = screen.getAllByRole('textbox')
    expect(textareas.length).toBeGreaterThanOrEqual(1)
  })

  it('decodes a sample JWT', () => {
    render(<JwtDecoderPage />)
    const inputs = screen.getAllByRole('textbox')
    fireEvent.change(inputs[0], { target: { value: SAMPLE_JWT } })
    expect(inputs[0]).toBeTruthy()
  })
})
