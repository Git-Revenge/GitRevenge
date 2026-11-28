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
  expect(await events(page, 'open')).toEqual([[{ mode: 'blame', file: 'lib/alerts.txt', commit: `${A}^` }]])
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
