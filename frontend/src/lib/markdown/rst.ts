/**
 * reStructuredText, translated to Markdown for the rendered file view. It covers what a README
 * or a Sphinx page usually holds: sections, paragraphs and their inline marks, lists, field
 * and definition lists, links (inline and named), images, literal and code blocks, admonitions,
 * math, line blocks and doctests. Tables keep their layout as preformatted text. Directives it
 * doesn't know are named, not shown. Nothing is ever turned into HTML.
 */

const ADMONITIONS = new Set(['note', 'tip', 'hint', 'important', 'warning', 'caution', 'danger', 'attention', 'error', 'seealso', 'admonition', 'versionadded', 'versionchanged', 'deprecated'])
const CODE = new Set(['code', 'code-block', 'sourcecode', 'highlight'])
/** Directives that only arrange a Sphinx site, which there's nothing to show of. */
const QUIET = new Set(['toctree', 'contents', 'index', 'only', 'meta', 'highlight', 'currentmodule', 'module', 'default-role', 'role', 'raw', 'sectnum', 'automodule', 'autoclass', 'autofunction'])
const TITLES: Record<string, string> = { seealso: 'See also', versionadded: 'New in version', versionchanged: 'Changed in version', deprecated: 'Deprecated since version' }

const PUNCT = /^([!-/:-@[-`{-~])\1+$/

/** How far a line is indented. */
const indentOf = (l: string) => l.length - l.trimStart().length

function fence(text: string): string {
  const longest = Math.max(2, ...[...text.matchAll(/`+/g)].map((m) => m[0].length))
  return '`'.repeat(longest + 1)
}

/** rST's inline marks, in a run of text, as Markdown's. `targets` are the document's named
 * links (.. _name: url), and `subs` its substitutions (.. |name| replace:: text). */
export function rstInline(text: string, targets: Record<string, string> = {}, subs: Record<string, string> = {}): string {
  const held: string[] = []
  const hold = (s: string) => `${held.push(s) - 1}`
  const code = (t: string) => {
    const f = t.includes('`') ? '``' : '`'
    return hold(f === '``' ? `\`\` ${t} \`\`` : `\`${t}\``)
  }
  let s = text
  s = s.replace(/``(.+?)``/g, (_, t: string) => code(t))
  // Roles: :math:`x`, :code:`x`, :ref:`text <label>`, :doc:`x`, :py:func:`x` and the rest.
  s = s.replace(/:([a-z][\w:.+-]*):`([^`]+)`/gi, (_, role: string, t: string) => {
    const r = role.toLowerCase()
    if (r === 'math') return hold(`$${t}$`)
    const shown = /^(.*?)\s*<([^>]+)>$/.exec(t)
    if (r === 'abbr') return t.replace(/\s*\([^)]*\)$/, '')
    if (r === 'emphasis') return hold(`*${t}*`)
    if (r === 'strong') return hold(`**${t}**`)
    if (r === 'sub' || r === 'sup' || r === 'subscript' || r === 'superscript') return t
    if (r === 'ref' || r === 'doc' || r === 'term' || r === 'numref') return shown ? shown[1] : t.replace(/^~/, '')
    return code(shown ? shown[1] : t.replace(/^[~!]/, ''))
  })
  // Links: `text <url>`_ and `text <url>`__, then `name`_ and name_ by the document's targets.
  s = s.replace(/`([^`<]*?)\s*<([^>`]+)>`__?/g, (_, label: string, url: string) => hold(`[${label.trim() || url}](${url})`))
  const named = (name: string) => targets[name.toLowerCase().replace(/\s+/g, ' ')]
  s = s.replace(/`([^`]+)`__?/g, (m, name: string) => (named(name) ? hold(`[${name}](${named(name)})`) : m))
  s = s.replace(/(^|[\s(])([A-Za-z0-9][\w.-]*)__?(?=$|[\s.,;:!?)])/g, (m, pre: string, name: string) => (named(name) ? pre + hold(`[${name}](${named(name)})`) : m))
  // Footnote and citation references: [1]_, [#]_, [#name]_, [CIT2002]_.
  s = s.replace(/\[(#?[\w-]*|\*)\]_/g, (_, label: string) => hold(`[${label.replace(/^#/, '') || '#'}]`))
  s = s.replace(/\|([^|\s][^|]*?)\|(_{0,2})/g, (m, name: string) => (name in subs ? subs[name] : m))
  // Strong before emphasis; the default role (`x`) reads as emphasis.
  s = s.replace(/\*\*(.+?)\*\*/g, (_, t: string) => hold('**') + t + hold('**'))
  s = s.replace(/(^|[^\w*\\])\*([^*\s](?:[^*]*[^*\s])?)\*(?![\w*])/g, (_, pre: string, t: string) => pre + hold('*') + t + hold('*'))
  s = s.replace(/(^|[^\w`])`([^`\s][^`]*?)`(?![\w`_])/g, (_, pre: string, t: string) => pre + hold('*') + t + hold('*'))
  return s.replace(/(\d+)/g, (_, i: string) => held[Number(i)])
}

/** A paragraph's line, safe from being read as Markdown structure it isn't. */
function guard(line: string): string {
  return line.replace(/^(#{1,6}\s|>|\d+[.)]\s)/, '\\$1')
}

/** A whole reStructuredText document as Markdown. */
export function rstToMarkdown(text: string): string {
  const lines = text.replace(/\r\n?/g, '\n').replace(/\t/g, '        ').split('\n')
  const targets: Record<string, string> = {}
  const subs: Record<string, string> = {}
  // Named links and substitutions can be defined after they're used: read them first.
  for (const l of lines) {
    const target = /^\.\.\s+_([^:]+):\s*(\S+)\s*$/.exec(l)
    if (target) targets[target[1].replace(/^`|`$/g, '').toLowerCase().replace(/\s+/g, ' ')] = target[2]
    const sub = /^\.\.\s+\|([^|]+)\|\s+(replace|image)::\s*(.+)$/.exec(l)
    if (sub) subs[sub[1]] = sub[2] === 'image' ? `![${sub[1]}](${sub[3].trim()})` : sub[3].trim()
  }
  const inline = (t: string) => rstInline(t, targets, subs)
  const out: string[] = []
  /** Section levels, by the style of their adornment, in the order they first appear. */
  const levels: string[] = []
  let i = 0

  /** The indented block from line `from`: its lines, dedented, and where it ends. */
  const indented = (from: number, base: number): { body: string[]; end: number } => {
    let end = from
    while (end < lines.length && (lines[end].trim() === '' || indentOf(lines[end]) > base)) end++
    while (end > from && lines[end - 1].trim() === '') end--
    const body = lines.slice(from, end)
    const indent = Math.min(...body.filter((l) => l.trim()).map(indentOf), Infinity)
    return { body: body.map((l) => l.slice(Number.isFinite(indent) ? indent : 0)), end }
  }
  const pushCode = (code: string, lang = '') => {
    const f = fence(code)
    out.push(f + lang, code, f, '')
  }
  const blank = () => {
    if (out.length && out[out.length - 1] !== '') out.push('')
  }
  const quote = (md: string, lead = '') => {
    const body = md.trim().split('\n')
    if (lead) out.push(`> ${lead}${body[0] ? ' ' + body.shift() : ''}`)
    for (const l of body) out.push(l ? `> ${l}` : '>')
    out.push('')
  }

  while (i < lines.length) {
    const line = lines[i]
    const t = line.trim()
    const next = lines[i + 1] ?? ''

    if (!t) {
      blank()
      i++
      continue
    }

    // A section title, underlined, and perhaps overlined, with the same character.
    const over = PUNCT.test(t) && indentOf(line) === 0 && lines[i + 2] !== undefined && lines[i + 1].trim() && PUNCT.test(lines[i + 2].trim()) && lines[i + 2].trim()[0] === t[0]
    const under = !PUNCT.test(t) && indentOf(line) === 0 && PUNCT.test(next.trim()) && indentOf(next) === 0 && next.trim().length >= Math.min(t.length, 3) && !/^\.\./.test(t)
    if (over || under) {
      const title = over ? lines[i + 1].trim() : t
      const style = (over ? 'o' : 'u') + (over ? t[0] : next.trim()[0])
      if (!levels.includes(style)) levels.push(style)
      out.push(`${'#'.repeat(Math.min(6, levels.indexOf(style) + 1))} ${inline(title)}`, '')
      i += over ? 3 : 2
      continue
    }
    // A transition.
    if (PUNCT.test(t) && t.length >= 4 && indentOf(line) === 0) {
      out.push('---', '')
      i++
      continue
    }

    // Explicit markup: directives, targets, substitutions, footnotes, comments.
    if (/^\.\.(\s|$)/.test(t) && indentOf(line) === 0) {
      const dir = /^\.\.\s+([\w:-]+)::\s*(.*)$/.exec(t)
      const sub = /^\.\.\s+\|[^|]+\|/.test(t)
      const target = /^\.\.\s+_/.test(t)
      const foot = /^\.\.\s+\[([^\]]+)\]\s*(.*)$/.exec(t)
      const { body, end } = indented(i + 1, 0)
      i = end
      if (sub || target) continue
      if (foot) {
        const label = foot[1] === '#' ? '' : foot[1].replace(/^#/, '')
        out.push(`${label ? `[${label}] ` : ''}${inline([foot[2], ...body].join(' ').trim())}`, '')
        continue
      }
      if (!dir) continue // a comment
      const name = dir[1].toLowerCase()
      const arg = dir[2].trim()
      // Options (:name: value) come first in the body.
      const options: Record<string, string> = {}
      let k = 0
      while (k < body.length && /^:[\w-]+:/.test(body[k])) {
        const m = /^:([\w-]+):\s*(.*)$/.exec(body[k])!
        options[m[1]] = m[2]
        k++
      }
      const content = body.slice(k).join('\n').replace(/^\n+/, '')
      if (CODE.has(name) && name !== 'highlight') {
        pushCode(content.trimEnd(), arg.split(/\s/)[0])
      } else if (name === 'math') {
        out.push('$$', [arg, content.trim()].filter(Boolean).join('\n'), '$$', '')
      } else if (name === 'image' || name === 'figure') {
        out.push(`![${options.alt ?? ''}](${arg})`, '')
        if (name === 'figure' && content.trim()) out.push(`*${inline(content.trim().split('\n\n')[0].replace(/\n/g, ' '))}*`, '')
      } else if (ADMONITIONS.has(name)) {
        const label = name === 'admonition' ? arg : (TITLES[name] ?? name[0].toUpperCase() + name.slice(1))
        const rest = name === 'admonition' ? content : [arg, content].filter(Boolean).join('\n\n')
        quote(rstToMarkdown(rest), `**${label}${TITLES[name] && arg ? ` ${arg}` : ''}:**`)
      } else if (name === 'include' || name === 'literalinclude') {
        out.push(`*(This includes ${arg}, which isn't shown here.)*`, '')
      } else if (name === 'csv-table' || name === 'list-table' || name === 'table') {
        if (arg) out.push(`**${inline(arg)}**`, '')
        if (content.trim()) pushCode(content.trimEnd())
      } else if (name === 'topic' || name === 'sidebar' || name === 'rubric') {
        out.push(`**${inline(arg)}**`, '')
        if (content.trim()) out.push(rstToMarkdown(content).trim(), '')
      } else if (name === 'code-block' || QUIET.has(name)) {
        // Nothing to show.
      } else {
        out.push(`*(A ${dir[1]} directive, which isn't shown here.)*`, '')
      }
      continue
    }

    // Tables keep their layout: a grid table (+---+) or a simple one (=== ===).
    if (/^\+[-=+]+\+$/.test(t) || (/^=+(\s+=+)+$/.test(t) && indentOf(line) === 0)) {
      let end = i
      while (end < lines.length && lines[end].trim()) end++
      pushCode(lines.slice(i, end).join('\n'))
      i = end
      continue
    }

    // A doctest block.
    if (t.startsWith('>>> ')) {
      let end = i
      while (end < lines.length && lines[end].trim()) end++
      pushCode(lines.slice(i, end).map((l) => l.slice(indentOf(line))).join('\n'), 'python')
      i = end
      continue
    }

    // A line block: each | line is a line of its own.
    if (/^\|(\s|$)/.test(t)) {
      while (i < lines.length && /^\|(\s|$)/.test(lines[i].trim())) out.push(inline(lines[i++].trim().replace(/^\|\s?/, '')) + '\\')
      out[out.length - 1] = out[out.length - 1].replace(/\\$/, '')
      out.push('')
      continue
    }

    // Lists: bullets, enumerations, and field lists.
    const bullet = /^(\s*)([-*+•])\s+(.*)$/.exec(line)
    const enumd = /^(\s*)(\d+|#|[a-zA-Z]|[ivxlcIVXLC]+)[.)]\s+(.*)$/.exec(line) ?? /^(\s*)\((\d+|#|[a-zA-Z])\)\s+(.*)$/.exec(line)
    const field = /^(\s*):([^:]+):\s*(.*)$/.exec(line)
    if (bullet || enumd || field) {
      const m = (bullet ?? enumd ?? field)!
      const lead = m[1].length
      const marker = bullet ? '- ' : enumd ? '1. ' : '- '
      const textStart = line.length - line.trimStart().length + line.trimStart().indexOf(m[3])
      const item = [m[3]]
      i++
      // The item's text runs on in lines indented to it; a deeper list starts its own item.
      while (i < lines.length && lines[i].trim() && indentOf(lines[i]) >= textStart && !/^\s*([-*+•]|\d+[.)]|#\.)\s/.test(lines[i])) item.push(lines[i++].trim())
      const depth = Math.round(lead / 2)
      const label = field ? `**${inline(field[2])}:** ` : ''
      out.push(`${'  '.repeat(depth)}${marker}${label}${inline(item.join(' '))}`)
      continue
    }

    // A definition list: a term, then its definition indented under it.
    if (next.trim() && indentOf(next) > indentOf(line) && !t.endsWith('::')) {
      const { body, end } = indented(i + 1, indentOf(line))
      const [term, ...classifiers] = t.split(/\s+:\s+/)
      out.push(`**${inline(term)}**${classifiers.length ? ` *(${classifiers.join(', ')})*` : ''}`, '')
      out.push(rstToMarkdown(body.join('\n')).trim(), '')
      i = end
      continue
    }

    // An indented block after a blank line, not after "::", is a quote.
    if (indentOf(line) > 0) {
      const { body, end } = indented(i, indentOf(line) - 1)
      quote(rstToMarkdown(body.join('\n')))
      i = end
      continue
    }

    // A paragraph. One ending "::" introduces a literal block, the indented block after it.
    const para: string[] = []
    while (i < lines.length && lines[i].trim() && indentOf(lines[i]) === 0) para.push(lines[i++].trimEnd())
    const last = para[para.length - 1]
    const literal = last.endsWith('::')
    if (literal) {
      if (last.trim() === '::') para.pop()
      else para[para.length - 1] = last.replace(/\s*::$/, last.endsWith(' ::') ? '' : ':')
    }
    if (para.length) out.push(...para.map((p) => guard(inline(p))), '')
    if (literal) {
      while (i < lines.length && !lines[i].trim()) i++
      if (i < lines.length && indentOf(lines[i]) > 0) {
        const { body, end } = indented(i, 0)
        pushCode(body.join('\n'))
        i = end
      }
    }
  }
  return out.join('\n').replace(/\n{3,}/g, '\n\n').trim() + '\n'
}
