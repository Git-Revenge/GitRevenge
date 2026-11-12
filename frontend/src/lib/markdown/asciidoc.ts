/**
 * AsciiDoc, translated to Markdown for the rendered file view. It covers what a README or a
 * docs page usually holds: the title and sections, paragraphs and their inline marks, lists,
 * links and images, listing and literal blocks, quotes, admonitions and tables. Anything it
 * doesn't know stays as text. Nothing is ever turned into HTML: a passthrough block shows as
 * code, and an include names the file it would have brought in.
 */

const ADMONITIONS: Record<string, string> = { NOTE: 'Note', TIP: 'Tip', IMPORTANT: 'Important', WARNING: 'Warning', CAUTION: 'Caution' }

/** A fence long enough for any run of backticks in the text it wraps. */
function fence(text: string): string {
  const longest = Math.max(2, ...[...text.matchAll(/`+/g)].map((m) => m[0].length))
  return '`'.repeat(longest + 1)
}

/** AsciiDoc's inline marks, in a run of text, as Markdown's. */
export function asciidocInline(text: string, attrs: Record<string, string> = {}): string {
  const held: string[] = []
  const hold = (s: string) => `${held.push(s) - 1}`
  let s = text
  // Literal text first, so nothing inside it is read as a mark.
  s = s.replace(/`\+(.+?)\+`/g, (_, t: string) => hold('`' + t + '`'))
  s = s.replace(/(^|[^\w`])`([^`\s][^`]*?)`(?![\w`])/g, (_, pre: string, t: string) => pre + hold('`' + t + '`'))
  s = s.replace(/(^|[^\w+])\+([^+\s][^+]*?)\+(?![\w+])/g, (_, pre: string, t: string) => pre + hold('`' + t + '`'))
  // Attribute references.
  s = s.replace(/\{([A-Za-z0-9_-]+)\}/g, (m, name: string) => (name in attrs ? attrs[name] : m))
  // Images and links: image:x[alt], link:url[text], url[text], xref:file[text], <<id,text>>.
  s = s.replace(/image:(?!:)([^\s[]+)\[([^\]]*)\]/g, (_, src: string, alt: string) => hold(`![${alt.split(',')[0].trim()}](${src})`))
  s = s.replace(/(?:link:)?((?:https?|ftp|mailto|irc):[^\s[\]]+)\[([^\]]*)\]/g, (_, url: string, label: string) => hold(`[${label.replace(/\^$/, '').trim() || url}](${url})`))
  s = s.replace(/link:([^\s[]+)\[([^\]]*)\]/g, (_, url: string, label: string) => hold(`[${label.trim() || url}](${url})`))
  s = s.replace(/xref:([^\s[]+)\[([^\]]*)\]/g, (_, ref: string, label: string) => hold(`[${label.trim() || ref.replace(/#.*$/, '')}](${ref.replace(/\.adoc(?=#|$)/, '.adoc')})`))
  s = s.replace(/<<([^,>]+)(?:,\s*([^>]+))?>>/g, (_, id: string, label?: string) => (label ?? id).trim())
  // Bold before italic: AsciiDoc's *x* is Markdown's **x**, and its _x_ is Markdown's *x*.
  s = s.replace(/\*\*(.+?)\*\*/g, (_, t: string) => hold('**') + t + hold('**'))
  s = s.replace(/(^|[^\w*])\*([^*\s](?:[^*]*[^*\s])?)\*(?![\w*])/g, (_, pre: string, t: string) => pre + hold('**') + t + hold('**'))
  s = s.replace(/__(.+?)__/g, (_, t: string) => hold('*') + t + hold('*'))
  s = s.replace(/(^|[^\w_])_([^_\s](?:[^_]*[^_\s])?)_(?![\w_])/g, (_, pre: string, t: string) => pre + hold('*') + t + hold('*'))
  // Highlighted text keeps its words; a role like [.role]#x# loses its role.
  s = s.replace(/(?:\[[^\]]*\])?#([^#\s][^#]*?)#/g, '$1')
  // A hard line break.
  s = s.replace(/ \+$/, '\\')
  return s.replace(/(\d+)/g, (_, i: string) => held[Number(i)])
}

interface Pending {
  style: string
  lang: string
  title: string
  header: boolean
}

/** A table's lines, read into rows of cells as Markdown writes them. */
function table(lines: string[], header: boolean, attrs: Record<string, string>): string[] {
  const firstCells = lines.find((l) => l.trim().startsWith('|'))
  const firstIndex = firstCells ? lines.indexOf(firstCells) : -1
  const width = firstCells ? firstCells.trim().slice(1).split('|').length : 0
  // A first row followed by a blank line is a header row.
  const implicitHeader = firstIndex >= 0 && lines[firstIndex + 1]?.trim() === ''
  const cells: string[] = []
  for (const line of lines) {
    const t = line.trim()
    if (!t) continue
    if (!t.startsWith('|')) {
      // A cell's text that runs on to another line.
      if (cells.length) cells[cells.length - 1] += ' ' + t
      continue
    }
    for (const c of t.slice(1).split('|')) cells.push(c.trim())
  }
  if (!width || !cells.length) return []
  const rows: string[][] = []
  for (let i = 0; i < cells.length; i += width) rows.push(cells.slice(i, i + width))
  const cell = (c: string | undefined) => asciidocInline(c ?? '', attrs).replace(/\|/g, '\\|')
  const line = (r: string[]) => `| ${Array.from({ length: width }, (_, k) => cell(r[k])).join(' | ')} |`
  const hasHeader = header || implicitHeader
  const head = hasHeader ? rows.shift()! : Array.from({ length: width }, () => '')
  return [line(head), `|${' --- |'.repeat(width)}`, ...rows.map(line)]
}

/** A whole AsciiDoc document as Markdown. */
export function asciidocToMarkdown(text: string): string {
  const lines = text.replace(/\r\n?/g, '\n').split('\n')
  const out: string[] = []
  const attrs: Record<string, string> = {}
  let pending: Pending = { style: '', lang: '', title: '', header: false }
  const clear = () => (pending = { style: '', lang: '', title: '', header: false })
  let i = 0
  let inHeader = false

  /** The lines up to a closing delimiter the same as the opening one. */
  const block = (delim: string): string[] => {
    const body: string[] = []
    i++
    while (i < lines.length && lines[i].trimEnd() !== delim) body.push(lines[i++])
    i++
    return body
  }
  const titleLine = () => {
    if (pending.title) out.push(`**${asciidocInline(pending.title, attrs)}**`, '')
  }

  while (i < lines.length) {
    const raw = lines[i]
    const line = raw.trimEnd()
    const t = line.trim()

    // The document header: the title, then its author and revision lines, up to a blank line.
    if (i === 0 || (out.length === 0 && /^=\s/.test(line))) {
      const doc = /^=\s+(.+)$/.exec(line)
      if (doc) {
        out.push(`# ${asciidocInline(doc[1], attrs)}`, '')
        inHeader = true
        i++
        continue
      }
    }
    if (inHeader) {
      if (!t) inHeader = false
      else if (!/^:[^:]+:/.test(t) && !t.startsWith('//')) {
        out.push(`*${asciidocInline(t, attrs)}*`, '')
        i++
        continue
      }
    }

    // Comments.
    if (t === '////') {
      block('////')
      continue
    }
    if (/^\/\/(?!\/)/.test(t)) {
      i++
      continue
    }
    // Attribute entries: kept for {references}, not shown.
    const attr = /^:(!?)([A-Za-z0-9_-]+)(!?):\s*(.*)$/.exec(t)
    if (attr) {
      if (attr[1] || attr[3]) delete attrs[attr[2]]
      else attrs[attr[2]] = attr[4]
      i++
      continue
    }
    // Block attributes and anchors, for the block that follows.
    const blockAttr = /^\[([^\]]*)\]$/.exec(t)
    if (blockAttr && !t.startsWith('[[')) {
      const parts = blockAttr[1].split(',').map((p) => p.trim())
      const first = parts[0].replace(/^%/, '')
      if (first === 'source' || first === 'listing') {
        pending.style = 'source'
        pending.lang = parts[1] ?? ''
      } else if (first) pending.style = first.replace(/^\./, '').replace(/[#%].*$/, '')
      if (/options\s*=\s*"?[^"]*header/.test(blockAttr[1]) || /%header/.test(blockAttr[1])) pending.header = true
      i++
      continue
    }
    if (/^\[\[[^\]]+\]\]$/.test(t)) {
      i++
      continue
    }
    // A block title.
    const blockTitle = /^\.([^.\s].*)$/.exec(t)
    if (blockTitle) {
      pending.title = blockTitle[1]
      i++
      continue
    }

    // Section titles: == Section is Markdown's ##.
    const section = /^(={2,6})\s+(.+?)\s*=*$/.exec(line)
    if (section) {
      out.push(`${'#'.repeat(section[1].length)} ${asciidocInline(section[2], attrs)}`, '')
      clear()
      i++
      continue
    }

    // Delimited blocks.
    if (/^-{4,}$/.test(t) || /^\.{4,}$/.test(t) || /^\+{4,}$/.test(t)) {
      const body = block(t)
      titleLine()
      const lang = t.startsWith('-') && pending.style === 'source' ? pending.lang : t.startsWith('+') ? 'html' : ''
      const code = body.join('\n')
      const f = fence(code)
      out.push(f + lang, code, f, '')
      clear()
      continue
    }
    if (/^_{4,}$/.test(t) || /^\*{4,}$/.test(t)) {
      const body = asciidocToMarkdown(block(t).join('\n')).trim()
      titleLine()
      for (const l of body.split('\n')) out.push(l ? `> ${l}` : '>')
      out.push('')
      clear()
      continue
    }
    if (/^={4,}$/.test(t) || t === '--') {
      const style = pending.style.toUpperCase()
      const body = asciidocToMarkdown(block(t).join('\n')).trim()
      if (ADMONITIONS[style]) {
        out.push(`> **${ADMONITIONS[style]}:**${pending.title ? ` ${asciidocInline(pending.title, attrs)}` : ''}`, '>')
        for (const l of body.split('\n')) out.push(l ? `> ${l}` : '>')
      } else {
        titleLine()
        out.push(body)
      }
      out.push('')
      clear()
      continue
    }
    if (/^\|={3,}$/.test(t)) {
      const body = block(t)
      titleLine()
      out.push(...table(body, pending.header, attrs), '')
      clear()
      continue
    }

    // Block macros.
    const image = /^image::([^[]+)\[([^\]]*)\]$/.exec(t)
    if (image) {
      titleLine()
      out.push(`![${image[2].split(',')[0].trim() || pending.title}](${image[1]})`, '')
      clear()
      i++
      continue
    }
    const include = /^include::([^[]+)\[/.exec(t)
    if (include) {
      out.push(`*(This includes ${include[1]}, which isn't shown here.)*`, '')
      i++
      continue
    }
    if (/^(toc|video|audio)::/.test(t)) {
      i++
      continue
    }

    // Lists: * or - bullets (** deeper), . numbers (.. deeper), term:: definitions.
    const bullet = /^(\*{1,5}|-)\s+(.*)$/.exec(t)
    if (bullet) {
      const depth = bullet[1] === '-' ? 0 : bullet[1].length - 1
      out.push(`${'  '.repeat(depth)}- ${asciidocInline(bullet[2], attrs)}`)
      i++
      if (lines[i]?.trim() === '') out.push('')
      continue
    }
    const numbered = /^(\.{1,5})\s+(.*)$/.exec(t) ?? /^(\d+)\.\s+(.*)$/.exec(t)
    if (numbered) {
      const depth = /^\d/.test(numbered[1]) ? 0 : numbered[1].length - 1
      out.push(`${'   '.repeat(depth)}1. ${asciidocInline(numbered[2], attrs)}`)
      i++
      continue
    }
    // A term ends at :: or ;; with a space or the line's end after it, so std::vector isn't one.
    const term = /^([^`]+?)(::|;;)(?:\s+(.*))?$/.exec(t)
    if (term && !/^[a-z]+:\/\//i.test(t) && !t.includes('image::')) {
      out.push(`- **${asciidocInline(term[1], attrs)}**${term[3] ? ` ${asciidocInline(term[3], attrs)}` : ''}`)
      i++
      continue
    }
    // A list item's text that carries on (+ joins a block to it).
    if (t === '+') {
      i++
      continue
    }

    // An admonition paragraph: NOTE: text.
    const note = /^(NOTE|TIP|IMPORTANT|WARNING|CAUTION):\s+(.*)$/.exec(t)
    if (note) {
      const para = [note[2]]
      i++
      while (i < lines.length && lines[i].trim()) para.push(lines[i++].trim())
      out.push(`> **${ADMONITIONS[note[1]]}:** ${para.map((p) => asciidocInline(p, attrs)).join(' ')}`, '')
      continue
    }
    // A literal paragraph: indented lines.
    if (/^\s+\S/.test(raw) && (out.length === 0 || out[out.length - 1] === '')) {
      const para: string[] = []
      while (i < lines.length && /^\s+\S/.test(lines[i])) para.push(lines[i++])
      const indent = Math.min(...para.map((p) => p.length - p.trimStart().length))
      const code = para.map((p) => p.slice(indent)).join('\n')
      out.push(fence(code), code, fence(code), '')
      continue
    }

    if (!t) {
      if (out.length && out[out.length - 1] !== '') out.push('')
      i++
      continue
    }

    // A paragraph line.
    if (pending.style === 'quote' || pending.style === 'verse') {
      const para: string[] = []
      while (i < lines.length && lines[i].trim()) para.push(lines[i++].trim())
      out.push(...para.map((p) => `> ${asciidocInline(p, attrs)}`), '')
      clear()
      continue
    }
    if (pending.title) {
      titleLine()
      pending.title = ''
    }
    out.push(asciidocInline(line.trimStart(), attrs))
    i++
  }
  return out.join('\n').replace(/\n{3,}/g, '\n\n').trim() + '\n'
}
