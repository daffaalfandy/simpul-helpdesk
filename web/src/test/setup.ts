import '@testing-library/jest-dom/vitest'
import '../i18n'

import { cleanup, configure } from '@testing-library/react'
import { afterAll, afterEach, beforeAll } from 'vitest'

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

// Any API call without a handler fails the test.
beforeAll(() => server.listen({ onUnhandledFrame: 'error' }))
afterEach(() => {
  cleanup()
  server.resetHandlers()
  resetApi()
})
afterAll(() => server.close())
