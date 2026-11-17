import Markdown, { type Components } from 'react-markdown'
import remarkBreaks from 'remark-breaks'
import remarkGfm from 'remark-gfm'

import { cx } from '../lib/text'
import { isSafeHref } from '../lib/links'
import { ExternalLink } from './ExternalLink'

/** The parts of an mdast node this file reads. */
type MdNode = { type: string; value?: string; children?: MdNode[] }

/** Parents whose children are blocks: raw HTML there becomes its own paragraph. */
const BLOCK_PARENTS = new Set(['root', 'blockquote', 'listItem', 'footnoteDefinition'])

/**
 * Raw HTML in a body is shown as the text it is, never rendered and never dropped: an old
 * plain-text body like "Error: <html> not found" keeps every word. (The API's email renderer
 * does the same.)
 */
function remarkHtmlAsText() {
  const walk = (node: MdNode) => {
    node.children = node.children?.map((child) => {
      if (child.type !== 'html') {
        walk(child)
        return child
      }
      const text: MdNode = { type: 'text', value: child.value ?? '' }
      return BLOCK_PARENTS.has(node.type) ? { type: 'paragraph', children: [text] } : text
    })
  }
  return walk
}

/**
 * Breaks run after the HTML pass, so a multi-line HTML block keeps its line breaks as text.
 * remark-breaks makes a single newline a line break, as in the plain-text bodies written before
 * Markdown.
 */
const remarkPlugins = [remarkGfm, remarkHtmlAsText, remarkBreaks]

export type RichTextTone = 'default' | 'onDark'

const linkTones: Record<RichTextTone, string> = {
  default: '',
  onDark: 'text-on-dark-accent underline underline-offset-2',
}

function components(tone: RichTextTone): Components {
  const heading: Components['p'] = ({ children }) => <p className="font-bold">{children}</p>
  return {
    a: ({ href, children }) => {
      if (!href || !isSafeHref(href)) return <span>{children}</span>
      if (href.startsWith('mailto:')) {
        return (
          <a href={href} className={linkTones[tone]}>
            {children}
          </a>
        )
      }
      return (
        <ExternalLink href={href} className={linkTones[tone]}>
          {children}
        </ExternalLink>
      )
    },
    // Images are never loaded (no tracking pixels; files travel as attachments): the alt text stays.
    img: ({ alt }) => (alt ? <span>{alt}</span> : null),
    // Headings come from pasted or AI-written text; inside a message they read as bold lines.
    h1: heading,
    h2: heading,
    h3: heading,
    h4: heading,
    h5: heading,
    h6: heading,
    // A wide table scrolls in its own box instead of pushing the page sideways on a phone.
    table: ({ children }) => (
      <div className="overflow-x-auto">
        <table>{children}</table>
      </div>
    ),
  }
}

const componentsByTone: Record<RichTextTone, Components> = {
  default: components('default'),
  onDark: components('onDark'),
}

type RichTextProps = {
  /** A ticket description or message body (Markdown). */
  markdown: string
  /** "onDark" for the navy UAT banners. */
  tone?: RichTextTone
  className?: string
}

/** A Markdown body, formatted like the editor shows it (`.rich-text` in app.css). Raw HTML shows as text. */
export function RichText({ markdown, tone = 'default', className }: RichTextProps) {
  return (
    <div className={cx('rich-text', tone === 'onDark' && 'rich-text-on-dark', className)}>
      <Markdown remarkPlugins={remarkPlugins} components={componentsByTone[tone]}>
        {markdown}
      </Markdown>
    </div>
  )
}
