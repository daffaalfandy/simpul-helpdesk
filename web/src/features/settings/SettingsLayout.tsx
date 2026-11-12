import { Outlet } from '@tanstack/react-router'

import { SettingsNav } from './SettingsNav'

/** Settings: sidebar plus the chosen section. Only the section changes between pages. */
export function SettingsLayout() {
  return (
    <div className="flex flex-col gap-7 lg:flex-row lg:items-start">
      <SettingsNav />
      <div className="flex min-w-0 flex-1 flex-col gap-5">
        <Outlet />
      </div>
    </div>
  )
}
