// Fails if a raw hex colour appears in web/src outside the token file.
// Colours must come from design tokens (src/styles/tokens.css).
import { readdirSync, readFileSync, statSync } from 'node:fs'
import { join, relative } from 'node:path'

const ROOT = new URL('../src/', import.meta.url).pathname
const ALLOWED = new Set(['styles/tokens.css'])
const EXTENSIONS = ['.ts', '.tsx', '.css']
const HEX = /#[0-9a-fA-F]{3,8}\b/g
const EXIT_FAILURE = 1

function* walk(dir) {
  for (const name of readdirSync(dir)) {
    const path = join(dir, name)
    if (statSync(path).isDirectory()) yield* walk(path)
    else if (EXTENSIONS.some((ext) => name.endsWith(ext))) yield path
  }
}

const problems = []
for (const file of walk(ROOT)) {
  const rel = relative(ROOT, file)
  if (ALLOWED.has(rel) || rel.endsWith('.gen.ts')) continue
  readFileSync(file, 'utf8')
    .split('\n')
    .forEach((line, i) => {
      for (const match of line.matchAll(HEX)) problems.push(`src/${rel}:${i + 1}: raw colour ${match[0]}`)
    })
}

if (problems.length > 0) {
  console.error(problems.join('\n'))
  console.error('\nUse a design token (src/styles/tokens.css) instead of a raw colour.')
  process.exit(EXIT_FAILURE)
}
