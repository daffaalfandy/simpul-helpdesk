import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { describe, expect, it, vi } from 'vitest'

import { Avatar } from './Avatar'
import { FilterPills } from './FilterPills'
import { OptionCard, OptionCardGroup } from './OptionCard'
import { ProgressTracker } from './ProgressTracker'
import { SearchSelect } from './SearchSelect'
import { SegmentedTabs } from './SegmentedTabs'

describe('OptionCardGroup', () => {
  function Cards() {
    const [value, setValue] = useState<'a' | 'b' | null>(null)
    return (
      <OptionCardGroup label="Type" value={value} onChange={setValue} isInvalid={value === null} errorMessage="Pick one">
        <OptionCard value="a">Alpha</OptionCard>
        <OptionCard value="b">Beta</OptionCard>
      </OptionCardGroup>
    )
  }

  it('is a radio group that arrow keys move through', async () => {
    const user = userEvent.setup()
    render(<Cards />)
    expect(screen.getByText('Pick one')).toBeInTheDocument()
    await user.click(screen.getByRole('radio', { name: 'Alpha' }))
    expect(screen.getByRole('radio', { name: 'Alpha' })).toBeChecked()
    await user.keyboard('{ArrowRight}')
    expect(screen.getByRole('radio', { name: 'Beta' })).toBeChecked()
  })
})

describe('FilterPills', () => {
  it('marks exactly one pill as pressed', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(
      <FilterPills
        label="Filter"
        options={[
          { id: 'open', label: 'Open' },
          { id: 'all', label: 'All' },
        ]}
        value="open"
        onChange={onChange}
      />,
    )
    const group = screen.getByRole('group', { name: 'Filter' })
    expect(within(group).getByRole('button', { name: 'Open' })).toHaveAttribute('aria-pressed', 'true')
    expect(within(group).getByRole('button', { name: 'All' })).toHaveAttribute('aria-pressed', 'false')
    await user.click(within(group).getByRole('button', { name: 'Open' }))
    expect(onChange).toHaveBeenLastCalledWith('open')
    await user.click(within(group).getByRole('button', { name: 'All' }))
    expect(onChange).toHaveBeenLastCalledWith('all')
  })
})

describe('SegmentedTabs', () => {
  it('renders tabs with the selected panel', async () => {
    const user = userEvent.setup()
    function Tabs() {
      const [key, setKey] = useState<'reply' | 'note'>('reply')
      return (
        <SegmentedTabs
          label="Message type"
          tabs={[
            { id: 'reply', label: 'Reply' },
            { id: 'note', label: 'Note' },
          ]}
          selectedKey={key}
          onSelectionChange={setKey}
        >
          {(id) => <p>Panel {id}</p>}
        </SegmentedTabs>
      )
    }
    render(<Tabs />)
    expect(screen.getByRole('tab', { name: 'Reply' })).toHaveAttribute('aria-selected', 'true')
    await user.click(screen.getByRole('tab', { name: 'Note' }))
    expect(screen.getByRole('tabpanel')).toHaveTextContent('Panel note')
  })
})

describe('ProgressTracker', () => {
  it('marks only the current step', () => {
    render(
      <ProgressTracker
        label="Progress"
        doneLabel="Completed"
        steps={[
          { id: 'a', label: 'Received', state: 'done' },
          { id: 'b', label: 'Working', state: 'current' },
          { id: 'c', label: 'Resolved', state: 'upcoming' },
        ]}
      />,
    )
    const items = within(screen.getByRole('list', { name: 'Progress' })).getAllByRole('listitem')
    expect(items.map((li) => li.getAttribute('aria-current'))).toEqual([null, 'step', null])
    expect(items[0]).toHaveTextContent('Completed')
  })
})

describe('Avatar', () => {
  it('shows "?" for nobody and a name when labelled', () => {
    render(
      <>
        <Avatar label="Assignee: Unassigned" />
        <Avatar name="Rina Pratiwi" label="Assignee: Rina Pratiwi" />
      </>,
    )
    expect(screen.getByRole('img', { name: 'Assignee: Unassigned' })).toHaveTextContent('?')
    expect(screen.getByRole('img', { name: 'Assignee: Rina Pratiwi' })).toHaveTextContent('RP')
  })
})

describe('SearchSelect', () => {
  it('filters by any text and offers an action for new entries', async () => {
    const user = userEvent.setup()
    const onAction = vi.fn()
    const onChange = vi.fn()
    render(
      <SearchSelect
        label="Requester"
        emptyText="No match"
        options={[
          { id: 'h', label: 'Hendra Gunawan', searchText: 'Hendra Gunawan hendra@kominfo.test' },
          { id: 'd', label: 'Dian Puspita', searchText: 'Dian Puspita dian@kominfo.test' },
        ]}
        value={null}
        onChange={onChange}
        action={{ label: (q) => `Add ${q}`, onAction }}
      />,
    )
    await user.type(screen.getByRole('combobox', { name: 'Requester' }), 'dian@')
    expect(await screen.findByRole('option', { name: 'Dian Puspita' })).toBeInTheDocument()
    expect(screen.queryByRole('option', { name: 'Hendra Gunawan' })).not.toBeInTheDocument()
    await user.click(screen.getByRole('option', { name: 'Add dian@' }))
    expect(onAction).toHaveBeenCalledWith('dian@')
    expect(onChange).not.toHaveBeenCalled()
  })
})
