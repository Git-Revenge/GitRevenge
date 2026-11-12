import { describe, expect, it } from 'vitest'
import { attachmentSrc, joined, pickOutput, readNotebook, stripAnsi } from './notebook'

const nb = (cells: unknown[], metadata: unknown = { language_info: { name: 'python' } }) => JSON.stringify({ nbformat: 4, nbformat_minor: 5, metadata, cells })

describe('readNotebook', () => {
  it('reads markdown and code cells, with their counts and the language', () => {
    const book = readNotebook(
      nb([
        { cell_type: 'markdown', source: ['# Title\n', 'Some *words*.'], metadata: {} },
        { cell_type: 'code', source: 'x = 1\nx', execution_count: 3, outputs: [{ output_type: 'execute_result', data: { 'text/plain': ['1'] }, execution_count: 3 }] },
        { cell_type: 'code', source: '', execution_count: null, outputs: [] },
        { cell_type: 'raw', source: 'raw text' },
      ]),
    )
    expect(book.language).toBe('python')
    expect(book.cells).toEqual([
      { kind: 'markdown', text: '# Title\nSome *words*.', attachments: {} },
      { kind: 'code', source: 'x = 1\nx', count: 3, outputs: [{ kind: 'text', text: '1' }] },
      { kind: 'code', source: '', count: null, outputs: [] },
      { kind: 'raw', text: 'raw text' },
    ])
  })

  it('takes the language from the kernel when the notebook doesn’t say it otherwise', () => {
    expect(readNotebook(nb([], { kernelspec: { language: 'Julia' } })).language).toBe('julia')
  })

  it('reads streams, keeping stderr apart, and errors without their colour codes', () => {
    const book = readNotebook(
      nb([
        {
          cell_type: 'code',
          source: 'go()',
          execution_count: 1,
          outputs: [
            { output_type: 'stream', name: 'stdout', text: ['hello\n'] },
            { output_type: 'stream', name: 'stderr', text: 'careful\n' },
            { output_type: 'error', ename: 'ValueError', evalue: 'bad value', traceback: ['\u001b[0;31mValueError\u001b[0m: bad value'] },
          ],
        },
      ]),
    )
    const cell = book.cells[0]
    expect(cell.kind === 'code' && cell.outputs).toEqual([
      { kind: 'text', text: 'hello\n', stream: 'stdout' },
      { kind: 'text', text: 'careful\n', stream: 'stderr' },
      { kind: 'error', name: 'ValueError', value: 'bad value', trace: 'ValueError: bad value' },
    ])
  })

  it('reads the older format 3, with its worksheets, prompts and heading cells', () => {
    const v3 = JSON.stringify({
      nbformat: 3,
      metadata: {},
      worksheets: [
        {
          cells: [
            { cell_type: 'heading', level: 2, source: ['Results'] },
            { cell_type: 'code', input: ['print(2)'], prompt_number: 7, outputs: [{ output_type: 'stream', stream: 'stdout', text: ['2\n'] }, { output_type: 'pyout', png: 'iVBOR', text: ['<Figure>'] }] },
          ],
        },
      ],
    })
    const book = readNotebook(v3)
    expect(book.format).toBe(3)
    expect(book.language).toBe('python')
    expect(book.cells[0]).toEqual({ kind: 'markdown', text: '## Results', attachments: {} })
    expect(book.cells[1]).toEqual({ kind: 'code', source: 'print(2)', count: 7, outputs: [{ kind: 'text', text: '2\n', stream: 'stdout' }, { kind: 'image', src: 'data:image/png;base64,iVBOR', alt: 'Output' }] })
  })

  it('says so when the text isn’t a notebook', () => {
    expect(() => readNotebook('{nope')).toThrow("This isn't valid JSON")
    expect(() => readNotebook('[1, 2]')).toThrow("doesn't look like a Jupyter notebook")
    expect(() => readNotebook('{"nbformat": 4}')).toThrow('it has no cells')
  })
})

describe('pickOutput', () => {
  it('shows a picture before anything else, without its line breaks', () => {
    expect(pickOutput({ 'image/png': 'iVBO\nRw0=\n', 'text/plain': '<Figure>' })).toEqual({ kind: 'image', src: 'data:image/png;base64,iVBORw0=', alt: 'Output' })
  })

  it('puts an SVG in a picture, where it can’t run anything', () => {
    const out = pickOutput({ 'image/svg+xml': ['<svg xmlns="http://www.w3.org/2000/svg"/>'] })
    expect(out.kind).toBe('image')
    expect(out.kind === 'image' && atob(out.src.split(',')[1])).toBe('<svg xmlns="http://www.w3.org/2000/svg"/>')
  })

  it('prefers Markdown to HTML, and keeps the plain text of HTML to fall back on', () => {
    expect(pickOutput({ 'text/markdown': '**hi**', 'text/html': '<b>hi</b>' })).toEqual({ kind: 'markdown', text: '**hi**' })
    expect(pickOutput({ 'text/html': '<table/>', 'text/plain': '   a\n0  1' })).toEqual({ kind: 'html', html: '<table/>', fallback: '   a\n0  1' })
  })

  it('shows LaTeX as math', () => {
    expect(pickOutput({ 'text/latex': '\\frac{1}{2}' })).toEqual({ kind: 'markdown', text: '$$\\frac{1}{2}$$' })
    expect(pickOutput({ 'text/latex': '$x^2$' })).toEqual({ kind: 'markdown', text: '$x^2$' })
  })

  it('names what it won’t run', () => {
    expect(pickOutput({ 'application/vnd.jupyter.widget-view+json': {} })).toEqual({ kind: 'hidden', what: 'An interactive output, which only runs in Jupyter' })
    expect(pickOutput({ 'application/javascript': 'alert(1)' }).kind).toBe('hidden')
    expect(pickOutput({})).toEqual({ kind: 'hidden', what: 'An empty output' })
  })
})

describe('the small helpers', () => {
  it('joins a notebook’s lists of lines', () => {
    expect(joined(['a\n', 'b'])).toBe('a\nb')
    expect(joined('ab')).toBe('ab')
    expect(joined(undefined)).toBe('')
  })

  it('takes colour codes out of terminal text', () => {
    expect(stripAnsi('\u001b[1;32mok\u001b[0m done')).toBe('ok done')
  })

  it('finds a Markdown cell’s attached picture', () => {
    const book = readNotebook(nb([{ cell_type: 'markdown', source: '![p](attachment:plot%201.png)', attachments: { 'plot 1.png': { 'image/png': 'iVBOR' } } }]))
    const cell = book.cells[0]
    expect(cell.kind === 'markdown' && attachmentSrc('attachment:plot%201.png', cell.attachments)).toBe('data:image/png;base64,iVBOR')
    expect(attachmentSrc('img/x.png', {})).toBe('')
  })
})
