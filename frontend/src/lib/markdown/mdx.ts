/**
 * MDX turned into plain Markdown for the preview (#517). Nothing in the file runs: imports, exports,
 * frontmatter and comments go, a component's tags go and what's inside it stays, and a component
 * with nothing inside it shows its name, so the reader can see something belongs there.
 * Code blocks and inline code are left as written. {expressions} stay as text.
 */

interface Tag {
  name: string
  closing: boolean
  selfClosing: boolean
  /** The tag's whole source, attributes included. */
  source: string
  end: number
}

interface Line {
  text: string
  /** Started inside a component, where indenting doesn't mean a code block. */
  inside: boolean
  /** Held only tags, so it ends a run of lines. */
  tagOnly: boolean
}

const FRONTMATTER = /^---[ \t]*\n[\s\S]*?\n---[ \t]*(\n|$)/
const FENCE = /[ \t]*(`{3,}|~{3,})/y
const ESM = /(import|export)[ \t{*]/y
const TAG_START = /<(\/?)([A-Za-z][\w.-]*)?/y
const VOID_TAGS = new Set(['area', 'base', 'br', 'col', 'embed', 'hr', 'img', 'input', 'link', 'meta', 'source', 'track', 'wbr'])

const matchAt = (re: RegExp, text: string, at: number) => {
  re.lastIndex = at
  return re.exec(text)
}

/** Where a quoted string starting at `at` ends (past its closing quote). Strings stop at a line's end. */
function skipString(text: string, at: number): number {
  const quote = text[at]
  for (let i = at + 1; i < text.length; i++) {
    if (text[i] === '\\') i++
    else if (text[i] === quote) return i + 1
    else if (text[i] === '\n' && quote !== '`') return i
  }
  return text.length
}

/** Reads a JSX tag at `at`, or null if this `<` is only text, as in `a < b` or an autolink. */
function readTag(text: string, at: number): Tag | null {
  const start = matchAt(TAG_START, text, at)
  if (!start) return null
  const closing = start[1] === '/'
  const name = start[2] ?? ''
  let i = at + start[0].length
  const after = text[i]
  const selfClose = after === '/' && text[i + 1] === '>'
  if (name === '' ? after !== '>' : !(after === '>' || selfClose || /\s/.test(after ?? ''))) return null
  let braces = 0
  for (; i < text.length; i++) {
    const c = text[i]
    if (c === '"' || c === "'" || c === '`') {
      i = skipString(text, i) - 1
    } else if (c === '{') {
      braces++
    } else if (c === '}') {
      braces--
    } else if (braces === 0) {
      // A blank line or another tag before this one closes means it was never a tag.
      if (c === '<' || (c === '\n' && text[i + 1] === '\n')) return null
      if (c === '>') {
        const end = i + 1
        return { name, closing, selfClosing: text[i - 1] === '/', source: text.slice(at, end), end }
      }
    }
  }
  return null
}

/** Where the import or export starting at `at` ends: the first blank line outside any brackets. */
function esmEnd(text: string, at: number): number {
  let depth = 0
  for (let i = at; i < text.length; i++) {
    const c = text[i]
    if (c === '"' || c === "'" || c === '`') i = skipString(text, i) - 1
    else if ('{[('.includes(c)) depth++
    else if ('}])'.includes(c)) depth--
    else if (c === '\n' && depth <= 0 && (text[i + 1] === '\n' || i + 1 === text.length)) return i + 1
  }
  return text.length
}

const attribute = (source: string, name: string) => new RegExp(`\\s${name}=(?:"([^"]*)"|'([^']*)')`).exec(source)?.slice(1).find((v) => v !== undefined)

/** What stands in for a tag in the Markdown. */
function replacement(tag: Tag): string {
  if (tag.closing) return ''
  if (tag.name === 'img') {
    const src = attribute(tag.source, 'src')
    return src ? `![${attribute(tag.source, 'alt') ?? ''}](<${src}>)` : ''
  }
  return tag.selfClosing && /^[A-Z]/.test(tag.name) ? `\`<${tag.name} />\`` : ''
}

/** Takes the shared indent off each run of lines inside a component. */
function dedent(lines: Line[]): string[] {
  const out = lines.map((l) => l.text)
  const flush = (from: number, to: number) => {
    const indents = out.slice(from, to).flatMap((text) => (text.trim() ? [text.length - text.trimStart().length] : []))
    const indent = indents.length ? Math.min(...indents) : 0
    for (let i = from; i < to; i++) out[i] = out[i].slice(Math.min(indent, out[i].length))
  }
  let from = -1
  lines.forEach((l, i) => {
    const inRun = l.inside && !l.tagOnly
    if (inRun && from < 0) from = i
    if (!inRun && from >= 0) {
      flush(from, i)
      from = -1
    }
  })
  if (from >= 0) flush(from, lines.length)
  return out
}

export function mdxToMarkdown(source: string): string {
  const text = source.replace(/\r\n?/g, '\n').replace(FRONTMATTER, '')
  const lines: Line[] = []
  let line = ''
  let tagged = false
  let depth = 0
  let inside = false
  const endLine = () => {
    lines.push({ text: line, inside, tagOnly: tagged && !line.trim() })
    line = ''
    tagged = false
    inside = depth > 0
  }

  let i = 0
  while (i < text.length) {
    if (i === 0 || text[i - 1] === '\n') {
      const fence = matchAt(FENCE, text, i)
      if (fence) {
        i = copyFence(text, i, fence[1], lines, depth > 0)
        inside = depth > 0
        continue
      }
      const afterBlank = lines.length === 0 || !lines[lines.length - 1].text.trim()
      if (depth === 0 && afterBlank && matchAt(ESM, text, i)) {
        i = esmEnd(text, i)
        continue
      }
    }
    const c = text[i]
    if (c === '\n') {
      endLine()
      i++
    } else if (c === '\\') {
      line += text.slice(i, i + 2)
      i += 2
    } else if (c === '`') {
      const run = /`+/y
      const ticks = matchAt(run, text, i)![0]
      const close = closingTicks(text, i + ticks.length, ticks.length)
      const end = close < 0 ? i + ticks.length : close + ticks.length
      line += text.slice(i, end).replace(/\n/g, ' ')
      i = end
    } else if (text.startsWith('{/*', i)) {
      const close = text.indexOf('*/}', i)
      i = close < 0 ? text.length : close + 3
      tagged = true
    } else if (c === '<') {
      const tag = readTag(text, i)
      if (!tag) {
        line += c
        i++
        continue
      }
      line += replacement(tag)
      tagged = true
      if (tag.closing) depth = Math.max(0, depth - 1)
      else if (!tag.selfClosing && !VOID_TAGS.has(tag.name)) depth++
      i = tag.end
    } else {
      line += c
      i++
    }
  }
  if (line || tagged) endLine()
  const out = dedent(lines).join('\n').trimStart()
  return out && text.endsWith('\n') ? out + '\n' : out
}

/** Copies a fenced code block, fences included, and returns where it ends. */
function copyFence(text: string, at: number, fence: string, lines: Line[], inside: boolean): number {
  let i = at
  let first = true
  while (i < text.length) {
    const nl = text.indexOf('\n', i)
    const end = nl < 0 ? text.length : nl
    const content = text.slice(i, end)
    lines.push({ text: content, inside, tagOnly: false })
    i = nl < 0 ? text.length : nl + 1
    const trimmed = content.trim()
    if (!first && trimmed.length >= fence.length && trimmed === fence[0].repeat(trimmed.length)) break
    first = false
  }
  return i
}

/** The start of the run of exactly `count` backticks closing an inline code span, or -1. A blank line ends the search. */
function closingTicks(text: string, from: number, count: number): number {
  for (let i = from; i < text.length; i++) {
    if (text[i] === '\n' && text[i + 1] === '\n') return -1
    if (text[i] !== '`') continue
    let end = i
    while (text[end] === '`') end++
    if (end - i === count) return i
    i = end - 1
  }
  return -1
}
