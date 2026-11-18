/** An address that may be handed to the system browser. */
export const WEB_ADDRESS = /^(https?:|mailto:)/i

/**
 * Keeps the app's window on the app, whatever is done to a link. A link's own click handler
 * decides what a click does; this covers what never reaches it:
 *
 * - a middle click, which a web view answers by opening the address in a window of its own.
 *   It opens a web address in the system browser, as a click does, and does nothing else;
 * - dragging a link or a picture, which would let its address be dropped somewhere;
 * - anything dropped on the window, which a web view answers by navigating to it. Text
 *   dropped on a text field still lands there.
 *
 * Links come from repository content (a README, a commit message), so their addresses are
 * whatever the repository's author wrote.
 */
export function guardLinks(win: Window, openInBrowser: (url: string) => void): void {
  const doc = win.document
  const within = (e: Event, selector: string) => (e.target instanceof Element ? e.target.closest(selector) : null)

  doc.addEventListener(
    'auxclick',
    (e) => {
      const a = within(e, 'a')
      if (!a) return
      e.preventDefault()
      const href = a.getAttribute('href') ?? ''
      if (e.button === 1 && WEB_ADDRESS.test(href)) openInBrowser(href)
    },
    true,
  )
  doc.addEventListener(
    'dragstart',
    (e) => {
      if (within(e, 'a, img')) e.preventDefault()
    },
    true,
  )
  for (const type of ['dragover', 'drop'] as const) {
    win.addEventListener(type, (e) => {
      if (!within(e, 'input, textarea, [contenteditable]')) e.preventDefault()
    })
  }
}
