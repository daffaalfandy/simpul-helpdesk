import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { describe, expect, it, vi } from 'vitest'

import { MAX_NESTING_DEPTH } from '../lib/richTextLimits'
import { writeRichText } from '../test/richText'
import { RichText } from './RichText'
import { RichTextField } from './RichTextField'

describe('RichText', () => {
  it('formats Markdown', () => {
    render(<RichText markdown={'**Login** fails\n\n- on Chrome\n- on Edge\n\nSee [the guide](https://example.com/help)'} />)
    expect(screen.getByText('Login').tagName).toBe('STRONG')
    expect(screen.getAllByRole('listitem')).toHaveLength(2)
    const link = screen.getByRole('link', { name: /the guide/ })
    expect(link).toHaveAttribute('href', 'https://example.com/help')
    expect(link).toHaveAttribute('target', '_blank')
  })

  it('shows raw HTML as text instead of running or dropping it', () => {
    const { container } = render(<RichText markdown={'<script>alert(1)</script>\n\nError: <html> not found'} />)
    expect(container.querySelector('script')).toBeNull()
    expect(screen.getByText('<script>alert(1)</script>')).toBeInTheDocument()
    expect(screen.getByText('Error: <html> not found')).toBeInTheDocument()
  })

  it('drops unsafe links and never loads images', () => {
    const { container } = render(<RichText markdown={'[click me](javascript:alert(1)) ![server rack](https://example.com/x.png)'} />)
    expect(screen.queryByRole('link')).not.toBeInTheDocument()
    expect(screen.getByText('click me')).toBeInTheDocument()
    expect(container.querySelector('img')).toBeNull()
    expect(screen.getByText('server rack')).toBeInTheDocument()
  })

  it('keeps single line breaks, as plain-text bodies always had', () => {
    const { container } = render(<RichText markdown={'Step one\nStep two'} />)
    expect(container.querySelector('br')).not.toBeNull()
    expect(container.querySelector('p')).toHaveTextContent(/^Step one\s*Step two$/)
  })

  it('puts tables in their own scrolling box', () => {
    const { container } = render(<RichText markdown={'| Office | Status |\n| --- | --- |\n| Jambi | Down |'} />)
    expect(container.querySelector('table')?.parentElement).toHaveClass('overflow-x-auto')
  })

  it('shows a body past the structure limits as plain text instead of formatting it', () => {
    const deep = '>'.repeat(MAX_NESTING_DEPTH + 1) + ' **deep**'
    const { container } = render(<RichText markdown={deep} />)
    expect(container.querySelector('blockquote')).toBeNull()
    expect(container.querySelector('strong')).toBeNull()
    expect(screen.getByText(deep)).toHaveClass('whitespace-pre-wrap')
  })
})

/** The field inside a form, like every composer: the link box must never submit it. */
function Field({ maxLength, initial = '', onSubmit }: { maxLength?: number; initial?: string; onSubmit?: () => void }) {
  const [value, setValue] = useState(initial)
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault()
        onSubmit?.()
      }}
    >
      <RichTextField label="Message" value={value} onChange={setValue} maxLength={maxLength} />
      <output data-testid="markdown">{value}</output>
    </form>
  )
}

const markdown = () => screen.getByTestId('markdown').textContent

describe('RichTextField', () => {
  it('is a labelled text box that reports Markdown', async () => {
    const user = userEvent.setup()
    render(<Field />)
    await writeRichText(user, screen.getByRole('textbox', { name: 'Message' }), 'Login fails since 08:00')
    expect(markdown()).toBe('Login fails since 08:00')
  })

  it('bolds the selection from the toolbar', async () => {
    const user = userEvent.setup()
    render(<Field />)
    const box = screen.getByRole('textbox', { name: 'Message' })
    await writeRichText(user, box, 'Urgent')
    await user.keyboard('{Control>}a{/Control}')
    const bold = within(screen.getByRole('toolbar', { name: 'Formatting' })).getByRole('button', { name: 'Bold' })
    await user.click(bold)
    expect(markdown()).toBe('**Urgent**')
    expect(bold).toHaveAttribute('aria-pressed', 'true')
  })

  it('adds a link only for web and email addresses, and Escape closes the link box', async () => {
    const user = userEvent.setup()
    const submitted = vi.fn()
    render(<Field onSubmit={submitted} />)
    await user.click(screen.getByRole('button', { name: 'Link' }))
    const address = await screen.findByRole('textbox', { name: 'Link address' })
    await user.type(address, 'javascript:alert(1)')
    await user.click(screen.getByRole('button', { name: 'Save link' }))
    expect(screen.getByText(/Use a web or email address/)).toBeInTheDocument()
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())

    await user.click(screen.getByRole('button', { name: 'Link' }))
    await user.type(await screen.findByRole('textbox', { name: 'Link address' }), 'example.com/help')
    await user.click(screen.getByRole('button', { name: 'Save link' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(markdown()).toBe('[https://example.com/help](https://example.com/help)')
    // Saving a link (a form in a portal) must not send the message it sits in.
    expect(submitted).not.toHaveBeenCalled()
  })

  it('says when the text is longer than the API takes', () => {
    render(<Field maxLength={5} initial="Too long" />)
    expect(screen.getByText('Keep it under 5 characters')).toBeInTheDocument()
    expect(screen.getByRole('textbox', { name: 'Message' })).toHaveAttribute('aria-invalid', 'true')
  })

  it('shows Markdown it is given and replaces it when the parent resets it', async () => {
    const { rerender } = render(<RichTextField label="Message" value="**Hi**" onChange={() => {}} />)
    const box = screen.getByRole('textbox', { name: 'Message' })
    expect(within(box).getByText('Hi').tagName).toBe('STRONG')
    rerender(<RichTextField label="Message" value="" onChange={() => {}} />)
    await waitFor(() => expect(box).toHaveTextContent(''))
  })
})
