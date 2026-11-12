import { resolveInRepo } from './paths'

/**
 * What the page is told it may load, set first thing in its <head>: nothing at all from the
 * network. Its own styles and the pictures handed to it as data URLs are all it gets.
 */
export const PAGE_POLICY = "default-src 'none'; img-src data:; media-src data:; font-src data:; style-src 'unsafe-inline'"

/** Elements that run code, load something of their own, or replace the page, which go. */
const GONE = 'script, iframe, frame, frameset, object, embed, applet, portal, base, link, meta[http-equiv]'

/**
 * An HTML file from the repository, made safe to show in a sandboxed frame. The frame already
 * runs no scripts, submits no forms and opens nothing; this also keeps the page from reaching
 * the network or the frame from being navigated away:
 *
 * - scripts, frames, plug-ins, <base>, <link> and <meta http-equiv> (a refresh) are removed,
 *   and so is every on… attribute;
 * - links keep their text and lose their address, which goes in their tooltip;
 * - a picture in the repository is read at the same version and inlined; one on the web is
 *   left out, its alt text in its place;
 * - the policy above goes first in <head>, for anything a stylesheet still asks for.
 *
 * The parsing is the browser's own (DOMParser), which loads nothing and runs nothing.
 */
export async function sandboxedPage(text: string, path: string, loadImage: (file: string) => Promise<string>): Promise<{ html: string; removed: number; remote: number }> {
  const doc = new DOMParser().parseFromString(text, 'text/html')
  let removed = 0
  let remote = 0
  for (const el of doc.querySelectorAll(GONE)) {
    el.remove()
    removed++
  }
  for (const el of doc.querySelectorAll('*')) {
    for (const attr of [...el.attributes]) {
      const name = attr.name.toLowerCase()
      if (name.startsWith('on') || name === 'srcset' || name === 'formaction' || name === 'ping') el.removeAttribute(attr.name)
      // A javascript: address anywhere is dead weight in a frame that runs no script.
      else if (/^\s*javascript:/i.test(attr.value)) el.removeAttribute(attr.name)
    }
  }
  for (const a of doc.querySelectorAll('a[href], area[href]')) {
    const href = a.getAttribute('href') ?? ''
    a.removeAttribute('href')
    if (href && !href.startsWith('#') && !a.getAttribute('title')) a.setAttribute('title', href)
  }
  for (const form of doc.querySelectorAll('form')) form.removeAttribute('action')
  await Promise.all(
    [...doc.querySelectorAll('img[src], video[poster], source[src], video[src], audio[src]')].map(async (el) => {
      const attr = el.hasAttribute('poster') && !el.hasAttribute('src') ? 'poster' : 'src'
      const src = el.getAttribute(attr) ?? ''
      if (/^data:image\//i.test(src)) return
      const file = resolveInRepo(path, src)
      const data = file && el.tagName === 'IMG' ? await loadImage(file).catch(() => '') : ''
      if (data) el.setAttribute(attr, data)
      else if (el.tagName === 'IMG') {
        // A picture that can't be shown becomes its words, not a broken-image icon.
        if (!file) remote++
        const words = doc.createElement('span')
        words.textContent = `[${el.getAttribute('alt') || src.split('/').pop() || 'picture'}]`
        words.setAttribute('title', file ? `${file}, which isn't in the repository at this version` : `${src}, which isn't fetched`)
        if (el.id) words.id = el.id
        el.replaceWith(words)
      } else {
        if (!file) remote++
        el.removeAttribute(attr)
      }
    }),
  )
  const policy = doc.createElement('meta')
  policy.setAttribute('http-equiv', 'Content-Security-Policy')
  policy.setAttribute('content', PAGE_POLICY)
  doc.head.prepend(policy)
  return { html: '<!doctype html>\n' + doc.documentElement.outerHTML, removed, remote }
}
