import { render } from 'svelte/server'
import { describe, expect, it, vi } from 'vitest'

// The Wails runtime only exists in the app; links don't need it to render.
vi.mock('../../wailsjs/runtime/runtime.js', () => ({ BrowserOpenURL: vi.fn() }))
// The theme store reads window when it loads; diagrams only need to know light or dark.
vi.mock('./theme.svelte', () => ({ theme: { resolved: 'light' } }))

const { default: Markdown } = await import('./Markdown.svelte')

/** Drops Svelte's hydration comments, which may span lines, until none are left. */
function withoutComments(out: string): string {
  let prev
  do {
    prev = out
    out = out.replace(/<!--[\s\S]*?-->/g, '')
  } while (out !== prev)
  return out
}

/** Rendered HTML without hydration comments or scoped class names. */
const html = (text: string, issueURL = '', extra: Record<string, unknown> = {}) =>
  withoutComments(render(Markdown, { props: { text, issueURL, ...extra } }).body).replace(/ class="[^"]*"/g, '')

describe('Markdown', () => {
  describe('links inside links', () => {
    const links = (out: string) => out.match(/<a /g)?.length ?? 0

    it('makes a bare URL one link, so a click opens it once', () => {
      const out = html('See https://example.com/x for details')
      expect(links(out)).toBe(1)
      expect(out).toContain('>https://example.com/x</a>')
    })

    it('leaves an issue reference in the text of a link alone', () => {
      const out = html('[fixes #12](https://example.com/x)', 'https://example.com/issues/{n}')
      expect(links(out)).toBe(1)
      expect(out).toContain('fixes #12</a>')
    })

    it('keeps formatting in the text of a link without linking it again', () => {
      const out = html('[**https://example.com/y**](https://example.com/x)')
      expect(links(out)).toBe(1)
      expect(out).toContain('<strong>https://example.com/y</strong>')
    })

    it('shows an unloaded image in a link as its alt text, so a badge opens only the link', () => {
      const out = html('[![build](https://example.com/badge.svg)](https://example.com/ci)')
      expect(links(out)).toBe(1)
      expect(out).toContain('>build</a>')
      expect(out).not.toContain('badge.svg')
    })

    it('still links text outside a link', () => {
      expect(links(html('#12 and [docs](https://example.com/x)', 'https://example.com/issues/{n}'))).toBe(2)
    })
  })

  it('renders emphasis, code, lists and links', () => {
    const out = html('Some **bold**, *em* and `x < y`.\n\n- one\n- two\n\nSee [docs](https://example.com).')
    expect(out).toContain('<strong>bold</strong>')
    expect(out).toContain('<em>em</em>')
    expect(out).toContain('<code>x &lt; y</code>')
    expect(out).toMatch(/<ul[^>]*>.*<li>.*one.*<\/li>.*<li>.*two.*<\/li>.*<\/ul>/s)
    expect(out).toContain('href="https://example.com"')
  })

  it('shows raw HTML as text, never as markup', () => {
    const out = html('<script>alert(1)</script>\n\nand <b>inline</b> too')
    expect(out).not.toContain('<script>')
    expect(out).not.toContain('<b>')
    expect(out).toContain('&lt;script>alert(1)&lt;/script>')
  })

  it('links issue references when the remote is known', () => {
    expect(html('Fixes the bug in #12.', 'https://github.com/o/r/issues/{n}')).toContain('href="https://github.com/o/r/issues/12"')
    expect(html('Fixes the bug in #12.')).not.toContain('href=')
  })

  it('keeps an indented block as code', () => {
    expect(html('Trace:\n\n    at foo (bar.go:12)')).toMatch(/<pre><code>at foo \(bar.go:12\)<\/code><\/pre>/)
  })

  it('keeps heading levels in a document, under the view heading', () => {
    const out = html('# Title\n\n## Part', '', { document: true })
    expect(out).toContain('<h2>Title</h2>')
    expect(out).toContain('<h3>Part</h3>')
    expect(html('# Title')).toContain('<h3>Title</h3>')
  })

  it('never loads a remote image, and waits on the resolver for a relative one', () => {
    const resolveImage = vi.fn(() => new Promise<string>(() => {}))
    const out = html('![logo](https://example.com/logo.png) ![dot](img/dot.png)', '', { document: true, resolveImage })
    expect(out).not.toMatch(/<img[^>]*example\.com/)
    expect(out).toContain('href="https://example.com/logo.png"')
    expect(out).toContain('[dot]') // placeholder until the repository image loads
  })

  it('draws mermaid blocks in documents only', () => {
    const md = '```mermaid\ngraph TD; A-->B\n```'
    expect(html(md, '', { document: true })).toContain('Drawing…')
    expect(html(md)).toContain('<pre><code>graph TD; A--')
    expect(html(md)).not.toContain('Drawing')
  })
})
