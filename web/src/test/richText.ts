import type { UserEvent } from '@testing-library/user-event'

/**
 * Writes text into a RichTextField by pasting it at the caret. Typing key by key is unreliable in
 * jsdom: ProseMirror reads typed text back from DOM mutations and moves its own caret on timers,
 * and with no layout the two race (letters go missing). A paste is one ProseMirror transaction.
 */
export async function writeRichText(user: UserEvent, editor: HTMLElement, text: string) {
  await user.click(editor)
  await user.paste(text)
}
