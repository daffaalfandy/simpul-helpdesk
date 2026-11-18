import { render, screen, within } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { Footer } from './Footer'

describe('Footer', () => {
  it('opens every link in a new tab and says so', () => {
    render(<Footer variant="internal" />)
    const links = within(screen.getByRole('navigation')).getAllByRole('link')
    expect(links.length).toBeGreaterThan(0)
    for (const link of links) {
      expect(link).toHaveAttribute('target', '_blank')
      expect(link).toHaveAttribute('rel', 'noopener noreferrer')
      expect(link).toHaveAccessibleName(/\(opens in a new tab\)$/)
    }
  })
})
