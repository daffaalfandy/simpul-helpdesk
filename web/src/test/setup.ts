import '@testing-library/jest-dom/vitest'
import '../i18n'

import { cleanup, configure } from '@testing-library/react'
import { afterAll, afterEach, beforeAll, beforeEach, vi } from 'vitest'

import { resetApi, server } from './server'

// A file's first render loads its routes cold; with every file running in parallel that can
// take longer than Testing Library's 1 s default for findBy and waitFor.
const ASYNC_UTIL_TIMEOUT_MS = 3000
configure({ asyncUtilTimeout: ASYNC_UTIL_TIMEOUT_MS })

// The rich text editor (ProseMirror) measures the caret with layout APIs jsdom leaves out. With
// nothing to measure they answer "no box", which is enough for typing and selecting in tests.
const noRects = (): DOMRectList => Object.assign([], { item: () => null }) as unknown as DOMRectList
Range.prototype.getClientRects = noRects
Range.prototype.getBoundingClientRect = () => new DOMRect()
document.elementFromPoint = () => null

// Fixtures are dated November 2026 and the date filters default to the current month, so every
// test runs on 18 Nov 2026 (10:00 in Jakarta). Only Date is faked: timers stay real, and the
// clock keeps moving from there.
const TEST_NOW = new Date('2026-11-18T03:00:00Z')
vi.useFakeTimers({ toFake: ['Date'], shouldAdvanceTime: true })
beforeEach(() => {
  vi.setSystemTime(TEST_NOW)
})

// Any API call without a handler fails the test.
beforeAll(() => server.listen({ onUnhandledFrame: 'error' }))
afterEach(() => {
  cleanup()
  server.resetHandlers()
  resetApi()
})
afterAll(() => server.close())
