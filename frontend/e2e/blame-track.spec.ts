import { expect, test, type Page } from '@playwright/test'
import { calls, events, open } from './support'

// The blame view's track through a file's versions (#1078). The backend is stubbed: the
// versions come from BlameTrack, each older version's text and change from FileDiff.

const A = 'a'.repeat(40)
const B = 'b'.repeat(40)
const C = 'c'.repeat(40)
const T = 1756000000
const version = (id: string, subject: string, author: string, time: number, added: number, removed = 0, binary = false) => ({ id, author, time, subject, added, removed, binary })
const versions = [version(A, 'Add the alerts', 'Ana Cruz', T, 1), version(B, 'Say two', 'Sam Ortiz', T + 86400, 1), version(C, 'Say three', 'Ana Cruz', T + 2 * 86400, 1)]
const blame = (commit: string, line: string) => ({ commit, author: commit === B ? 'Sam Ortiz' : 'Ana Cruz', time: T, summary: 'S', line, origLine: 1 })
const step = (text: string, blocks: object[], kind = 'text') => ({ contents: { path: 'a.txt', old: { exists: true, text: '', size: 0 }, new: { exists: true, text, size: text.length }, kind }, diff: kind === 'text' ? { blocks, moves: [], renames: [], algorithm: 'myers', engine: 'lines', language: '', fallback: '', unparsed: false, added: 1, removed: 0 } : null })

const scenario = (track: object, answers: Record<string, unknown> = {}) => ({
  responses: {
    BlameNeeds: { partial: false, versions: 0, missing: [] },
    Blame: [blame(A, 'one'), blame(B, 'two'), blame(C, 'three')],
    BlameTrack: { versions, renamedFrom: '', workTree: false, ...track },
  },
  byArg: {
    FileDiff: {
      index: 2,
      answers: { [A]: step('one\n', [{ oldStart: 0, oldEnd: 0, newStart: 0, newEnd: 1 }]), [B]: step('one\ntwo\n', [{ oldStart: 1, oldEnd: 1, newStart: 1, newEnd: 2 }]), ...answers },
    },
  },
})
const track = (page: Page) => page.getByRole('region', { name: 'The file through time' })
const slider = (page: Page) => page.getByRole('slider', { name: 'Version of the file' })
const code = (page: Page) => page.locator('.body td.code')

test('the track opens at the newest version, a mark for each', async ({ page }) => {
  await open(page, 'blame', scenario({}))
  await expect(track(page)).toContainText('Version 3 of 3, the newest')
  await expect(track(page)).toContainText('Say three')
  await expect(track(page).locator('.mark')).toHaveCount(3)
  await expect(slider(page)).toHaveAttribute('aria-valuetext', /^Version 3 of 3, the newest: Say three, Ana Cruz, /)
  await expect(code(page)).toHaveText(['one', 'two', 'three'])
})

test('stepping back shows each version’s text, with what it changed washed', async ({ page }) => {
  await open(page, 'blame', scenario({}))
  await expect(code(page)).toHaveCount(3)
  await track(page).getByRole('button', { name: 'The version before' }).click()
  await expect(track(page)).toContainText('Version 2 of 3')
  await expect(code(page)).toHaveText(['one', 'two'])
  expect((await calls(page, 'FileDiff')).at(-1)).toEqual(['/r', 'commit', B, 'a.txt', '', false, 'lines'])
  await expect(page.locator('.body tr.changed td.code')).toHaveText(['two'])

  // [ and ] step from anywhere in the view.
  await page.locator('.body').click()
  await page.keyboard.press('[')
  await expect(track(page)).toContainText('Version 1 of 3')
  await expect(code(page)).toHaveText(['one'])
  await page.keyboard.press(']')
  await page.keyboard.press(']')
  await expect(track(page)).toContainText('Version 3 of 3, the newest')
  // The newest is the blame again, its own change washed.
  await expect(code(page)).toHaveText(['one', 'two', 'three'])
  await expect(page.locator('.body tr.changed td.code')).toHaveText(['three'])
})

test('the slider moves with the keys, Home and End', async ({ page }) => {
  await open(page, 'blame', scenario({}))
  await slider(page).focus()
  await page.keyboard.press('Home')
  await expect(track(page)).toContainText('Version 1 of 3')
  await expect(code(page)).toHaveText(['one'])
  await page.keyboard.press('ArrowRight')
  await expect(track(page)).toContainText('Version 2 of 3')
  await page.keyboard.press('End')
  await expect(track(page)).toContainText('Version 3 of 3, the newest')
  // [ and ] work on the handle too, where the view's own keys leave a focused input alone.
  await page.keyboard.press('[')
  await expect(track(page)).toContainText('Version 2 of 3')
})

test('a version where the file was binary says so, with no text', async ({ page }) => {
  await open(page, 'blame', scenario({}, { [B]: step('', [], 'binary') }))
  await track(page).getByRole('button', { name: 'The version before' }).click()
  await expect(page.getByRole('status')).toHaveText('Not text at this version: the file was binary.')
})

test('a rename stops the track, and offers the old name', async ({ page }) => {
  await open(page, 'blame', scenario({ renamedFrom: 'lib/alerts.txt' }))
  await slider(page).focus()
  await page.keyboard.press('Home')
  const note = track(page).getByRole('note')
  await expect(note).toContainText('Before this it was lib/alerts.txt.')
  await note.getByRole('button', { name: 'Carry on under the old name' }).click()
  expect(await events(page, 'open')).toEqual([[{ mode: 'blame', file: 'lib/alerts.txt', commit: `${A}^`, writeAs: 'a.txt' }]])
})

test('changes not committed yet are the last step', async ({ page }) => {
  await open(page, 'blame', { ...scenario({ workTree: true }), responses: { ...scenario({}).responses, BlameTrack: { versions, renamedFrom: '', workTree: true }, Blame: [blame(A, 'one'), blame('0'.repeat(40), 'two, changed')] } })
  await expect(track(page)).toContainText('Not committed yet')
  await expect(track(page).locator('.mark')).toHaveCount(4)
  await expect(track(page).locator('.mark.tree')).toHaveCount(1)
  await track(page).getByRole('button', { name: 'The version before' }).click()
  await expect(track(page)).toContainText('Version 3 of 3, the newest')
})

test('a file with one version has no track', async ({ page }) => {
  await open(page, 'blame', scenario({ versions: [versions[0]] }))
  await expect(code(page)).toHaveText(['one', 'two', 'three'])
  await expect(track(page)).toHaveCount(0)
})

// #1079: who wrote each line at an older version, read once the handle settles, and worked
// out from a version next to it when that one's blame is known.
test.describe('who wrote each line, at any version', () => {
  const who = (page: Page) => page.locator('.body td.gutter .sha')
  const cStep = step('one\ntwo\nthree\n', [{ oldStart: 2, oldEnd: 2, newStart: 2, newEnd: 3 }])

  test('a step back from the newest is worked out from it, with no blame at all', async ({ page }) => {
    await open(page, 'blame', scenario({}, { [C]: cStep }))
    await expect(code(page)).toHaveCount(3)
    await track(page).getByRole('button', { name: 'The version before' }).click()
    await expect(who(page)).toHaveText(['aaaaaaa', 'bbbbbbb'])
    expect(await calls(page, 'BlameStep')).toEqual([])
  })

  test('a step back over a changed line blames only that line', async ({ page }) => {
    // The newest version changed two to TWO, and added three.
    const s = scenario({}, { [C]: step('one\nTWO\nthree\n', [{ oldStart: 1, oldEnd: 2, newStart: 1, newEnd: 3 }]) })
    await open(page, 'blame', { ...s, responses: { ...s.responses, Blame: [blame(A, 'one'), blame(C, 'TWO'), blame(C, 'three')], BlameStep: [blame(B, 'two')] } })
    await expect(code(page)).toHaveCount(3)
    await track(page).getByRole('button', { name: 'The version before' }).click()
    await expect(who(page)).toHaveText(['aaaaaaa', 'bbbbbbb'])
    expect(await calls(page, 'BlameStep')).toEqual([['/r', 'a.txt', B, [{ from: 2, to: 2 }]]])
  })

  test('a version with no neighbour read is blamed whole once the handle rests, and the next step forward from it needs nothing', async ({ page }) => {
    const s = scenario({}, { [C]: cStep })
    await open(page, 'blame', { ...s, responses: { ...s.responses, BlameStep: [blame(A, 'one')] } })
    await expect(code(page)).toHaveCount(3)
    await slider(page).focus()
    await page.keyboard.press('Home')
    await expect(who(page)).toHaveText(['aaaaaaa'])
    expect(await calls(page, 'BlameStep')).toEqual([['/r', 'a.txt', A, []]])
    await page.keyboard.press('ArrowRight')
    await expect(track(page)).toContainText('Version 2 of 3')
    await expect(who(page)).toHaveText(['aaaaaaa', 'bbbbbbb'])
    expect(await calls(page, 'BlameStep')).toHaveLength(1)
  })
})

// #1080: What changed, and How old, beside Who wrote it; the choice kept for the repository.
test.describe('three ways to read a version', () => {
  const diffOf = (before: string, after: string, blocks: object[]) => ({ ...step(after, blocks), contents: { ...step(after, blocks).contents, old: { exists: true, text: before, size: before.length } } })
  const ways = (page: Page) => track(page).getByRole('group', { name: 'How to read this version' })

  test('What changed shows what the version took out, struck through, and what it added', async ({ page }) => {
    const s = scenario({}, { [C]: diffOf('one\ntwo\n', 'one\nTWO\nthree\n', [{ oldStart: 1, oldEnd: 2, newStart: 1, newEnd: 3 }]), [B]: diffOf('one\n', 'one\ntwo\n', [{ oldStart: 1, oldEnd: 1, newStart: 1, newEnd: 2 }]) })
    await open(page, 'blame', { ...s, responses: { ...s.responses, Blame: [blame(A, 'one'), blame(C, 'TWO'), blame(C, 'three')] } })
    await expect(code(page)).toHaveCount(3)
    await ways(page).getByRole('button', { name: 'What changed' }).click()
    const rows = page.locator('table.changes tr')
    await expect(rows).toHaveCount(4)
    await expect(page.locator('table.changes td.sign')).toHaveText(['', '−', '+', '+'])
    await expect(page.locator('table.changes tr.del td.code')).toHaveText('Taken out: two')
    await expect(page.locator('table.changes tr.add td.code')).toHaveText(['TWO', 'three'])
    // An older version, the same way.
    await track(page).getByRole('button', { name: 'The version before' }).click()
    await expect(page.locator('table.changes td.sign')).toHaveText(['', '+'])
    await expect(page.locator('table.changes td.code')).toHaveText(['one', 'two'])
  })

  test('How old washes each line by how many versions back it changed, with a key', async ({ page }) => {
    await open(page, 'blame', scenario({}))
    await expect(code(page)).toHaveCount(3)
    await ways(page).getByRole('button', { name: 'How old' }).click()
    await expect(page.getByLabel('How old: the key')).toContainText('in this version')
    await expect(page.locator('.body tbody tr')).toHaveClass([/age-recent/, /age-recent/, /age-now/])
  })

  test('the way is kept for the repository', async ({ page }) => {
    await open(page, 'blame', scenario({}))
    await ways(page).getByRole('button', { name: 'How old' }).click()
    await expect(ways(page).getByRole('button', { name: 'How old' })).toHaveAttribute('aria-pressed', 'true')
    await open(page, 'blame', scenario({}))
    await expect(ways(page).getByRole('button', { name: 'How old' })).toHaveAttribute('aria-pressed', 'true')
    await expect(ways(page).getByRole('button', { name: 'Who wrote it' })).toHaveAttribute('aria-pressed', 'false')
  })
})

// #1081: Rewind these lines: a block's own history beside the code.
test.describe('rewind a block of lines', () => {
  const bv = (id: string, subject: string, from: number, to: number, lines: [string, string][]) => ({ id, author: 'Ana Cruz', time: T, subject, from, to, lines: lines.map(([kind, text]) => ({ kind, text })) })
  const history = [bv(B, 'Say two', 2, 2, [['+', 'two']])]
  const panel = (page: Page) => page.getByRole('complementary', { name: /through time/ })

  test('a blame block’s own button rewinds it, and a version shows the whole file there', async ({ page }) => {
    const s = scenario({})
    await open(page, 'blame', { ...s, responses: { ...s.responses, BlameBlock: history } })
    await expect(code(page)).toHaveCount(3)
    await page.getByRole('button', { name: 'Rewind line 2' }).click()
    expect(await calls(page, 'BlameBlock')).toEqual([['/r', 'a.txt', C, 2, 2]])
    await expect(panel(page)).toHaveAccessibleName('Line 2, through time')
    await expect(panel(page)).toContainText('1 of the file’s 3 versions changed it.')
    await expect(panel(page)).toContainText('Say two · Ana Cruz')
    await expect(panel(page).locator('.l.add')).toHaveText('+Added: two')
    await panel(page).getByRole('button', { name: `Show the whole file at ${B.slice(0, 7)}` }).click()
    await expect(track(page)).toContainText('Version 2 of 3')
    await panel(page).getByRole('button', { name: 'Close' }).click()
    await expect(panel(page)).toHaveCount(0)
  })

  test('selected lines can be rewound', async ({ page }) => {
    const s = scenario({})
    await open(page, 'blame', { ...s, responses: { ...s.responses, BlameBlock: history } })
    await expect(code(page)).toHaveCount(3)
    await page.evaluate(() => {
      const cells = document.querySelectorAll('.body td.code')
      const range = document.createRange()
      range.setStart(cells[0], 0)
      range.setEnd(cells[1], cells[1].childNodes.length)
      document.getSelection()?.removeAllRanges()
      document.getSelection()?.addRange(range)
    })
    const bar = page.getByRole('group', { name: 'Lines selected' })
    await expect(bar).toContainText('Lines 1 to 2')
    await bar.getByRole('button', { name: 'Rewind these lines' }).click()
    expect(await calls(page, 'BlameBlock')).toEqual([['/r', 'a.txt', C, 1, 2]])
    await expect(panel(page)).toHaveAccessibleName('Lines 1 to 2, through time')
  })

  test('lines not committed yet have nothing to rewind', async ({ page }) => {
    const s = scenario({ workTree: true })
    await open(page, 'blame', { ...s, responses: { ...s.responses, BlameTrack: { versions, renamedFrom: '', workTree: true } } })
    await expect(code(page)).toHaveCount(3)
    await expect(page.getByRole('button', { name: /^Rewind line/ })).toHaveCount(0)
  })
})

// #1082: restoring a version, or a block's lines, into the working tree. It asks first, and
// Undo brings back what was there. History is never edited.
test.describe('restore', () => {
  const plan = { added: 1, removed: 2, uncommitted: 3 }
  const ask = (page: Page) => page.getByRole('dialog')

  test('a version: it asks first, says what changes and what isn’t committed, and Undo takes it back', async ({ page }) => {
    const s = scenario({})
    await open(page, 'blame', { ...s, responses: { ...s.responses, RestorePlan: plan, RestoreVersion: { id: 'restore-1' } } })
    await expect(code(page)).toHaveCount(3)
    await track(page).getByRole('button', { name: 'The version before' }).click()
    await track(page).getByRole('button', { name: 'Restore this version' }).click()
    await expect(ask(page)).toContainText(`Restore a.txt as it was at ${B.slice(0, 7)}?`)
    await expect(ask(page)).toContainText('1 line added and 2 taken out, against the file as it is. No commit changes.')
    await expect(ask(page).getByRole('note')).toContainText('the file has 3 changed lines you haven’t committed. Restoring replaces them; Undo brings them back.')
    expect(await calls(page, 'RestorePlan')).toEqual([['/r', 'a.txt', B, '']])
    await ask(page).getByRole('button', { name: 'Restore' }).click()
    expect(await calls(page, 'RestoreVersion')).toEqual([['/r', 'a.txt', B, '']])
    const note = page.getByRole('status').filter({ hasText: 'is as it was' })
    await expect(note).toContainText(`a.txt is as it was at ${B.slice(0, 7)}. It’s a change in the working tree, ready to commit or throw away.`)
    await note.getByRole('button', { name: 'Undo' }).click()
    expect(await calls(page, 'UndoRestore')).toEqual([['/r', 'restore-1']])
    await expect(note).toHaveCount(0)
  })

  test('a block’s lines, as an older version had them', async ({ page }) => {
    const bv = (id: string, subject: string, lines: [string, string][]) => ({ id, author: 'Ana Cruz', time: T, subject, from: 2, to: 2, lines: lines.map(([kind, text]) => ({ kind, text })) })
    const s = scenario({})
    await open(page, 'blame', { ...s, responses: { ...s.responses, BlameBlock: [bv(C, 'Say it louder', [['-', 'two'], ['+', 'TWO']]), bv(B, 'Say two', [['+', 'two']])], RestoreBlock: { id: 'restore-2' } } })
    await expect(code(page)).toHaveCount(3)
    await page.getByRole('button', { name: 'Rewind line 2' }).click()
    const panel = page.getByRole('complementary', { name: 'Line 2, through time' })
    // The newest version of the lines is what's there already: nothing to put back.
    await expect(panel.getByRole('button', { name: 'Restore these lines' })).toHaveCount(1)
    await panel.getByRole('button', { name: 'Restore these lines' }).click()
    await expect(ask(page)).toContainText(`Put line 2 back as it was at ${B.slice(0, 7)}?`)
    await ask(page).getByRole('button', { name: 'Put them back' }).click()
    expect(await calls(page, 'RestoreBlock')).toEqual([['/r', 'a.txt', '', C, 2, 2, ['two']]])
    await expect(page.getByRole('status').filter({ hasText: 'as it was' })).toContainText(`Line 2 is as it was at ${B.slice(0, 7)}.`)
  })

  test('a version from before a rename is written under today’s name', async ({ page }) => {
    const s = scenario({})
    await open(page, 'blame', { ...s, blameWriteAs: 'lib/texts.txt', responses: { ...s.responses, RestorePlan: { added: 0, removed: 0, uncommitted: 0 }, RestoreVersion: { id: 'restore-3' } } })
    await expect(code(page)).toHaveCount(3)
    await track(page).getByRole('button', { name: 'Restore this version' }).click()
    await expect(ask(page)).toContainText(`Restore lib/texts.txt as it was at ${C.slice(0, 7)}?`)
    await expect(ask(page)).toContainText('the file is already as it was then')
    await ask(page).getByRole('button', { name: 'Restore' }).click()
    expect(await calls(page, 'RestoreVersion')).toEqual([['/r', 'lib/texts.txt', C, 'a.txt']])
  })
})

// #1083: the ways in, and a partial clone's versions.
test.describe('the ways in', () => {
  test('blamed at a commit, the handle is on its version, with newer ones after it', async ({ page }) => {
    const s = scenario({ home: 1 }, { [C]: step('one\ntwo\nthree\n', [{ oldStart: 2, oldEnd: 2, newStart: 2, newEnd: 3 }]) })
    await open(page, 'blame', { ...s, blameRev: B, responses: { ...s.responses, Blame: [blame(A, 'one'), blame(B, 'two')] } })
    await expect(track(page)).toContainText('Version 2 of 3')
    await expect(code(page)).toHaveText(['one', 'two'])
    expect((await calls(page, 'BlameTrack'))[0]).toEqual(['/r', 'a.txt', B])
    await track(page).getByRole('button', { name: 'The version after' }).click()
    await expect(track(page)).toContainText('Version 3 of 3, the newest')
    await expect(code(page)).toHaveText(['one', 'two', 'three'])
    // And back home: the view's own blame again.
    await track(page).getByRole('button', { name: 'The version before' }).click()
    await expect(track(page)).toContainText('Version 2 of 3')
    await expect(page.locator('.body td.gutter .sha')).toHaveText(['aaaaaaa', 'bbbbbbb'])
  })

  test('File history’s Blame at opens the selected commit’s, under the name it had then', async ({ page }) => {
    const renamed = { id: 'e3'.repeat(20), author: 'Ana Cruz', time: 1792000000, subject: 'rename', path: 'old.txt', status: 'M' }
    await open(page, 'fileHistory', { responses: { FileHistory: { commits: [renamed] } } })
    await page.getByRole('button', { name: `Blame at ${renamed.id.slice(0, 7)}` }).click()
    expect(await events(page, 'open')).toEqual([[{ mode: 'blame', file: 'old.txt', commit: renamed.id, writeAs: 'new.txt' }]])
  })

  test('a partial clone without a version asks once, and downloads what the track needs', async ({ page }) => {
    const s = scenario({}, { [B]: { ...step('', [], 'missing'), contents: { ...step('', []).contents, kind: 'missing', missing: ['x'.repeat(40)] } } })
    await open(page, 'blame', { ...s, responses: { ...s.responses, BlameNeeds: { partial: true, versions: 3, missing: [] } } })
    await expect(code(page)).toHaveCount(3)
    await page.evaluate(() => (window as unknown as { __respond: (r: object) => void }).__respond({ BlameNeeds: { partial: true, versions: 3, missing: ['b'.repeat(40)] } }))
    await track(page).getByRole('button', { name: 'The version before' }).click()
    const ask = page.getByRole('group', { name: 'Download the file’s versions?' })
    await expect(ask).toContainText('hasn’t been downloaded to this partial clone')
    await ask.getByRole('button', { name: 'Download the versions' }).click()
    await expect.poll(() => calls(page, 'DownloadFiles')).toHaveLength(1)
    expect((await calls(page, 'DownloadFiles'))[0][2]).toEqual(['b'.repeat(40)])
    expect((await calls(page, 'BlameNeeds')).at(-1)).toEqual(['/r', 'a.txt', ''])
  })
})

test('a commit’s file menu blames it at that commit', async ({ page }) => {
  const person = { name: 'Ana Cruz', email: 'ana@example.com', time: 1793500000 }
  const row0 = '0'.repeat(40)
  const c = { id: row0, subject: 'Commit 0', body: '', trailers: [], parents: ['a'.repeat(40)], author: person, committer: person }
  const files = [
    { status: 'M', path: 'src/alerts/sms.js', oldPath: '', added: 1, deleted: 1 },
    { status: 'D', path: 'gone.js', oldPath: '', added: 0, deleted: 3 },
  ]
  const identity = { name: 'A', email: 'a@example.com' }
  await open(page, 'repoView', {
    graphRows: 3,
    responses: { Refs: [], CommitSignatures: [], CommitNotes: [], WorkingTree: { status: { files: [] }, identity, signing: { enabled: false } }, CommitDetail: { commit: c, files }, CommitRow: 1, Blame: [], BlameTrack: { versions: [], renamedFrom: '', workTree: false, home: 0 }, BlameNeeds: { partial: false, versions: 0, missing: [] } },
  })
  const box = (await page.locator('.scroller').boundingBox())!
  await page.mouse.click(box.x + 500, box.y + 52 + 26)
  await expect(page.getByRole('heading', { name: 'Commit 0' })).toBeVisible()
  const tab = page.getByRole('tab', { name: 'Files', exact: true })
  if (await tab.count()) await tab.click()
  await page.locator('.files .file-row', { hasText: 'gone.js' }).click({ button: 'right' })
  await expect(page.getByRole('menu').getByRole('menuitem', { name: 'Blame at this commit' })).toHaveCount(0)
  await page.keyboard.press('Escape')
  await page.locator('.files .file-row', { hasText: 'sms.js' }).click({ button: 'right' })
  await page.getByRole('menu').getByRole('menuitem', { name: 'Blame at this commit' }).click()
  await expect.poll(() => calls(page, 'Blame')).toEqual([['/r', 'src/alerts/sms.js', row0]])
})
