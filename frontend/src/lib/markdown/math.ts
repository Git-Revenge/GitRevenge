import type Katex from 'katex'
import { Marked, type TokenizerExtension } from 'marked'

/** TeX as MathML. trust: false keeps \href, \url and HTML extensions out, so the markup is only
 * what KaTeX generates, and a parse error shows the source with the error, not a crash. */
export function renderMath(katex: typeof Katex, tex: string, display: boolean): string {
  return katex.renderToString(tex, { displayMode: display, output: 'mathml', throwOnError: false, trust: false, strict: 'ignore' })
}

/** A math token: TeX source, shown inline or as a display block. */
export interface MathToken {
  type: 'math'
  raw: string
  text: string
  display: boolean
}

/** $$…$$ on its own lines (or one line). */
const blockMath: TokenizerExtension = {
  name: 'math',
  level: 'block',
  start: (src) => src.indexOf('$$'),
  tokenizer(src) {
    const m = /^\$\$\s*([\s\S]+?)\s*\$\$[ \t]*(?:\n|$)/.exec(src)
    if (m) return { type: 'math', raw: m[0], text: m[1], display: true }
  },
}

/**
 * $…$ inline, by Pandoc's rule so prices don't turn into math: no space just inside the
 * dollars, no line break, and no digit right after the closing one ("$5 and $10" stays text).
 */
const inlineMath: TokenizerExtension = {
  name: 'math',
  level: 'inline',
  start: (src) => src.indexOf('$'),
  tokenizer(src) {
    const m = /^\$(?!\s)((?:\\\$|[^$\n])+?)(?<!\s)\$(?!\d)/.exec(src)
    if (m) return { type: 'math', raw: m[0], text: m[1], display: false }
  },
}

/** A marked instance that also reads $…$ and $$…$$ math. */
export const mathMarked = new Marked({ gfm: true, extensions: [blockMath, inlineMath] })

/**
 * The math in a .tex file: a full document (\documentclass) gives its display math (\[…\],
 * $$…$$ and equation, align and gather environments); a file of bare math is one block.
 */
export function texMath(source: string): string[] {
  if (!/\\documentclass|\\begin\{document\}/.test(source)) {
    const body = source.replace(/^\s*%.*$/gm, '').trim()
    return body ? [body] : []
  }
  const out: string[] = []
  const re = /\\\[([\s\S]+?)\\\]|\$\$([\s\S]+?)\$\$|\\begin\{(equation|align|gather|multline)(\*?)\}([\s\S]+?)\\end\{\3\4\}/g
  for (const m of source.matchAll(re)) {
    if (m[1] ?? m[2]) out.push((m[1] ?? m[2]).trim())
    // align and gather keep their environment so the line breaks and alignment work.
    else out.push(m[3] === 'equation' ? m[5].trim() : `\\begin{${m[3]}ed}${m[5]}\\end{${m[3]}ed}`)
  }
  return out
}
