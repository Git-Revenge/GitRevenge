import { expect, test, type Page } from '@playwright/test'
import { open } from './support'

// The backend sends the lanes that run straight through a row packed (graph.Page in Go), not as
// a line in each half of the row. These draw one graph both ways and want the same picture.

interface Seg {
  from: number
  to: number
  color: number
  kind?: string
}
interface Row {
  index: number
  col: number
  color: number
  up: Seg[]
  down: Seg[]
  pass?: number[]
  passKind?: string[]
  width: number
  kind?: string
  id: string
  parents: number
  author: string
  authorTime: number
  subject: string
}

/** What graph.Page sends, worked out from a row that has every line: straight lanes of one colour and kind in both halves become runs. */
function pack(row: Row): Row {
  const down = [...row.down]
  const up: Seg[] = []
  const pass: number[] = []
  const passKind: string[] = []
  let last = -2
  let run = -1
  for (const s of row.up) {
    const j = s.from === s.to ? down.findIndex((d) => d.from === s.from && d.to === s.from) : -1
    if (j < 0 || down[j].color !== s.color || (down[j].kind ?? '') !== (s.kind ?? '')) {
      up.push(s)
      continue
    }
    down.splice(j, 1)
    if (run < 0 || s.from !== last + 1 || passKind[passKind.length - 1] !== (s.kind ?? '')) {
      run = pass.length + 1
      pass.push(s.from, 0)
      passKind.push(s.kind ?? '')
    }
    pass[run]++
    pass.push(s.color)
    last = s.from
  }
  return { ...row, up, down, pass, passKind }
}

const LANES = 40
const kindOf = (lane: number) => (lane === 0 ? 'trunk' : lane === 2 ? 'docs' : lane === 7 ? 'bot' : undefined)
const seg = (from: number, to: number, color = from): Seg => ({ from, to, color, ...(kindOf(color) ? { kind: kindOf(color) } : {}) })

/** Forty lines of history side by side, more than the graph column fits. Each row's commit is on
 * one of them and the rest pass through; row 5 merges lane 9 into lane 0, and lane 12 ends at
 * row 8 and starts again in another colour at row 10. */
function wide(): Row[] {
  const nodes = [0, 3, 1, 2, 5, 0, 30, 7, 12, 4, 12, 0, 39, 6]
  return nodes.map((col, i) => {
    const lanes = Array.from({ length: LANES }, (_, lane) => lane).filter((lane) => lane !== 12 || i <= 8 || i >= 10)
    const colorOf = (lane: number) => (lane === 12 && i >= 10 ? 50 : lane)
    const up = i === 0 ? [] : lanes.filter((lane) => lane !== 12 || i !== 10).map((lane) => seg(lane, lane, colorOf(lane)))
    const down = lanes.filter((lane) => lane !== 12 || i !== 8).map((lane) => seg(lane, lane, colorOf(lane)))
    if (i === 5) down.push({ from: 0, to: 9, color: 9 })
    return { index: i, col, color: colorOf(col), up, down, width: LANES, kind: kindOf(col), id: String(i).padStart(40, '0'), parents: i === 5 ? 2 : 1, author: 'Ana Cruz', authorTime: 1793500000 - i * 60, subject: `Commit ${i}` }
  })
}

async function picture(page: Page, graphData: Row[], after?: (page: Page) => Promise<void>) {
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await open(page, 'graph', { graphData, responses: { Refs: [], CommitSignatures: [] } })
  await expect(page.locator('.scroller')).toHaveAttribute('aria-rowcount', String(graphData.length))
  await after?.(page)
  // The rows arrive and the graph column settles to their width over a few frames. After that,
  // until two looks a moment apart agree, something is still being drawn.
  await page.waitForTimeout(500)
  // A digest, so a picture that differs doesn't print itself into the report.
  const shot = () =>
    page
      .locator('canvas')
      .first()
      .evaluate(async (c) => {
        const el = c as HTMLCanvasElement
        const pixels = el.getContext('2d')!.getImageData(0, 0, el.width, el.height).data
        const digest = await crypto.subtle.digest('SHA-256', pixels)
        return `${el.width}x${el.height} ${Array.from(new Uint8Array(digest), (b) => b.toString(16).padStart(2, '0')).join('')}`
      })
  let before = ''
  await expect
    .poll(async () => {
      const now = await shot()
      const settled = now === before
      before = now
      return settled
    })
    .toBe(true)
  return before
}

test.describe('lanes that pass straight through a row', () => {
  test('are packed out of the two halves, most of a wide row', () => {
    const rows = wide().map(pack)
    const sent = (r: Row) => r.up.length + r.down.length
    expect(Math.max(...rows.slice(1).map(sent))).toBeLessThanOrEqual(4)
    expect(rows[3].pass!.length).toBeGreaterThan(LANES)
    expect(new Set(rows[3].passKind)).toEqual(new Set(['', 'trunk', 'docs', 'bot']))
  })

  test('draw the same packed as they do as lines', async ({ page }) => {
    const plain = await picture(page, wide())
    const packed = await picture(page, wide().map(pack))
    expect(packed).toBe(plain)
  })

  test('draw the same with a commit far to the right selected, which moves the lanes in view', async ({ page }) => {
    const select = async (p: Page) => {
      const box = (await p.locator('.scroller').boundingBox())!
      // Row 12's commit is on lane 39.
      await p.mouse.click(box.x + 500, box.y + 26 + 52 + 11 * 30 + 15)
      await p.mouse.move(0, 0)
    }
    const plain = await picture(page, wide(), select)
    const packed = await picture(page, wide().map(pack), select)
    expect(packed).toBe(plain)
  })

  test('are drawn at all: a packed row isn’t the row without them', async ({ page }) => {
    const packed = await picture(page, wide().map(pack))
    const without = await picture(page, wide().map((r) => ({ ...pack(r), pass: [], passKind: [] })))
    expect(packed).not.toBe(without)
  })
})
