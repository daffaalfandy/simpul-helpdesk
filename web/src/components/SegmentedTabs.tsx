import type { ReactNode } from 'react'
import { Tab, TabList, TabPanel, Tabs } from 'react-aria-components'

import { cx } from '../lib/text'

export type SegmentedTab<K extends string> = { id: K; label: string }

type SegmentedTabsProps<K extends string> = {
  label: string
  tabs: ReadonlyArray<SegmentedTab<K>>
  selectedKey: K
  onSelectionChange: (key: K) => void
  /** Panel content for a tab. */
  children: (key: K) => ReactNode
}

/** Two equal tabs over a panel (HANDOFF §14.3): selected tab is bold with a primary underline. */
export function SegmentedTabs<K extends string>({ label, tabs, selectedKey, onSelectionChange, children }: SegmentedTabsProps<K>) {
  return (
    <Tabs selectedKey={selectedKey} onSelectionChange={(key) => onSelectionChange(String(key) as K)}>
      <TabList aria-label={label} className="flex border-b border-border">
        {tabs.map((tab) => (
          <Tab
            key={tab.id}
            id={tab.id}
            className={({ isSelected, isFocusVisible }) =>
              cx(
                'relative flex min-h-(--control-h-md) flex-1 cursor-pointer items-center justify-center px-4 text-base outline-none',
                isSelected
                  ? 'font-extrabold text-ink after:absolute after:inset-x-0 after:bottom-0 after:h-(--tab-bar-h) after:bg-primary'
                  : 'font-semibold text-muted hover:text-ink',
                isFocusVisible && 'shadow-(--focus-ring)',
              )
            }
          >
            {tab.label}
          </Tab>
        ))}
      </TabList>
      {tabs.map((tab) => (
        <TabPanel key={tab.id} id={tab.id} className="pt-3 outline-none">
          {children(tab.id)}
        </TabPanel>
      ))}
    </Tabs>
  )
}
