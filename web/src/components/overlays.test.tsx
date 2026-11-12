import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { describe, expect, it, vi } from 'vitest'

import { ConfirmDialog } from './Dialog'
import { Dropdown } from './Dropdown'

const OPTIONS = [
  { id: 'standard', label: 'Standard', description: 'P1 24/7' },
  { id: 'gold', label: 'Gold', description: 'P1, P2 24/7' },
]

function Controlled({ onChange }: { onChange: (v: string) => void }) {
  const [value, setValue] = useState('standard')
  return (
    <Dropdown
      label="SLA plan"
      options={OPTIONS}
      value={value}
      onChange={(v) => {
        setValue(v)
        onChange(v)
      }}
    />
  )
}

describe('Dropdown', () => {
  it('is a labelled listbox trigger that works with the keyboard', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(<Controlled onChange={onChange} />)
    const trigger = screen.getByRole('button', { name: /SLA plan/ })
    expect(trigger).toHaveAttribute('aria-haspopup', 'listbox')
    expect(trigger).toHaveTextContent('Standard')

    trigger.focus()
    await user.keyboard('{ArrowDown}')
    const listbox = await screen.findByRole('listbox')
    expect(listbox).toBeInTheDocument()
    expect(screen.getByRole('option', { name: /Standard/ })).toHaveAttribute('aria-selected', 'true')
    await user.keyboard('{ArrowDown}{Enter}')
    expect(onChange).toHaveBeenCalledWith('gold')
    await waitFor(() => expect(screen.queryByRole('listbox')).not.toBeInTheDocument())
    expect(trigger).toHaveTextContent('Gold')

    await user.click(trigger)
    await screen.findByRole('listbox')
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('listbox')).not.toBeInTheDocument())
    expect(onChange).toHaveBeenCalledTimes(1)
  })
})

describe('ConfirmDialog', () => {
  function Harness({ onConfirm, onOpenChange }: { onConfirm: (r: string) => void; onOpenChange: (o: boolean) => void }) {
    return (
      <ConfirmDialog
        isOpen
        onOpenChange={onOpenChange}
        tone="danger"
        title="Send back?"
        confirmLabel="Send back"
        reasonLabel="Reason (required, saved to the audit log)"
        onConfirm={onConfirm}
      >
        The ticket returns to In progress.
      </ConfirmDialog>
    )
  }

  it('keeps confirm disabled until a reason is written', async () => {
    const user = userEvent.setup()
    const onConfirm = vi.fn()
    render(<Harness onConfirm={onConfirm} onOpenChange={vi.fn()} />)
    const dialog = screen.getByRole('alertdialog', { name: 'Send back?' })
    const confirm = screen.getByRole('button', { name: 'Send back' })
    expect(dialog).toBeInTheDocument()
    expect(confirm).toBeDisabled()
    await user.type(screen.getByRole('textbox', { name: /Reason/ }), '  Still broken  ')
    expect(confirm).toBeEnabled()
    await user.click(confirm)
    expect(onConfirm).toHaveBeenCalledWith('Still broken')
  })

  it('closes on Escape', async () => {
    const user = userEvent.setup()
    const onOpenChange = vi.fn()
    render(<Harness onConfirm={vi.fn()} onOpenChange={onOpenChange} />)
    await user.keyboard('{Escape}')
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })
})
