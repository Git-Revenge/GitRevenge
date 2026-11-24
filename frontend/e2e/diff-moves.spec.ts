// #947: moves and renames in full — a move that changed opens against where it came from,
// code wrapped in new structure says so, moves cycle their colours, a move's note goes to it,
// and a click on a rename lights up every place of it.
import { expect, test, type Page } from '@playwright/test'
import { open } from './support'

const oldLines = ['package tides', '', 'func helper() int {', '\treturn 1', '}', '', 'func run() {', '\tload()', '}']
const newLines = ['package tides', '', 'func run() {', '\tif ready() {', '\t\tload()', '\t}', '}', '', 'func helper() int {', '\treturn 2', '}']
const text = (lines: string[]) => lines.map((l) => l + '\n').join('')

const moved = {
  blocks: [
    { oldStart: 2, oldEnd: 6, newStart: 2, newEnd: 2 },
    { oldStart: 7, oldEnd: 8, newStart: 3, newEnd: 6 },
    { oldStart: 9, oldEnd: 9, newStart: 7, newEnd: 11 },
  ],
  moves: [
    // load() wrapped in an if where it was; helper() moved below and changed on the way.
    { oldStart: 7, oldEnd: 8, newStart: 4, newEnd: 5, reindented: true, into: 'if statement' },
    {
      oldStart: 2,
      oldEnd: 5,
      newStart: 8,
      newEnd: 11,
      reindented: false,
      changed: true,
      edits: [{ oldStart: 1, oldEnd: 2, newStart: 1, newEnd: 2, old: [{ line: 1, from: 8, to: 9 }], new: [{ line: 1, from: 8, to: 9 }] }],
    },
  ],
  renames: [],
  algorithm: 'myers',
  engine: 'code',
  language: 'Go',
  fallback: '',
  unparsed: false,
  added: 7,
  removed: 5,
}

const renLines = ['func f(a, b int) int {', '\treturn a + b + a', '}']
const renNew = ['func f(lo, hi int) int {', '\treturn lo + hi + lo', '}']
const span = (lines: string[], line: number, word: string, nth: number, rename: number) => {
  let from = -1
  for (let k = 0; k <= nth; k++) from = lines[line].indexOf(word, from + 1)
  return { line, from, to: from + word.length, kind: 'rename', rename }
}
const renamed = {
  blocks: [
    {
      oldStart: 0,
      oldEnd: 2,
      newStart: 0,
      newEnd: 2,
      kind: 'rename',
      note: 'renamed from a',
      old: [span(renLines, 0, 'a', 0, 1), span(renLines, 0, 'b', 0, 2), span(renLines, 1, 'a', 0, 1), span(renLines, 1, 'b', 0, 2), span(renLines, 1, 'a', 1, 1)],
      new: [span(renNew, 0, 'lo', 0, 1), span(renNew, 0, 'hi', 0, 2), span(renNew, 1, 'lo', 0, 1), span(renNew, 1, 'hi', 0, 2), span(renNew, 1, 'lo', 1, 1)],
    },
  ],
  moves: [],
  renames: [
    { from: 'a', to: 'lo', count: 3 },
    { from: 'b', to: 'hi', count: 2 },
  ],
  algorithm: 'myers',
  engine: 'code',
  language: 'Go',
  fallback: '',
  unparsed: false,
  added: 2,
  removed: 2,
}

const prefs = {
  theme: '',
  graphDensity: '',
  experimentalViews: [],
  updateChecks: true,
  gitPath: '',
  gitInUse: 'git',
  cloneDir: '',
  zoom: 100,
  forgeColors: false,
  diffAlgorithm: '',
  diffRibbon: true,
  diffMoves: true,
  diffCode: true,
  diffHideFormatting: false,
  languages: ['Go'],
}

async function openDiff(page: Page, layout: 'unified' | 'split', diff: unknown, before: string[], after: string[]) {
  await page.addInitScript((l) => localStorage.setItem('gitrevenge.diffLayout', l), layout)
  const side = (l: string[]) => ({ exists: true, text: text(l), size: text(l).length })
  await open(page, 'diff', { commandKeys: true, responses: { Preferences: prefs, FileDiff: { contents: { path: 'internal/tides/run.go', kind: 'text', old: side(before), new: side(after) }, diff } } })
}

test.describe('moves', () => {
  test('cycle their colours, and say what code moved into', async ({ page }) => {
    await openDiff(page, 'unified', moved, oldLines, newLines)
    const wrapped = page.locator('tr.moved.add', { hasText: 'load()' })
    await expect(wrapped).toHaveClass(/\bt0\b/)
    await expect(wrapped.locator('.move-note')).toContainText('moved into an if statement')
    await expect(page.locator('tr.moved.del', { hasText: 'load()' }).locator('.move-note')).toContainText('moved into an if statement at line 5')
    await expect(page.locator('tr.moved.add', { hasText: 'func helper' })).toHaveClass(/\bt1\b/)
  })

  test('a move that changed opens against where it came from', async ({ page }) => {
    await openDiff(page, 'unified', moved, oldLines, newLines)
    const note = page.locator('tr.moved.add', { hasText: 'func helper' }).locator('.move-note')
    await expect(note).toContainText('moved from line 3, changed')
    await note.getByRole('button', { name: 'Compare with the original' }).click()
    const dialog = page.getByRole('dialog', { name: 'Compare with the original' })
    await expect(dialog).toContainText('Lines 3–5 before are lines 9–11 now, changed in one place.')
    await expect(dialog.locator('tr.del .chg')).toHaveText('1')
    await expect(dialog.locator('tr.add .chg')).toHaveText('2')
    await page.keyboard.press('Escape')
    await expect(dialog).toHaveCount(0)
  })

  test("a move's note goes to the move", async ({ page }) => {
    await openDiff(page, 'unified', moved, oldLines, newLines)
    await page.locator('tr.moved.del', { hasText: 'func helper' }).getByRole('button', { name: 'moved to line 9, and changed' }).click()
    await expect(page.locator('tr.moved.add.current', { hasText: 'func helper' })).toHaveCount(1)
  })

  test('in the split view, bands take their move colour and a changed move opens', async ({ page }) => {
    await openDiff(page, 'split', moved, oldLines, newLines)
    await expect(page.locator('.band.move.t0')).toHaveCount(1)
    await expect(page.locator('.band.move.t1')).toHaveCount(1)
    await expect(page.locator('.cm-moved.t1').first()).toBeVisible()
    await page.locator('[data-side="new"] .cm-move-note', { hasText: 'changed' }).getByRole('button', { name: 'Compare with the original' }).click()
    await expect(page.getByRole('dialog', { name: 'Compare with the original' })).toBeVisible()
  })
})

test.describe('renames', () => {
  test('each has its colour, and a click lights up every place of it', async ({ page }) => {
    await openDiff(page, 'unified', renamed, renLines, renNew)
    const lo = page.locator('tr.add .chg.rename[data-rename="1"]')
    const hi = page.locator('tr.add .chg.rename[data-rename="2"]')
    await expect(lo).toHaveCount(3)
    await expect(lo.first()).toHaveClass(/\br3\b/)
    await expect(hi.first()).toHaveClass(/\br0\b/)
    await lo.first().click()
    await expect(page.locator('.chg.rename.hot')).toHaveCount(6)
    await expect(page.locator('.chg.rename.hot[data-rename="2"]')).toHaveCount(0)
    await lo.first().click()
    await expect(page.locator('.chg.rename.hot')).toHaveCount(0)
  })

  test('in the split view too, on both sides', async ({ page }) => {
    await openDiff(page, 'split', renamed, renLines, renNew)
    const hi = page.locator('[data-side="new"] .cm-word-rename[data-rename="2"]')
    await expect(hi).toHaveCount(2)
    await hi.first().click()
    await expect(page.locator('.cm-word-rename.cm-hot[data-rename="2"]')).toHaveCount(4)
  })
})
