import { describe, expect, it } from 'vitest'
import katex from 'katex'
import { mathMarked, renderMath, texMath } from './math'

const types = (md: string) => mathMarked.lexer(md).map((t) => t.type)
const inline = (md: string) => (mathMarked.lexer(md)[0] as { tokens?: { type: string; text?: string }[] }).tokens ?? []

describe('math tokens', () => {
  it('reads display math as a block', () => {
    const [t] = mathMarked.lexer('$$\nE = mc^2\n$$')
    expect(t).toMatchObject({ type: 'math', text: 'E = mc^2', display: true })
    expect(types('Before\n\n$$x^2$$\n\nAfter')).toEqual(['paragraph', 'space', 'math', 'paragraph'])
  })

  it('reads inline math', () => {
    expect(inline('Area is $\\pi r^2$ here.').map((t) => t.type)).toEqual(['text', 'math', 'text'])
    expect(inline('Area is $\\pi r^2$ here.')[1].text).toBe('\\pi r^2')
  })

  it('leaves prices and loose dollars as text', () => {
    for (const md of ['It costs $5 and $10.', 'A $ sign alone', 'Spaced $ x $ isn\'t math', 'Two\n$lines\n$ apart']) {
      expect(inline(md).some((t) => t.type === 'math'), md).toBe(false)
    }
  })
})

describe('texMath', () => {
  it('takes a file of bare math whole, without comments', () => {
    expect(texMath('% Euler\ne^{i\\pi} + 1 = 0\n')).toEqual(['e^{i\\pi} + 1 = 0'])
  })

  it('collects display math from a full document', () => {
    const doc = '\\documentclass{article}\\begin{document}\nText $a$.\n\\[ x^2 \\]\n\\begin{equation} y = 1 \\end{equation}\n\\begin{align*} a &= b \\\\ c &= d \\end{align*}\n\\end{document}'
    expect(texMath(doc)).toEqual(['x^2', 'y = 1', '\\begin{aligned} a &= b \\\\ c &= d \\end{aligned}'])
  })
})

describe('renderMath', () => {
  it('renders MathML and refuses links', () => {
    const out = renderMath(katex, String.raw`\frac{1}{2} \href{https://example.com}{x}`, false)
    expect(out).toContain('<math')
    expect(out).not.toContain('href="https://example.com"')
  })
})
