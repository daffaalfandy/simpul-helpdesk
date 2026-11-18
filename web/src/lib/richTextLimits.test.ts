import { describe, expect, it } from 'vitest'

import { isTooComplex, MAX_NESTING_DEPTH, MAX_TABLE_CELLS } from './richTextLimits'
import { safeRedirect } from './text'

const wideHeader = '|a'.repeat(500) + '|\n' + '|-'.repeat(500) + '|\n'

// The same cases as the API's richtext limits_test.go: both sides must agree.
describe('isTooComplex', () => {
  it.each([
    ['plain text', 'Login fails since this morning.', false],
    ['small table', '| a | b |\n|---|---|\n| 1 | 2 |', false],
    ['table rows up to the limit', '|a|b|\n|-|-|\n' + '|1|2|\n'.repeat(MAX_TABLE_CELLS / 2 - 1), false],
    ['table rows past the limit', '|a|b|\n|-|-|\n' + '|1|2|\n'.repeat(MAX_TABLE_CELLS / 2), true],
    ['short rows still fill to the header', wideHeader + '|\n'.repeat(MAX_TABLE_CELLS / 500), true],
    ['a blank line ends the table', wideHeader + '\n' + 'text\n'.repeat(MAX_TABLE_CELLS), false],
    ['tables add up', ('|a|b|\n|-|-|\n' + '|1|2|\n'.repeat(999) + '\n').repeat(3), true],
    ['aligned delimiter', '| a |\n| :-: |\n| 1 |', false],
    ['thematic break is not a table', '---\n'.repeat(MAX_TABLE_CELLS + 1), false],
    ['quotes up to the limit', '>'.repeat(MAX_NESTING_DEPTH) + ' deep', false],
    ['quotes past the limit', '>'.repeat(MAX_NESTING_DEPTH + 1) + ' deep', true],
    ['spaced quotes', '> '.repeat(MAX_NESTING_DEPTH + 1) + 'deep', true],
    ['bullets past the limit', '- '.repeat(MAX_NESTING_DEPTH + 1) + 'deep', true],
    ['numbers past the limit', '1. '.repeat(MAX_NESTING_DEPTH + 1) + 'deep', true],
    ['mixed markers', '> * 2) '.repeat(Math.floor(MAX_NESTING_DEPTH / 3) + 1) + 'deep', true],
    ['dashes without spaces', '-'.repeat(1000), false],
    ['a long number is not a marker', '1234567890. text', false],
    ['emphasis is not nesting', '*'.repeat(1000), false],
  ])('%s', (_name, markdown, want) => {
    expect(isTooComplex(markdown)).toBe(want)
  })
})

describe('safeRedirect', () => {
  it.each([
    ['/board', '/board'],
    ['/tickets/HD-1?tab=activity', '/tickets/HD-1?tab=activity'],
    ['//evil.example', undefined],
    ['/\\evil.example', undefined],
    ['https://evil.example', undefined],
    [42, undefined],
  ])('%s', (target, want) => {
    expect(safeRedirect(target)).toBe(want)
  })
})
