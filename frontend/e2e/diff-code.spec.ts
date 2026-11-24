// #945: diffs read as code — formatting and renames told apart from changes, Hide formatting,
// Code and Lines, lines first while a big file is read, and staging that shows everything.
import { expect, test, type Page } from '@playwright/test'
import { calls, open, type Scenario } from './support'

const oldLines = ['package tides', '', 'const maxHours = 0x30', '', 'func forecast(n int) []Reading {', '\tout := make([]Reading, 0, n)', '\treturn out', '}']
const newLines = ['package tides', '', 'const maxHours = 48', '', 'func forecast(n int, step int) []Reading {', '\treadings := make([]Reading, 0, n)', '\treturn readings', '}']
const text = (lines: string[]) => lines.map((l) => l + '\n').join('')

/** A span over the first `word` in a line, of a kind. */
const at = (lines: string[], line: number, word: string, kind?: string) => {
  const from = lines[line].indexOf(word)
  return { line, from, to: from + word.length, ...(kind ? { kind } : {}) }
}

const code = {
  blocks: [
    { oldStart: 2, oldEnd: 3, newStart: 2, newEnd: 3, kind: 'format', note: 'same value', old: [at(oldLines, 2, '0x30', 'format')], new: [at(newLines, 2, '48', 'format')] },
    { oldStart: 4, oldEnd: 5, newStart: 4, newEnd: 5, new: [at(newLines, 4, ', step int')] },
    {
      oldStart: 5,
      oldEnd: 7,
      newStart: 5,
      newEnd: 7,
      kind: 'rename',
      note: 'renamed from out',
      old: [at(oldLines, 5, 'out', 'rename'), at(oldLines, 6, 'out', 'rename')],
      new: [at(newLines, 5, 'readings', 'rename'), at(newLines, 6, 'readings', 'rename')],
    },
  ],
  moves: [],
  renames: [{ from: 'out', to: 'readings', count: 2 }],
  algorithm: 'histogram',
  engine: 'code',
  language: 'Go',
  fallback: '',
  unparsed: false,
  added: 4,
  removed: 4,
}
const lines = { ...code, blocks: [{ oldStart: 2, oldEnd: 3, newStart: 2, newEnd: 3 }, { oldStart: 4, oldEnd: 7, newStart: 4, newEnd: 7 }], renames: [], engine: 'lines' }
const slow = { ...lines, fallback: 'time' }

const side = (l: string[]) => ({ exists: true, text: text(l), size: text(l).length })
const contents = { path: 'internal/tides/forecast.go', kind: 'text', old: side(oldLines), new: side(newLines) }
const answer = (diff: unknown) => ({ contents, diff })
const prefs = (extra: Record<string, unknown> = {}) => ({
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
  ...extra,
})

/** Opens the diff with FileDiff answering by how it's asked to read. */
async function openDiff(page: Page, layout: 'unified' | 'split', reads: Record<string, unknown>, scenario: Scenario = {}, extra: Record<string, unknown> = {}) {
  await page.addInitScript((l) => localStorage.setItem('gitrevenge.diffLayout', l), layout)
  await open(page, 'diff', {
    commandKeys: true,
    ...scenario,
    responses: { Preferences: prefs(extra), ...scenario.responses },
    byArg: { FileDiff: { index: 6, answers: Object.fromEntries(Object.entries(reads).map(([k, v]) => [k, answer(v)])) } },
  })
}

const reads = (page: Page) => calls(page, 'FileDiff').then((c) => c.map((args) => args[6]))

test.describe('read as code', () => {
  test('says what changed: changes, renames and formatting', async ({ page }) => {
    await openDiff(page, 'unified', { code, lines })
    await expect(page.locator('.engine.code')).toHaveText('Read as Go')
    const summary = page.getByRole('group', { name: 'What changed' })
    await expect(summary).toContainText('1 change')
    await expect(summary).toContainText('rename: out → readings, 2 places')
    await expect(summary).toContainText('1 formatting change')
    await expect(page.locator('tr.format .chg.format').first()).toHaveText('0x30')
    await expect(page.locator('tr.add.format .block-note')).toHaveText('same value')
    await expect(page.locator('tr.add.rename .chg.rename').first()).toHaveText('readings')
    await expect(page.locator('tr.add.rename .block-note')).toHaveText('renamed from out')
    // Highlighting can split the mark into several pieces.
    await expect.poll(() => page.locator('tr.add:not(.format):not(.rename) .chg').allTextContents().then((t) => t.join(''))).toBe(', step int')
  })

  test('hides formatting, and says it did', async ({ page }) => {
    await openDiff(page, 'unified', { code, lines })
    const summary = page.getByRole('group', { name: 'What changed' })
    await expect(summary.locator('.where')).toHaveText('3 places')
    await page.getByRole('switch', { name: 'Hide formatting' }).click()
    await expect(page.locator('tr.format')).toHaveCount(0)
    // The line reads as unchanged, in its new form.
    await expect(page.locator('tr.eq', { hasText: 'const maxHours = 48' })).toHaveCount(1)
    await expect(summary).toContainText('1 formatting change hidden')
    await expect(summary.locator('.where')).toHaveText('2 places')
    await summary.getByRole('button', { name: 'Show it' }).click()
    await expect(page.locator('tr.format')).toHaveCount(2)
  })

  test('starts hidden when Settings says so', async ({ page }) => {
    await openDiff(page, 'unified', { code, lines }, {}, { diffHideFormatting: true })
    await expect(page.getByRole('switch', { name: 'Hide formatting' })).toHaveAttribute('aria-checked', 'true')
    await expect(page.locator('tr.format')).toHaveCount(0)
  })

  test('Lines compares whole lines, as git does', async ({ page }) => {
    await openDiff(page, 'unified', { code, lines })
    await expect(page.locator('.engine.code')).toBeVisible()
    await page.getByRole('group', { name: 'Compare by' }).getByRole('button', { name: 'Lines' }).click()
    await expect(page.locator('.engine.lines')).toHaveText('Lines')
    await expect(page.getByRole('switch', { name: 'Hide formatting' })).toBeDisabled()
    await expect(page.getByRole('group', { name: 'What changed' })).toContainText('4 lines added')
    expect(await reads(page)).toEqual(['code', 'lines'])
  })

  test('the split view draws formatting and renames in the ribbon', async ({ page }) => {
    await openDiff(page, 'split', { code, lines })
    const ribbon = page.locator('svg.gap')
    await expect(ribbon.locator('g.band.format')).toHaveCount(1)
    await expect(ribbon.locator('g.band.rename')).toHaveCount(1)
    await expect(ribbon.locator('g.band.change')).toHaveCount(1)
    await expect(page.locator('[data-side="new"] .cm-word-rename').first()).toHaveText('readings')
    await expect(page.locator('[data-side="new"] .cm-block-note').first()).toHaveText('same value')
    await page.getByRole('switch', { name: 'Hide formatting' }).click()
    await expect(ribbon.locator('g.band.format')).toHaveCount(0)
    await expect(page.locator('[data-side="new"] .cm-format')).toHaveCount(0)
  })
})

test.describe('a file that takes long to read', () => {
  test('shows lines first, then the code when reading is done', async ({ page }) => {
    await openDiff(page, 'unified', { code: slow, patient: code, lines }, { delays: { FileDiff: 1500 } })
    await expect(page.locator('.engine.reading')).toHaveText('Reading as Go')
    await expect(page.locator('.fallback')).toHaveCount(0)
    await expect(page.locator('.engine.code')).toHaveText('Read as Go')
    await expect(page.locator('tr.rename').first()).toBeVisible()
    expect(await reads(page)).toEqual(['code', 'patient'])
  })

  test('says why when it can’t be read in time, and reads it anyway when asked', async ({ page }) => {
    await openDiff(page, 'unified', { code: slow, patient: slow, anyway: code, lines })
    const note = page.locator('.fallback')
    await expect(note).toContainText('Shown as lines: reading it as Go took longer than 3 seconds.')
    await expect(page.locator('.engine.lines')).toHaveText('Lines')
    await note.getByRole('button', { name: 'Read it as code anyway' }).click()
    await expect(page.locator('.engine.code')).toHaveText('Read as Go')
    await expect(note).toHaveCount(0)
    expect(await reads(page)).toEqual(['code', 'patient', 'anyway'])
  })

  test('says why a file falls back for other reasons', async ({ page }) => {
    await openDiff(page, 'unified', { code: { ...lines, fallback: 'errors' }, lines })
    await expect(page.locator('.fallback')).toContainText('Shown as lines: more than a quarter of it doesn’t parse as Go.')
    await expect(page.locator('.fallback').getByRole('button')).toHaveCount(0)
  })
})

test.describe('staging', () => {
  test('shows every change, and formatting stages on its own', async ({ page }) => {
    await openDiff(page, 'unified', { code, lines }, { diffTarget: { mode: 'unstaged', file: 'internal/tides/forecast.go' } }, { diffHideFormatting: true })
    const hide = page.getByRole('switch', { name: 'Hide formatting' })
    await expect(hide).toBeDisabled()
    await expect(hide).toHaveAttribute('aria-checked', 'false')
    await expect(page.locator('tr.format')).toHaveCount(2)
    const formatting = page.locator('section.hunk', { has: page.locator('.tag', { hasText: 'formatting only' }) })
    await expect(formatting).toHaveCount(1)
    await expect(formatting.getByRole('button', { name: 'Stage hunk' })).toBeVisible()
    await expect(page.locator('section.hunk')).toHaveCount(2)
  })
})

test.describe('file lists', () => {
  const sha = '0'.repeat(40)
  const ana = { name: 'Ana Cruz', email: 'ana@example.com', time: 1793500000 }
  const commit = { id: sha, subject: 'Commit 0', body: '', trailers: [], parents: ['a'.repeat(40)], author: ana, committer: ana }
  const file = (path: string) => ({ path, status: 'M', added: 1, deleted: 1, binary: false })
  const repo = (extra: Record<string, unknown>) => ({
    graphRows: 3,
    responses: {
      Refs: [],
      CommitSignatures: [],
      CommitNotes: [],
      WorkingTree: { status: { files: [] }, signing: { enabled: false }, identity: { name: 'A', email: 'a@example.com' } },
      CommitDetail: { commit, files: [file('internal/tides/forecast.go'), file('internal/tides/stations.json')] },
      CommitRow: 1,
      FormattingOnly: ['internal/tides/stations.json'],
      ...extra,
    },
  })

  const panelFile = (page: Page, name: string) => page.locator('.files .file-row', { hasText: name })

  async function selectCommit(page: Page) {
    const box = (await page.locator('.scroller').boundingBox())!
    await page.mouse.click(box.x + 500, box.y + 52 + 26)
    await expect(page.getByRole('heading', { name: 'Commit 0' })).toBeVisible()
    await page.getByRole('tab', { name: 'Files', exact: true }).click()
  }

  test('a commit marks the files that only change formatting', async ({ page }) => {
    await open(page, 'repoView', repo({ Preferences: prefs() }))
    await selectCommit(page)
    await expect(panelFile(page, 'stations.json')).toBeVisible()
    await expect(panelFile(page, 'stations.json').locator('.fmt-only')).toHaveText('formatting only')
    await expect(panelFile(page, 'forecast.go').locator('.fmt-only')).toHaveCount(0)
    const asked = await calls(page, 'FormattingOnly')
    expect(asked[0].slice(1)).toEqual(['commit', sha, ['internal/tides/forecast.go', 'internal/tides/stations.json']])
  })
})
