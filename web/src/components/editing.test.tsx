import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { describe, expect, it, vi } from 'vitest'

import { EditableCard } from './EditableCard'
import { TextInput } from './TextInput'
import { UnsavedBar } from './UnsavedBar'

function Harness({ canEdit = true, onSave = () => {} }: { canEdit?: boolean; onSave?: (v: string) => void }) {
  const [saved, setSaved] = useState('4 hours')
  const [draft, setDraft] = useState(saved)
  const [editing, setEditing] = useState(false)
  return (
    <EditableCard
      title="Service level"
      editLabel="Edit SLA"
      saveLabel="Save SLA"
      canEdit={canEdit}
      isEditing={editing}
      isSaveDisabled={draft.trim() === ''}
      onEdit={() => setEditing(true)}
      onCancel={() => {
        setDraft(saved)
        setEditing(false)
      }}
      onSave={() => {
        setSaved(draft)
        onSave(draft)
        setEditing(false)
      }}
    >
      {editing ? <TextInput label="Resolve within" value={draft} onChange={setDraft} /> : <p>Resolve: {saved}</p>}
    </EditableCard>
  )
}

describe('EditableCard', () => {
  it('switches to a form, focuses it, and returns focus to Edit on cancel', async () => {
    const user = userEvent.setup()
    render(<Harness />)
    expect(screen.getByRole('region', { name: 'Service level' })).toHaveTextContent('Resolve: 4 hours')

    await user.click(screen.getByRole('button', { name: 'Edit SLA' }))
    const field = screen.getByLabelText('Resolve within')
    expect(field).toHaveFocus()
    await user.clear(field)
    await user.type(field, '8 hours')

    await user.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(screen.getByText('Resolve: 4 hours')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Edit SLA' })).toHaveFocus()
  })

  it('saves with the button or Enter, and blocks saving while invalid', async () => {
    const user = userEvent.setup()
    const onSave = vi.fn()
    render(<Harness onSave={onSave} />)

    await user.click(screen.getByRole('button', { name: 'Edit SLA' }))
    await user.clear(screen.getByLabelText('Resolve within'))
    expect(screen.getByRole('button', { name: 'Save SLA' })).toBeDisabled()
    await user.keyboard('{Enter}')
    expect(onSave).not.toHaveBeenCalled()

    await user.type(screen.getByLabelText('Resolve within'), '1 working day{Enter}')
    expect(onSave).toHaveBeenCalledWith('1 working day')
    expect(screen.getByText('Resolve: 1 working day')).toBeInTheDocument()
  })

  it('shows no Edit button to people who can only read', () => {
    render(<Harness canEdit={false} />)
    expect(screen.queryByRole('button', { name: 'Edit SLA' })).not.toBeInTheDocument()
  })
})

describe('UnsavedBar', () => {
  it('offers Discard and Save', async () => {
    const user = userEvent.setup()
    const onDiscard = vi.fn()
    const onSave = vi.fn()
    render(<UnsavedBar message="You have unsaved changes." onDiscard={onDiscard} onSave={onSave} />)
    const bar = screen.getByRole('region', { name: 'Unsaved changes' })
    expect(bar).toHaveTextContent('You have unsaved changes.')
    await user.click(screen.getByRole('button', { name: 'Discard' }))
    await user.click(screen.getByRole('button', { name: 'Save changes' }))
    expect(onDiscard).toHaveBeenCalledOnce()
    expect(onSave).toHaveBeenCalledOnce()
  })
})
