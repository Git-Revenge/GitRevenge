import { expect, test, type Page } from '@playwright/test'
import { calls, events, open } from './support'

// Trailer care (#832): the questions the app won't answer for a repository, what it joined
// without asking, and every key with where it shows and why. An answer is a rule in git
// config, written the moment it's given.

const key = (label: string, kind: string, commits: number, values: number, over: object = {}, held = '') => ({
  key: { id: label.toLowerCase(), label, spellings: [{ text: label, commits }], kind, commits, lines: commits, values, held, known: true },
  where: held ? 'held' : 'panel',
  kindSaid: false,
  whereSaid: false,
  also: [],
  scope: '',
  ...over,
})
const slip = (other: string, keyLabel: string, otherCommits: number, commits: number) => ({ kind: 'spelling', key: keyLabel.toLowerCase(), keyLabel, commits, other: other.toLowerCase(), otherLabel: other, otherCommits, shared: 0, examples: [] })
const same = (other: string, keyLabel: string, shared: number, examples: string[]) => ({ kind: 'same', key: keyLabel.toLowerCase(), keyLabel, commits: 24, other: other.toLowerCase(), otherLabel: other, otherCommits: 12, shared, examples })
const done = { reading: false, read: 212, total: 212, done: true, commits: 140 }
const keys = [key('Signed-off-by', 'person', 900, 40), key('Reviewed-by', 'person', 58, 12), key('Closes', 'ticket', 24, 24), key('Fixes', 'ticket', 12, 12), key('Changelog', 'label', 80, 3), key('Change-Id', 'identifier', 70, 70, {}, 'identifier'), key('Note', 'text', 1, 1, {}, 'rare')]
const care = (over: object = {}) => ({
  questions: [slip('Rviewed-by', 'Reviewed-by', 2, 58), slip('Reviewed-bt', 'Reviewed-by', 1, 58), same('Fixes', 'Closes', 0, [])],
  open: 3,
  people: [{ kind: 'person', key: '', keyLabel: '', commits: 4, other: '', otherLabel: '', otherCommits: 0, shared: 0, examples: ['Sarah Lind <sarah@tidewater.example>', 'sarah lind <slind@users.noreply.example>'] }],
  peopleOpen: 1,
  joined: { spellings: [{ ...keys[2].key, spellings: [{ text: 'Closes', commits: 20 }, { text: 'closed', commits: 4 }] }], several: 3, bots: 1 },
  keys,
  status: done,
  ...over,
})
const dialog = (page: Page) => page.getByRole('dialog', { name: 'Trailer care' })
const show = async (page: Page, c: object = care(), more: object = {}) => {
  await open(page, 'trailerCare', { responses: { TrailerCare: c, ...more } })
  await expect(dialog(page).getByRole('heading', { name: 'Keys' })).toBeVisible()
}
test.describe('Trailer care', () => {
  test('asks about slips of one key together, and about keys holding the same things', async ({ page }) => {
    await show(page)
    await expect(dialog(page).getByRole('status').first()).toHaveText('3 questions')
    const slips = dialog(page).getByRole('group', { name: 'Spellings one letter off Reviewed-by' })
    await expect(slips).toContainText('2 keys are one letter off Reviewed-by, which is on 58 commits.')
    await expect(slips.getByRole('listitem')).toHaveCount(2)
    await expect(slips.getByRole('button', { name: 'Read all 2 as Reviewed-by' })).toBeVisible()
    const verbs = dialog(page).getByRole('group', { name: 'Fixes and Closes' })
    await expect(verbs).toContainText('both settle tickets, though no ticket is under both. Joined, the row reads “Closes”.')
    // Who it's for is in the title.
    await expect(dialog(page)).toContainText('tidewater')
  })

  test('an answer is written at once, and what comes back is shown', async ({ page }) => {
    await show(page, care(), { AnswerTrailerQuestion: care({ questions: [slip('Reviewed-bt', 'Reviewed-by', 1, 58)], open: 1 }) })
    const slips = dialog(page).getByRole('group', { name: 'Spellings one letter off Reviewed-by' })
    await slips.getByRole('listitem').filter({ hasText: 'Rviewed-by' }).getByRole('button', { name: 'Read as Reviewed-by' }).click()
    await expect.poll(() => calls(page, 'AnswerTrailerQuestion')).toEqual([['/r', 'reviewed-by', 'rviewed-by', true]])
    await expect(dialog(page).getByRole('status').first()).toHaveText('1 question')
    await expect(slips.getByRole('listitem')).toHaveCount(1)
    // One left: nothing to answer together.
    await expect(slips.getByRole('button', { name: /^Read all/ })).toHaveCount(0)
  })

  test('Keep apart says which two, and is remembered by the backend', async ({ page }) => {
    await show(page, care(), { AnswerTrailerQuestion: care({ questions: [], open: 0 }) })
    await dialog(page).getByRole('button', { name: 'Keep Fixes apart from Closes' }).click()
    await expect.poll(() => calls(page, 'AnswerTrailerQuestion')).toEqual([['/r', 'closes', 'fixes', false]])
    await expect(dialog(page).getByRole('status').first()).toHaveText('Nothing needs care.')
  })

  test('every slip of a key can be answered together', async ({ page }) => {
    await show(page, care(), { AnswerTrailerSlips: care({ questions: [same('Fixes', 'Closes', 0, [])], open: 1 }) })
    await dialog(page).getByRole('button', { name: 'Read all 2 as Reviewed-by' }).click()
    // One call for them all: the history is read through the new rule once.
    await expect.poll(() => calls(page, 'AnswerTrailerSlips')).toEqual([['/r', 'reviewed-by', ['rviewed-by', 'reviewed-bt']]])
    expect(await calls(page, 'AnswerTrailerQuestion')).toEqual([])
    await expect(dialog(page).getByRole('status').first()).toHaveText('1 question')
    await expect(dialog(page).getByRole('group', { name: 'Spellings one letter off Reviewed-by' })).toHaveCount(0)
  })

  test('says when more questions wait than are listed', async ({ page }) => {
    await show(page, care({ open: 232 }))
    await expect(dialog(page).getByRole('status').first()).toHaveText('232 questions. The first 3 are here; answer some and the next ones show.')
  })

  test('people written two ways are shown, and answered in People', async ({ page }) => {
    await show(page)
    const people = dialog(page).getByRole('group', { name: 'People written two ways' })
    await expect(people).toContainText('1 person is written two ways. They join once the mailmap says so')
    await expect(people).toContainText('sarah lind <slind@users.noreply.example>')
    await people.getByRole('button', { name: 'Open People' }).click()
    expect(await events(page, 'close')).toHaveLength(1)
  })

  test('what was joined without asking is there to read, folded until asked for', async ({ page }) => {
    await show(page)
    const fold = dialog(page).getByRole('button', { name: 'Joined without asking' })
    await expect(fold).toHaveAttribute('aria-expanded', 'false')
    await expect(dialog(page).getByText('Several on a line.')).toHaveCount(0)
    await fold.click()
    await expect(fold).toHaveAttribute('aria-expanded', 'true')
    await expect(dialog(page).getByText('1 key is written more than one way and read as one:')).toBeVisible()
    await expect(dialog(page).locator('.spelled')).toHaveText('Closes, closed')
    await expect(dialog(page).getByText('3 lines name more than one value')).toBeVisible()
    await expect(dialog(page).getByText('1 of the people named is a bot')).toBeVisible()
  })

  test('the table sorts by a column, and turns round on a second click', async ({ page }) => {
    await show(page)
    const names = () => dialog(page).locator('tbody .key-name').allTextContents()
    // Most commits first, to begin with.
    expect(await names()).toEqual(['Signed-off-by', 'Changelog', 'Change-Id', 'Reviewed-by', 'Closes', 'Fixes', 'Note'])
    const header = (name: string) => dialog(page).getByRole('columnheader', { name })
    await expect(header('Commits')).toHaveAttribute('aria-sort', 'descending')
    await header('Key').getByRole('button').click()
    expect(await names()).toEqual(['Change-Id', 'Changelog', 'Closes', 'Fixes', 'Note', 'Reviewed-by', 'Signed-off-by'])
    await expect(header('Key')).toHaveAttribute('aria-sort', 'ascending')
    await expect(header('Commits')).toHaveAttribute('aria-sort', 'none')
    await header('Key').getByRole('button').click()
    expect((await names())[0]).toBe('Signed-off-by')
    await expect(header('Key')).toHaveAttribute('aria-sort', 'descending')
    await header('Values').getByRole('button').click()
    expect((await names())[0]).toBe('Change-Id')
    await expect(header('Values')).toHaveAttribute('aria-sort', 'descending')
    await header('Values').getByRole('button').click()
    expect((await names())[0]).toBe('Note')
    await expect(header('Values')).toHaveAttribute('aria-sort', 'ascending')
  })

  test('the filter finds a key by name, kind or place, and counts what it keeps', async ({ page }) => {
    await show(page)
    const filter = dialog(page).getByRole('searchbox', { name: 'Filter keys' })
    const rows = dialog(page).locator('tbody tr')
    await expect(dialog(page).getByText('7 keys', { exact: true })).toBeVisible()
    await filter.fill('by')
    await expect(rows).toHaveCount(2)
    await expect(dialog(page).getByText('2 of 7 keys')).toBeVisible()
    await filter.fill('held')
    await expect(rows).toHaveCount(2)
    await filter.fill('tickets')
    await expect(rows).toHaveCount(2)
    await filter.fill('zzz')
    await expect(rows).toHaveCount(0)
    await expect(dialog(page).getByText('No key matches that.')).toBeVisible()
  })

  test('a kind and a place are guesses until changed, and say why', async ({ page }) => {
    const held = keys.map((k) => (k.key.id === 'changelog' ? key('Changelog', 'label', 80, 3, { where: 'held', whereSaid: true, scope: 'local' }, 'rule') : k))
    await show(page, care(), { SetTrailerPlace: care({ keys: held }), SetTrailerKind: care() })
    const row = dialog(page).locator('tbody tr').filter({ hasText: 'Changelog' })
    const place = row.getByRole('combobox', { name: 'Where Changelog shows' })
    await expect(place.locator('option:checked')).toHaveText('Panel (a guess)')
    await expect(row.locator('.why')).toHaveText('A guess: its values are labels, on 80 commits.')
    await place.selectOption('held')
    await expect.poll(() => calls(page, 'SetTrailerPlace')).toEqual([['/r', 'changelog', 'held']])
    await expect(place.locator('option:checked')).toHaveText('Held back')
    await expect(row.locator('.why')).toHaveText('This repository’s git config says so.')

    // A key held back says why it was guessed so.
    await expect(dialog(page).locator('tbody tr').filter({ hasText: 'Change-Id' }).locator('.why')).toHaveText('A guess: it’s a different value on nearly every commit.')
    const kind = dialog(page).locator('tbody tr').filter({ hasText: 'Note' }).getByRole('combobox', { name: 'What the values of Note are' })
    await expect(kind.locator('option:checked')).toHaveText('text (a guess)')
    await kind.selectOption('labels')
    await expect.poll(() => calls(page, 'SetTrailerKind')).toEqual([['/r', 'note', 'labels']])
  })

  test('a join shows on its key, and can be undone there', async ({ page }) => {
    const joined = keys.map((k) => (k.key.id === 'reviewed-by' ? key('Reviewed-by', 'person', 60, 12, { also: ['Rviewed-by'], scope: 'local' }) : k))
    await show(page, care({ keys: joined, questions: [], open: 0 }), { UndoTrailerJoin: care() })
    const row = dialog(page).locator('tbody tr').filter({ hasText: 'Reviewed-by' })
    await expect(row).toContainText('reads Rviewed-by too')
    await row.getByRole('button', { name: 'Stop reading Rviewed-by as Reviewed-by' }).click()
    await expect.poll(() => calls(page, 'UndoTrailerJoin')).toEqual([['/r', 'reviewed-by', 'Rviewed-by']])
    // The question is back.
    await expect(dialog(page).getByRole('status').first()).toHaveText('3 questions')
  })

  test('a long table shows a hundred keys at a time', async ({ page }) => {
    const many = Array.from({ length: 260 }, (_, i) => key(`Key-${String(i).padStart(3, '0')}`, 'label', 500 - i, 3))
    await show(page, care({ keys: many, questions: [], open: 0 }))
    const rows = dialog(page).locator('tbody tr')
    await expect(rows).toHaveCount(100)
    await dialog(page).getByRole('button', { name: 'Show 100 more of 160' }).click()
    await expect(rows).toHaveCount(200)
    await dialog(page).getByRole('button', { name: 'Show 60 more of 60' }).click()
    await expect(rows).toHaveCount(260)
    await expect(dialog(page).getByRole('button', { name: /^Show \d+ more/ })).toHaveCount(0)
    // A filter starts from the top again.
    await dialog(page).getByRole('searchbox', { name: 'Filter keys' }).fill('key-2')
    await expect(rows).toHaveCount(60)
  })

  test('says what went wrong when a rule cannot be written, and shows what stands', async ({ page }) => {
    await open(page, 'trailerCare', { responses: { TrailerCare: care() }, failures: { AnswerTrailerQuestion: 'That couldn’t be written to the repository’s git config: locked' } })
    await expect(dialog(page).getByRole('heading', { name: 'Keys' })).toBeVisible()
    await dialog(page).getByRole('button', { name: 'Keep Fixes apart from Closes' }).click()
    await expect(dialog(page).getByRole('alert')).toContainText('couldn’t be written')
    // The questions are still there to answer.
    await expect(dialog(page).getByRole('group', { name: 'Fixes and Closes' })).toBeVisible()
  })

  test('says counts are so far while the history is still being read', async ({ page }) => {
    await show(page, care({ status: { ...done, reading: true, done: false, read: 90 } }))
    await expect(dialog(page)).toContainText('Counts so far: 90 of 212 commits read.')
  })
})

test.describe('the panel’s Needs care pill', () => {
  const value = (label: string, kind: string, commits: number) => ({ id: `${kind}:${label}`, label, kind, commits })
  const panel = (careCount: number) => ({
    keys: [{ key: keys[1].key, top: [value('Sam Ortiz <sam@tidewater.example>', 'person', 30)], more: 0 }],
    moreKeys: 0,
    held: [],
    matches: 0,
    status: done,
    care: careCount,
  })
  const show = async (page: Page, careCount: number) => {
    await open(page, 'sidebar', { pinnedKeys: ['reviewed-by'], responses: { Refs: [], ShowingHidden: false, Worktrees: [], TrailerPanel: panel(careCount) } })
    // The sidebar remembers its panel, and a click on its button while it shows would fold
    // the sidebar away: the panel is given a moment to show first.
    const tree = page.getByRole('tree', { name: 'Smart filters' })
    const showing = await tree
      .waitFor({ state: 'visible', timeout: 1500 })
      .then(() => true)
      .catch(() => false)
    if (!showing) await page.getByRole('navigation', { name: 'Sidebar panels' }).getByRole('button', { name: /^Smart filters/ }).click()
    await expect(tree).toBeVisible()
  }

  test('counts the open questions', async ({ page }) => {
    await show(page, 3)
    await expect(page.getByRole('button', { name: 'Needs care 3' })).toBeVisible()
  })

  test('is not there when nothing needs care', async ({ page }) => {
    await show(page, 0)
    await expect(page.getByRole('button', { name: /^Needs care/ })).toHaveCount(0)
  })
})
