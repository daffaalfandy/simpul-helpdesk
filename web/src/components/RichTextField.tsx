import { Extension } from '@tiptap/core'
import { Placeholder } from '@tiptap/extensions'
import { Markdown } from '@tiptap/markdown'
import { EditorContent, useEditor, useEditorState, type Editor, type UseEditorOptions } from '@tiptap/react'
import StarterKit from '@tiptap/starter-kit'
import { Bold, Code, Italic, Link as LinkIcon, List, ListOrdered, SquareCode, TextQuote, type LucideIcon } from 'lucide-react'
import { useEffect, useId, useRef, useState, type CSSProperties, type FormEvent, type ReactNode } from 'react'
import {
  Button as AriaButton,
  Dialog,
  DialogTrigger,
  Form,
  Popover,
  Separator,
  ToggleButton,
  Toolbar,
  Tooltip,
  TooltipTrigger,
} from 'react-aria-components'
import { useTranslation } from 'react-i18next'

import { DATE_LOCALE } from '../i18n/locale'
import { LINK_PROTOCOLS, normalizeHref } from '../lib/links'
import { cx, runeLength } from '../lib/text'
import { Button } from './Button'
import { fieldLabel, type FieldSize } from './fieldStyles'
import { TextInput } from './TextInput'

const DEFAULT_ROWS = 4
/** How long a pointer rests on a tool before its name and shortcut show. */
const TOOLTIP_DELAY_MS = 600
/** Gap between a tool and its tooltip, in pixels (React Aria's offset prop takes a number). */
const TOOLTIP_OFFSET_PX = 4

type Keys = { key: string; shift?: boolean; alt?: boolean }
type Tool = {
  id: 'bold' | 'italic' | 'code' | 'bulletList' | 'orderedList' | 'blockquote' | 'codeBlock'
  icon: LucideIcon
  /** Tiptap's own shortcut for the same command (Mod is Cmd on a Mac, Ctrl elsewhere). */
  keys: Keys
  run: (editor: Editor) => void
}

const markTools: Tool[] = [
  { id: 'bold', icon: Bold, keys: { key: 'B' }, run: (e) => e.chain().focus().toggleBold().run() },
  { id: 'italic', icon: Italic, keys: { key: 'I' }, run: (e) => e.chain().focus().toggleItalic().run() },
  { id: 'code', icon: Code, keys: { key: 'E' }, run: (e) => e.chain().focus().toggleCode().run() },
]
const blockTools: Tool[] = [
  { id: 'bulletList', icon: List, keys: { key: '8', shift: true }, run: (e) => e.chain().focus().toggleBulletList().run() },
  { id: 'orderedList', icon: ListOrdered, keys: { key: '7', shift: true }, run: (e) => e.chain().focus().toggleOrderedList().run() },
  { id: 'blockquote', icon: TextQuote, keys: { key: 'B', shift: true }, run: (e) => e.chain().focus().toggleBlockquote().run() },
  { id: 'codeBlock', icon: SquareCode, keys: { key: 'C', alt: true }, run: (e) => e.chain().focus().toggleCodeBlock().run() },
]
/** Tiptap's Link has no shortcut; Mod-K is the common one for "add a link". */
const linkKeys: Keys = { key: 'K' }
const toolIds = [...markTools, ...blockTools].map((tool) => tool.id)

const isMac = typeof navigator !== 'undefined' && /Mac|iPhone|iPad/.test(navigator.userAgent)
/** How key names join in a shortcut: "⌘⇧8" on a Mac, "Ctrl+Shift+8" elsewhere. */
const KEY_JOIN = { mac: '', other: '+' } as const

/** Opens the link popover on Mod-K from inside the editor. */
const LinkShortcut = Extension.create<{ onOpen: () => void }>({
  name: 'linkShortcut',
  addOptions: () => ({ onOpen: () => {} }),
  addKeyboardShortcuts() {
    return {
      [`Mod-${linkKeys.key.toLowerCase()}`]: () => {
        this.options.onOpen()
        return true
      },
    }
  },
})

/** The editor's Markdown, or '' when it holds nothing but an empty paragraph. */
function markdownOf(editor: Editor): string {
  return editor.isEmpty ? '' : editor.getMarkdown()
}

/** "Ctrl+Shift+8" or "⌘⇧8", from translated key names. */
function useShortcutLabel() {
  const { t } = useTranslation()
  const platform = isMac ? 'mac' : 'other'
  return (keys: Keys) => {
    const parts = [t(`editor.keys.${platform}.mod`)]
    if (keys.shift) parts.push(t(`editor.keys.${platform}.shift`))
    if (keys.alt) parts.push(t(`editor.keys.${platform}.alt`))
    parts.push(keys.key)
    return parts.join(KEY_JOIN[platform])
  }
}

/** The same shortcut in ARIA's syntax (aria-keyshortcuts). */
function ariaKeys(keys: Keys): string {
  return [isMac ? 'Meta' : 'Control', keys.shift && 'Shift', keys.alt && 'Alt', keys.key].filter(Boolean).join('+')
}

type RichTextFieldProps = {
  label: ReactNode
  hideLabel?: boolean
  description?: ReactNode
  placeholder?: string
  /** Lines of text the empty field shows (its minimum height). */
  rows?: number
  size?: FieldSize
  /** "internal" turns the field yellow, like an internal note (HANDOFF §14.3 Tabs). */
  tone?: 'default' | 'internal'
  /** Markdown. Setting it to something other than what the field last reported (e.g. '' after sending) replaces the content. */
  value: string
  onChange: (markdown: string) => void
  isRequired?: boolean
  isInvalid?: boolean
  isDisabled?: boolean
  errorMessage?: string
  /** Longest Markdown the API takes, in characters; past it the field shows an error. */
  maxLength?: number
  className?: string
}

/**
 * A text field with formatting (HANDOFF §14.3 field look, plus a toolbar). It reads and writes
 * Markdown, so bodies stay plain text for emails and the MCP server.
 */
export function RichTextField({
  label,
  hideLabel,
  description,
  placeholder,
  rows = DEFAULT_ROWS,
  size = 'md',
  tone = 'default',
  value,
  onChange,
  isRequired,
  isInvalid,
  isDisabled,
  errorMessage,
  maxLength,
  className,
}: RichTextFieldProps) {
  const { t } = useTranslation()
  const labelId = useId()
  const descriptionId = useId()
  const errorId = useId()
  const [linkOpen, setLinkOpen] = useState(false)
  // What the field last reported, so a parent's own value (a reset) can be told apart from typing.
  const reported = useRef(value)

  // useEditor re-applies its options whenever one differs from the last render's, which would
  // reset the view on every keystroke; so they are made once. The placeholder is read here only:
  // each composer tab mounts its own field.
  const [options] = useState<UseEditorOptions>(() => ({
    extensions: [
      StarterKit.configure({
        heading: false,
        horizontalRule: false,
        // Only shows beside images and rules, which these fields don't have.
        gapcursor: false,
        underline: false,
        link: {
          openOnClick: false,
          autolink: true,
          defaultProtocol: 'https',
          isAllowedUri: (url) => normalizeHref(url) !== null,
        },
      }),
      Placeholder.configure({ placeholder: placeholder ?? '' }),
      Markdown,
      LinkShortcut.configure({ onOpen: () => setLinkOpen(true) }),
    ],
    content: value,
    contentType: 'markdown',
    // The app's CSP allows no inline <style>; the editor's few rules live in app.css.
    injectCSS: false,
    shouldRerenderOnTransaction: false,
    editorProps: {
      attributes: { role: 'textbox', 'aria-multiline': 'true', 'aria-labelledby': labelId, class: 'rich-text' },
    },
  }))
  const editor = useEditor(options)

  useEffect(() => {
    const update = () => {
      const markdown = markdownOf(editor)
      reported.current = markdown
      onChange(markdown)
    }
    editor.on('update', update)
    return () => {
      editor.off('update', update)
    }
  }, [editor, onChange])

  useEffect(() => {
    if (value === reported.current) return
    reported.current = value
    editor.commands.setContent(value, { contentType: 'markdown', emitUpdate: false })
  }, [editor, value])

  useEffect(() => {
    editor.setEditable(!isDisabled, false)
  }, [editor, isDisabled])

  const tooLong = maxLength !== undefined && runeLength(value) > maxLength
  const invalid = Boolean(isInvalid || tooLong)
  const error = tooLong ? t('editor.tooLong', { max: maxLength?.toLocaleString(DATE_LOCALE) }) : isInvalid ? errorMessage : undefined

  // Attributes that change with the field's state go straight onto the editable element: passing
  // them through editor.setOptions would re-apply the view state while keys may still be in flight.
  useEffect(() => {
    const dom = editor.view.dom
    const describedBy = [description && descriptionId, error && errorId].filter(Boolean).join(' ')
    const set = (name: string, value: string | false | undefined) =>
      value ? dom.setAttribute(name, value) : dom.removeAttribute(name)
    set('aria-invalid', String(invalid))
    set('aria-describedby', describedBy)
    set('aria-required', isRequired && 'true')
    set('aria-disabled', isDisabled && 'true')
    set('aria-placeholder', placeholder)
  }, [editor, description, descriptionId, error, errorId, invalid, isRequired, isDisabled, placeholder])

  const active = useEditorState({
    editor,
    selector: ({ editor }) => ({
      tools: Object.fromEntries(toolIds.map((id) => [id, editor.isActive(id)])) as Record<Tool['id'], boolean>,
      link: editor.isActive('link'),
    }),
  })

  const internal = tone === 'internal'
  // React's style prop sets the CSS variable through the DOM, which the CSP allows.
  const rowsStyle = { '--editor-rows': rows } as CSSProperties

  return (
    <div className={cx('flex flex-col gap-1.5', className)}>
      <span id={labelId} className={hideLabel ? 'sr-only' : fieldLabel(size)} onClick={() => editor.commands.focus()}>
        {label}
      </span>
      <div
        data-invalid={invalid || undefined}
        data-disabled={isDisabled || undefined}
        className={cx(
          'overflow-hidden rounded-lg border transition-shadow duration-(--duration-fast) ease-standard',
          'has-[.ProseMirror:focus]:shadow-(--focus-ring) data-invalid:border-danger',
          internal ? 'border-note-internal-border bg-note-internal-bg' : 'border-border-strong bg-surface',
        )}
      >
        <Toolbar
          aria-label={t('editor.toolbar')}
          className={cx(
            'flex flex-wrap items-center gap-2 border-b px-1.5 py-1',
            internal ? 'border-note-internal-border' : 'border-border',
          )}
        >
          {markTools.map((tool) => (
            <ToolButton key={tool.id} tool={tool} editor={editor} isSelected={active.tools[tool.id]} isDisabled={isDisabled} />
          ))}
          <Separator orientation="vertical" className={cx('h-(--icon-lg) border-l', internal ? 'border-note-internal-border' : 'border-border')} />
          <LinkTool editor={editor} isOpen={linkOpen} onOpenChange={setLinkOpen} isActive={active.link} isDisabled={isDisabled} />
          <Separator orientation="vertical" className={cx('h-(--icon-lg) border-l', internal ? 'border-note-internal-border' : 'border-border')} />
          {blockTools.map((tool) => (
            <ToolButton key={tool.id} tool={tool} editor={editor} isSelected={active.tools[tool.id]} isDisabled={isDisabled} />
          ))}
        </Toolbar>
        <EditorContent
          editor={editor}
          style={rowsStyle}
          // A click in the padding around the text still puts the caret in it, as in a textarea.
          onClick={(e) => e.target === e.currentTarget && editor.commands.focus('end')}
          className={cx(
            'rich-text-field max-h-(--editor-max-h) overflow-y-auto px-3.5 py-2.5 leading-relaxed',
            size === 'lg' ? 'text-md' : 'text-base',
            isDisabled ? 'text-muted' : 'text-ink',
          )}
        />
      </div>
      {description && (
        <p id={descriptionId} className="text-xs text-muted">
          {description}
        </p>
      )}
      {error && (
        <p id={errorId} className="text-xs font-semibold text-danger">
          {error}
        </p>
      )}
    </div>
  )
}

const toolClasses = cx(
  'hit-area inline-flex size-(--control-h-sm) items-center justify-center rounded-md text-muted outline-none',
  'transition-colors duration-(--duration-fast) ease-standard',
  'data-hovered:bg-surface-subtle data-hovered:text-ink data-pressed:bg-surface-subtle',
  'data-selected:bg-primary-tint data-selected:text-primary data-focus-visible:shadow-(--focus-ring)',
  'data-disabled:text-disabled data-disabled:bg-transparent',
)

function ToolTip({ children }: { children: ReactNode }) {
  return (
    <Tooltip
      offset={TOOLTIP_OFFSET_PX}
      className="rounded-md bg-ink px-2 py-1 text-xs font-semibold text-on-primary shadow-popover data-entering:animate-fade-in"
    >
      {children}
    </Tooltip>
  )
}

function ToolButton({ tool, editor, isSelected, isDisabled }: { tool: Tool; editor: Editor; isSelected: boolean; isDisabled?: boolean }) {
  const { t } = useTranslation()
  const shortcut = useShortcutLabel()
  const Icon = tool.icon
  const name = t(`editor.tools.${tool.id}`)
  return (
    <TooltipTrigger delay={TOOLTIP_DELAY_MS}>
      <ToggleButton
        aria-label={name}
        aria-keyshortcuts={ariaKeys(tool.keys)}
        isSelected={isSelected}
        isDisabled={isDisabled}
        onChange={() => tool.run(editor)}
        className={toolClasses}
      >
        <Icon aria-hidden="true" className="size-(--icon-md)" />
      </ToggleButton>
      <ToolTip>{t('editor.withShortcut', { name, keys: shortcut(tool.keys) })}</ToolTip>
    </TooltipTrigger>
  )
}

type LinkToolProps = {
  editor: Editor
  isOpen: boolean
  onOpenChange: (open: boolean) => void
  isActive: boolean
  isDisabled?: boolean
}

/** Link button and its popover: type an address, save it on the selection, or remove the link. */
function LinkTool({ editor, isOpen, onOpenChange, isActive, isDisabled }: LinkToolProps) {
  const { t } = useTranslation()
  const shortcut = useShortcutLabel()
  const name = t('editor.tools.link')
  return (
    <DialogTrigger isOpen={isOpen} onOpenChange={onOpenChange}>
      <TooltipTrigger delay={TOOLTIP_DELAY_MS}>
        <AriaButton
          aria-label={name}
          aria-keyshortcuts={ariaKeys(linkKeys)}
          data-selected={isActive || undefined}
          isDisabled={isDisabled}
          className={toolClasses}
        >
          <LinkIcon aria-hidden="true" className="size-(--icon-md)" />
        </AriaButton>
        <ToolTip>{t('editor.withShortcut', { name, keys: shortcut(linkKeys) })}</ToolTip>
      </TooltipTrigger>
      <Popover
        placement="bottom start"
        className={cx(
          'w-(--link-popover-w) max-w-[calc(100vw-var(--space-8))] rounded-xl border border-border bg-surface shadow-popover outline-none',
          'data-entering:animate-enter',
        )}
      >
        <Dialog aria-label={t('editor.link.title')} className="p-4 outline-none">
          {({ close }) => <LinkForm editor={editor} isActive={isActive} onDone={close} />}
        </Dialog>
      </Popover>
    </DialogTrigger>
  )
}

function LinkForm({ editor, isActive, onDone }: { editor: Editor; isActive: boolean; onDone: () => void }) {
  const { t } = useTranslation()
  const [address, setAddress] = useState(() => (editor.getAttributes('link').href as string | undefined) ?? '')
  const [tried, setTried] = useState(false)
  const href = normalizeHref(address)

  // Closing hands focus back to the toolbar; the editor takes it after that, so typing goes on.
  const finish = () => {
    onDone()
    window.setTimeout(() => editor.commands.focus())
  }

  const save = (e: FormEvent) => {
    e.preventDefault()
    // The popover is a portal, but React still bubbles its submit to the composer's own form,
    // which would send the message.
    e.stopPropagation()
    setTried(true)
    if (!href) return
    const chain = editor.chain().extendMarkRange('link')
    if (editor.state.selection.empty && !isActive) {
      chain.insertContent({ type: 'text', text: href, marks: [{ type: 'link', attrs: { href } }] }).run()
    } else {
      chain.setLink({ href }).run()
    }
    finish()
  }

  const remove = () => {
    editor.chain().extendMarkRange('link').unsetLink().run()
    finish()
  }

  return (
    <Form onSubmit={save} validationBehavior="aria" className="flex flex-col gap-3">
      <TextInput
        label={t('editor.link.address')}
        type="url"
        inputMode="url"
        autoFocus
        placeholder={t('editor.link.placeholder')}
        value={address}
        onChange={setAddress}
        isInvalid={tried && !href}
        errorMessage={t('editor.link.invalid', { protocols: LINK_PROTOCOLS.map((p) => p.replace(':', '')).join(', ') })}
      />
      <div className="flex flex-wrap justify-end gap-2">
        {isActive && (
          <Button variant="outline" size="sm" onPress={remove}>
            {t('editor.link.remove')}
          </Button>
        )}
        <Button type="submit" size="sm">
          {t('editor.link.save')}
        </Button>
      </div>
    </Form>
  )
}
