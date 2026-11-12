/**
 * A Jupyter notebook, read for showing: its cells and the outputs stored with them. Nothing in
 * it is ever run. Outputs come as the notebook stored them, and each is shown in the safest
 * form it has: a picture, Markdown, a table pulled out of HTML, or plain text. Scripts and
 * widgets are named, never run.
 */

/** One output of a code cell, ready to show. */
export type Output =
  | { kind: 'text'; text: string; stream?: 'stdout' | 'stderr' }
  | { kind: 'image'; src: string; alt: string }
  | { kind: 'markdown'; text: string }
  | { kind: 'html'; html: string; fallback: string }
  | { kind: 'error'; name: string; value: string; trace: string }
  | { kind: 'hidden'; what: string }

export type Cell =
  | { kind: 'markdown'; text: string; attachments: Record<string, string> }
  | { kind: 'code'; source: string; count: number | null; outputs: Output[] }
  | { kind: 'raw'; text: string }

export interface Notebook {
  /** The language the code cells are in, as the notebook says: "python", "julia", "r". */
  language: string
  cells: Cell[]
  /** The notebook format's major version. */
  format: number
}

type Json = Record<string, unknown>
const isObj = (v: unknown): v is Json => !!v && typeof v === 'object' && !Array.isArray(v)

/** A notebook's multi-line string: a string, or a list of lines to join. */
export function joined(v: unknown): string {
  if (typeof v === 'string') return v
  if (Array.isArray(v)) return v.map((s) => (typeof s === 'string' ? s : '')).join('')
  return ''
}

/** Terminal colour codes, which tracebacks are full of, taken out. */
export function stripAnsi(text: string): string {
  // eslint-disable-next-line no-control-regex -- escape sequences are exactly what's matched
  return text.replace(/\u001b\[[0-9;?]*[ -/]*[@-~]/g, '').replace(/\u001b[@-_]/g, '')
}

const PICTURES = ['image/png', 'image/jpeg', 'image/gif', 'image/webp']

/** The best way to show one display_data or execute_result bundle, by what's in it. */
export function pickOutput(data: Json, alt = 'Output'): Output {
  for (const mime of PICTURES) {
    const v = joined(data[mime]).replace(/\s+/g, '')
    if (v) return { kind: 'image', src: `data:${mime};base64,${v}`, alt }
  }
  // An SVG goes in an <img>, where it can't run anything.
  const svg = joined(data['image/svg+xml'])
  if (svg) return { kind: 'image', src: `data:image/svg+xml;base64,${utf8ToBase64(svg)}`, alt }
  const md = joined(data['text/markdown'])
  if (md) return { kind: 'markdown', text: md }
  const plain = joined(data['text/plain'])
  const html = joined(data['text/html'])
  if (html) return { kind: 'html', html, fallback: plain }
  const latex = joined(data['text/latex'])
  if (latex) return { kind: 'markdown', text: latex.includes('$') ? latex : `$$${latex}$$` }
  if (plain) return { kind: 'text', text: plain }
  const kinds = Object.keys(data)
  if (kinds.some((k) => k.includes('javascript') || k.includes('widget'))) return { kind: 'hidden', what: 'An interactive output, which only runs in Jupyter' }
  return { kind: 'hidden', what: kinds.length ? `An output in ${kinds.join(', ')}` : 'An empty output' }
}

function utf8ToBase64(text: string): string {
  const bytes = new TextEncoder().encode(text)
  let bin = ''
  for (const b of bytes) bin += String.fromCharCode(b)
  return btoa(bin)
}

function outputOf(o: Json, v3: boolean): Output | null {
  const type = o.output_type
  if (type === 'stream') return { kind: 'text', text: stripAnsi(joined(o.text)), stream: o.name === 'stderr' || o.stream === 'stderr' ? 'stderr' : 'stdout' }
  if (type === 'error' || type === 'pyerr') {
    const trace = Array.isArray(o.traceback) ? o.traceback.map((l) => stripAnsi(String(l))).join('\n') : ''
    return { kind: 'error', name: String(o.ename ?? 'Error'), value: String(o.evalue ?? ''), trace }
  }
  if (type === 'execute_result' || type === 'display_data' || type === 'pyout') {
    if (!v3) return isObj(o.data) ? pickOutput(o.data) : null
    // Format 3 kept each kind under its own short key.
    const data: Json = {}
    const keys: Record<string, string> = { png: 'image/png', jpeg: 'image/jpeg', svg: 'image/svg+xml', html: 'text/html', markdown: 'text/markdown', latex: 'text/latex', text: 'text/plain', javascript: 'application/javascript' }
    for (const [k, mime] of Object.entries(keys)) if (o[k] !== undefined) data[mime] = o[k]
    return pickOutput(data)
  }
  return null
}

function attachmentsOf(v: unknown): Record<string, string> {
  const out: Record<string, string> = {}
  if (!isObj(v)) return out
  for (const [name, bundle] of Object.entries(v)) {
    if (!isObj(bundle)) continue
    for (const mime of [...PICTURES, 'image/svg+xml']) {
      const data = joined(bundle[mime]).replace(/\s+/g, '')
      if (data) {
        out[name] = mime === 'image/svg+xml' && !/^[A-Za-z0-9+/=]+$/.test(data) ? `data:${mime};base64,${utf8ToBase64(joined(bundle[mime]))}` : `data:${mime};base64,${data}`
        break
      }
    }
  }
  return out
}

/** Reads a notebook's JSON. It throws, with a message to show, when the text isn't one. */
export function readNotebook(text: string): Notebook {
  let doc: unknown
  try {
    doc = JSON.parse(text)
  } catch {
    throw new Error("This isn't valid JSON, so it can't be read as a notebook.")
  }
  if (!isObj(doc)) throw new Error("This doesn't look like a Jupyter notebook.")
  const format = typeof doc.nbformat === 'number' ? doc.nbformat : 4
  const meta = isObj(doc.metadata) ? doc.metadata : {}
  const info = isObj(meta.language_info) ? meta.language_info : {}
  const kernel = isObj(meta.kernelspec) ? meta.kernelspec : {}
  const language = String(info.name ?? kernel.language ?? (format < 4 ? 'python' : '')).toLowerCase()
  const v3 = format < 4
  const raw: unknown[] = v3 ? (Array.isArray(doc.worksheets) && isObj(doc.worksheets[0]) && Array.isArray(doc.worksheets[0].cells) ? doc.worksheets[0].cells : []) : Array.isArray(doc.cells) ? doc.cells : []
  if (!v3 && !Array.isArray(doc.cells)) throw new Error("This doesn't look like a Jupyter notebook: it has no cells.")
  const cells: Cell[] = []
  for (const c of raw) {
    if (!isObj(c)) continue
    const type = c.cell_type
    if (type === 'markdown' || type === 'heading') {
      // Format 3 had heading cells of their own, with a level.
      const text = type === 'heading' ? `${'#'.repeat(Math.min(6, Number(c.level) || 1))} ${joined(c.source)}` : joined(c.source)
      cells.push({ kind: 'markdown', text, attachments: attachmentsOf(c.attachments) })
    } else if (type === 'code') {
      const outputs = (Array.isArray(c.outputs) ? c.outputs : []).map((o) => (isObj(o) ? outputOf(o, v3) : null)).filter((o): o is Output => o !== null)
      const count = v3 ? c.prompt_number : c.execution_count
      cells.push({ kind: 'code', source: joined(v3 ? c.input : c.source), count: typeof count === 'number' ? count : null, outputs })
    } else if (type === 'raw') cells.push({ kind: 'raw', text: joined(c.source) })
  }
  return { language, cells, format }
}

/** A Markdown cell's attachment link ("attachment:plot.png") to its picture, if it has one. */
export function attachmentSrc(src: string, attachments: Record<string, string>): string {
  const m = /^attachment:(.+)$/.exec(src)
  if (!m) return ''
  let name = m[1]
  try {
    name = decodeURIComponent(name)
  } catch {
    // Keep it as written.
  }
  return attachments[name] ?? ''
}
