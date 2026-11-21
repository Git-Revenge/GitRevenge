import { expect, test, type Page } from '@playwright/test'
import { calls, open } from './support'

// The People tab (#837): who a repository's history names, how its mailmap reads them, and
// the mailmap itself. The backend is stubbed, so these check what the screen shows and what
// it asks for, above all that nothing is written, and no file created, without a yes.

const id = (name: string, email: string, commits: number, lines: { source: string; line: number }[] = [], coAuthored = 0) => ({ name, email, commits, coAuthored, lines })
const person = (name: string, email: string, commits: number, identities = [id(name, email, commits)], more: object = {}) => ({ name, email, commits, coAuthored: 0, bot: false, identities, ...more })

const asWritten = {
  people: [
    person('Maya Okafor', 'maya@tidewater.example', 71),
    person('Priya Raman', 'priya@tidewater.example', 48),
    person('Priya R', 'priya@oldmill.example', 9),
    person('dependabot[bot]', 'bot@users.noreply.example', 5, undefined, { bot: true }),
  ],
  matching: 4,
  everyone: 4,
  bots: 1,
  identities: 4,
  commits: 133,
}

const line = 'Priya Raman <priya@tidewater.example> <priya@oldmill.example>'
const joined = {
  people: [
    person('Maya Okafor', 'maya@tidewater.example', 71),
    person('Priya Raman', 'priya@tidewater.example', 57, [id('Priya Raman', 'priya@tidewater.example', 48), id('Priya R', 'priya@oldmill.example', 9, [{ source: '.mailmap', line: 1 }])]),
    person('dependabot[bot]', 'bot@users.noreply.example', 5, undefined, { bot: true }),
  ],
  matching: 3,
  everyone: 3,
  bots: 1,
  identities: 4,
  commits: 133,
}

const noMailmap = { MailmapSources: [{ source: '.mailmap', path: '/r/.mailmap', exists: false, editable: true, lines: 0 }], ReadMailmapFile: '', People: asWritten, MailmapNotes: [], WhoIs: [] }
const withMailmap = { MailmapSources: [{ source: '.mailmap', path: '/r/.mailmap', exists: true, editable: true, lines: 1 }], ReadMailmapFile: `${line}\n`, People: joined, MailmapNotes: [], WhoIs: [] }

const save = (page: Page) => page.locator('.foot').getByRole('button', { name: /mailmap/ })
const list = (page: Page) => page.getByRole('list', { name: 'People' })

test.describe('the People tab', () => {
  test('a repository with no mailmap works without one', async ({ page }) => {
    await open(page, 'people', { responses: noMailmap })
    // It says there's no mailmap, and that none is needed: a note, not an error.
    await expect(page.getByRole('heading', { name: '.mailmap' })).toBeVisible()
    await expect(page.locator('.head .note')).toHaveText('none in this repository')
    await expect(page.locator('.none')).toContainText("has no mailmap and doesn't need one")
    await expect(page.getByRole('alert')).toHaveCount(0)
    // Everyone is listed as their commits have them.
    await expect(list(page).getByRole('listitem')).toHaveCount(4)
    await expect(list(page)).toContainText('Priya R')
    await expect(page.locator('.sum')).toHaveText('3 names and 1 bot on 133 commits')
    await expect(list(page).getByRole('listitem').filter({ hasText: 'dependabot[bot]' })).toContainText('bot')
    // The button says a file would be created, and there's nothing to save yet.
    await expect(save(page)).toHaveText('Create .mailmap…')
    await expect(save(page)).toBeDisabled()
    await expect(page.locator('.foot')).toContainText('only created when you say so')
    expect(await calls(page, 'SaveMailmap')).toHaveLength(0)
  })

  test('the first .mailmap is only created after a yes', async ({ page }) => {
    await open(page, 'people', { responses: noMailmap })
    await page.getByRole('button', { name: 'Text', exact: true }).click()
    await expect(page.locator('.rules')).toContainText("There's no .mailmap, and there doesn't have to be")
    await page.getByLabel('New line').fill(line)
    await page.getByRole('button', { name: 'Add', exact: true }).click()
    await expect(page.locator('.foot')).toContainText('1 line not saved')
    // The people are asked for again through the text being edited, which isn't saved.
    await expect.poll(async () => (await calls(page, 'People')).some((c) => (c[1] as { mailmap: { edited: boolean; text: string } }).mailmap.edited)).toBe(true)

    // Saving asks first.
    await save(page).click()
    const ask = page.getByRole('dialog', { name: 'Create .mailmap?' })
    await expect(ask).toContainText('/r/.mailmap')
    await expect(ask).toContainText(line)
    await expect(ask).toContainText('No commit changes')
    expect(await calls(page, 'SaveMailmap')).toHaveLength(0)

    // Not now: nothing is written, and the line is still waiting.
    await ask.getByRole('button', { name: 'Not now' }).click()
    await expect(ask).toHaveCount(0)
    expect(await calls(page, 'SaveMailmap')).toHaveLength(0)
    await expect(page.locator('.foot')).toContainText('1 line not saved')

    // Yes: the save says the user agreed to the new file.
    await save(page).click()
    await page.getByRole('dialog', { name: 'Create .mailmap?' }).getByRole('button', { name: 'Create .mailmap' }).click()
    await expect.poll(() => calls(page, 'SaveMailmap')).toEqual([['/r', '', `${line}\n`, true]])
  })

  test('identities a line joins sit under the person, and can be taken apart', async ({ page }) => {
    await open(page, 'people', { responses: withMailmap })
    await expect(page.locator('.head .note')).toHaveText('shared · committed with the repository')
    await expect(page.locator('.none')).toHaveCount(0)
    const priya = list(page).getByRole('listitem').filter({ hasText: 'Priya Raman' })
    await expect(priya).toContainText('57 commits')
    await expect(priya.locator('.also')).toContainText('also Priya R priya@oldmill.example · 9 commits, line 1')

    // A small text button, sized to its words: it once took on the text box's style.
    const apart = priya.getByRole('button', { name: 'Take apart' })
    expect((await apart.boundingBox())!.width).toBeLessThan(120)
    await apart.click()
    // The line is gone from the text being edited, and nothing is saved yet.
    await expect(page.locator('.foot')).toContainText('1 line not saved')
    await expect.poll(async () => (await calls(page, 'People')).some((c) => (c[1] as { mailmap: { edited: boolean; text: string } }).mailmap.edited && (c[1] as { mailmap: { text: string } }).mailmap.text === '')).toBe(true)
    expect(await calls(page, 'SaveMailmap')).toHaveLength(0)

    // The file exists, so saving it needs no question.
    await expect(save(page)).toHaveText('Save .mailmap')
    await save(page).click()
    await expect.poll(() => calls(page, 'SaveMailmap')).toEqual([['/r', `${line}\n`, '', false]])
    await expect(page.getByRole('dialog')).toHaveCount(0)
  })

  test('Text says what each line does', async ({ page }) => {
    const text = `# Tidewater\n${line}\nJo Whitlock <jo@tidewater.example> <jo@harbour.example>\nLeo Brandt leo@tidewater.example\n`
    const note = (n: number, kind: string, over: object = {}) => ({ line: n, kind, identities: 0, commits: 0, coAuthored: 0, replaced: false, ...over })
    await open(page, 'people', {
      responses: {
        ...withMailmap,
        ReadMailmapFile: text,
        MailmapNotes: [note(1, 'comment'), note(2, 'map', { identities: 1, commits: 9 }), note(3, 'map'), note(4, 'skipped', { why: 'no address in < >' })],
      },
    })
    await page.getByRole('button', { name: 'Text', exact: true }).click()
    const rows = page.locator('.rules .rule')
    await expect(rows).toHaveCount(4)
    await expect(rows.nth(1).locator('.what')).toHaveText('joins 9 commits')
    await expect(rows.nth(2).locator('.what')).toHaveText('matches nobody now')
    await expect(rows.nth(3).locator('.what')).toHaveText('git skips this: no address in < >')
    await expect(rows.nth(0).locator('.what')).toHaveText('')

    // The whole file can be edited as one box, and that edit isn't saved either.
    await page.getByRole('button', { name: 'Edit as plain text' }).click()
    await expect(page.getByLabel('.mailmap as text')).toHaveValue(text)
  })

  test('Who is this? answers with the line that decides', async ({ page }) => {
    const answer = { written: { name: 'Priya R', email: 'priya@oldmill.example' }, shown: { name: 'Priya Raman', email: 'priya@tidewater.example' }, lines: [{ source: '.mailmap', line: 1 }], commits: 9, coAuthored: 0, known: true }
    await open(page, 'people', { responses: { ...withMailmap, WhoIs: [answer] } })
    await page.getByRole('textbox', { name: 'Who is this?' }).fill('priya@oldmill.example')
    const card = page.locator('.answer')
    await expect(card).toContainText('Priya Raman')
    await expect(card).toContainText('by line 1 of .mailmap, for 9 commits')
    await expect(card).toContainText('A commit says Priya R <priya@oldmill.example>')
    await expect.poll(async () => (await calls(page, 'WhoIs')).at(-1)?.[2]).toBe('priya@oldmill.example')
  })

  // The tab bar loads with the app and the view doesn't, so the tab bar only knows whether
  // there's an unsaved edit, and the view is what asks before one is thrown away.
  test('closing with an unsaved edit asks first', async ({ page }) => {
    type Win = { __closePeople: () => Promise<void>; __peopleTab: () => { open: boolean; dirty: boolean } }
    const tab = () => page.evaluate(() => (window as unknown as Win).__peopleTab())
    const close = () => page.evaluate(() => void (window as unknown as Win).__closePeople())
    await open(page, 'people', { responses: withMailmap })
    await expect.poll(tab).toEqual({ open: true, dirty: false })

    await list(page).getByRole('listitem').filter({ hasText: 'Priya Raman' }).getByRole('button', { name: 'Take apart' }).click()
    await expect.poll(tab).toEqual({ open: true, dirty: true })

    await close()
    const ask = page.getByRole('dialog', { name: 'Discard unsaved changes to the mailmap?' })
    await expect(ask).toContainText('Nothing has been written')
    await ask.getByRole('button', { name: 'Keep editing' }).click()
    await expect(ask).toHaveCount(0)
    await expect.poll(tab).toEqual({ open: true, dirty: true })

    await close()
    await page.getByRole('dialog', { name: 'Discard unsaved changes to the mailmap?' }).getByRole('button', { name: 'Discard and close' }).click()
    await expect.poll(tab).toEqual({ open: false, dirty: false })
    expect(await calls(page, 'SaveMailmap')).toHaveLength(0)
    // Closed is remembered, so the tab doesn't come back at the next launch.
    await expect.poll(async () => (await calls(page, 'SetPeopleTab')).at(-1)).toEqual([''])
  })
})

// Same person (#838): likely matches are offered, never applied; the dialog shows the line
// before anything changes; and the line becomes an unsaved edit, not a write.
test.describe('saying two people are one', () => {
  const who = (name: string, email: string, commits: number) => ({ name, email, commits, coAuthored: 0, bot: false, identities: [{ name, email }] })
  const sarah = who('Sarah Lind', 'sarah@tidewater.example', 2)
  const slind = who('sarah lind', 'slind@users.noreply.example', 1)
  const offer = { key: 'name:sarah lind', kind: 'name', people: [sarah, slind] }
  const models = { key: 'address:noreply@models.example', kind: 'address', why: 'a no-reply address', people: [who('Claude Opus', 'noreply@models.example', 3), who('Claude Fable', 'noreply@models.example', 2)] }
  const apart = { key: 'address:pair@tidewater.example', kind: 'address', people: [who('Jo Whitlock', 'pair@tidewater.example', 4), who('sam', 'pair@tidewater.example', 1)] }
  const suggestions = { offers: [offer], offered: 1, shared: [models], sharedCount: 1, apart: [apart], apartCount: 1 }
  const joinLine = 'Sarah Lind <sarah@tidewater.example> <slind@users.noreply.example>'
  const plan = { lines: [joinLine], at: [1], text: `${joinLine}\n`, named: [], blocked: [] }
  const responses = { ...noMailmap, PeopleSuggestions: suggestions, PlanSamePerson: plan }
  const section = (page: Page) => page.getByRole('region', { name: 'Likely the same person' })

  test('likely matches are offered, and the rest is folded away', async ({ page }) => {
    await open(page, 'people', { responses })
    await expect(section(page).getByRole('heading', { name: 'Looks like the same person' })).toBeVisible()
    await expect(section(page)).toContainText('Offers only. Nothing is joined unless you say so.')
    const row = section(page).locator('.offer').first()
    await expect(row).toContainText('Same name, another address')
    await expect(row).toContainText('Sarah Lind sarah@tidewater.example · 2 commits')
    await expect(row).toContainText('sarah lind slind@users.noreply.example · 1 commit')

    // Bots and shared addresses aren't offered. They're there to open, with why.
    await expect(section(page)).not.toContainText('Claude Opus')
    const shared = section(page).getByRole('button', { name: /Bots and shared addresses/ })
    await expect(shared).toHaveAttribute('aria-expanded', 'false')
    await shared.click()
    await expect(section(page)).toContainText('Same address, another name · a no-reply address')
    await expect(section(page)).toContainText('Claude Opus')

    // Nothing was joined or written by looking.
    expect(await calls(page, 'SaveMailmap')).toHaveLength(0)
    expect(await calls(page, 'KeepApart')).toHaveLength(0)
  })

  test('Different people is remembered, and can be offered again', async ({ page }) => {
    await open(page, 'people', { responses })
    await section(page).locator('.offer').first().getByRole('button', { name: 'Different people' }).click()
    await expect.poll(() => calls(page, 'KeepApart')).toEqual([['/r', 'name:sarah lind', true]])

    await section(page).getByRole('button', { name: /Kept apart/ }).click()
    const kept = section(page).locator('.offer').filter({ hasText: 'Jo Whitlock' })
    await expect(kept).toContainText('sam pair@tidewater.example')
    await kept.getByRole('button', { name: 'Offer again' }).click()
    await expect.poll(async () => (await calls(page, 'KeepApart')).at(-1)).toEqual(['/r', 'address:pair@tidewater.example', false])
  })

  test('the dialog shows the line before it is added, and adding it saves nothing', async ({ page }) => {
    await open(page, 'people', { responses })
    await section(page).locator('.offer').first().getByRole('button', { name: 'Same person…' }).click()
    const dialog = page.getByRole('dialog', { name: 'Two identities, one person' })
    await expect(dialog).toContainText('This adds one line')
    await expect(dialog.locator('.lines')).toHaveText(joinLine)
    await expect(dialog).toContainText('No other spelling of a name is written out')
    await expect(dialog).toContainText('No commit changes')
    // This repository has no mailmap: the dialog says adding the line doesn't create one.
    await expect(dialog).toContainText("has no .mailmap yet. Adding the line doesn't create one: saving asks you first.")

    // Shown as the one with the most commits, with every identity of both covered.
    type Req = { identities: { name: string; email: string }[]; name: string; email: string }
    const last = async () => (await calls(page, 'PlanSamePerson')).at(-1)?.[2] as Req
    await expect.poll(last).toEqual({ identities: [sarah.identities[0], slind.identities[0]], name: 'Sarah Lind', email: 'sarah@tidewater.example' })
    // Picking the other, or typing, asks again.
    await dialog.getByRole('radio').nth(1).check()
    await expect.poll(async () => (await last()).name).toBe('sarah lind')
    await dialog.getByLabel('Name').fill('Sarah L. Lind')
    await expect.poll(async () => (await last()).name).toBe('Sarah L. Lind')

    await dialog.getByRole('button', { name: 'Add the line' }).click()
    await expect(dialog).toHaveCount(0)
    // The line is an edit in the view: not saved, and no file made.
    await expect(page.locator('.foot')).toContainText('1 line not saved')
    await expect(page.locator('.foot').getByRole('button', { name: /mailmap/ })).toHaveText('Create .mailmap…')
    await expect.poll(async () => (await calls(page, 'People')).some((c) => (c[1] as { mailmap: { edited: boolean; text: string } }).mailmap.text === `${joinLine}\n`)).toBe(true)
    expect(await calls(page, 'SaveMailmap')).toHaveLength(0)
  })

  test('a line that has to name the commit says so, and one that cannot be written says why', async ({ page }) => {
    const named = { ...plan, lines: ['Sarah Lind <sarah@tidewater.example> sarah lind <slind@users.noreply.example>'], named: [slind.identities[0]] }
    await open(page, 'people', { responses: { ...responses, PlanSamePerson: named } })
    await section(page).locator('.offer').first().getByRole('button', { name: 'Same person…' }).click()
    const dialog = page.getByRole('dialog', { name: 'Two identities, one person' })
    await expect(dialog).toContainText("For sarah lind <slind@users.noreply.example> the line gives the commit's name as well")
    await expect(dialog).not.toContainText('No other spelling of a name is written out')
    await dialog.getByRole('button', { name: 'Cancel' }).click()
    await expect(page.locator('.foot')).not.toContainText('not saved')
  })

  test('nothing can be added when another mailmap decides', async ({ page }) => {
    const blocked = { lines: [], at: [], text: '', named: [], blocked: [slind.identities[0]] }
    await open(page, 'people', { responses: { ...responses, PlanSamePerson: blocked } })
    await section(page).locator('.offer').first().getByRole('button', { name: 'Same person…' }).click()
    const dialog = page.getByRole('dialog', { name: 'Two identities, one person' })
    await expect(dialog.getByRole('alert')).toContainText("sarah lind <slind@users.noreply.example> can't be joined from this file")
    await expect(dialog.getByRole('button', { name: 'Add the line' })).toBeDisabled()
  })

  test('any two people in the list can be joined by hand', async ({ page }) => {
    await open(page, 'people', { responses })
    const rows = list(page).getByRole('listitem')
    await rows.filter({ hasText: 'Maya Okafor' }).getByRole('button', { name: 'Same person as…' }).click()
    await expect(page.getByRole('status')).toContainText('Who is Maya Okafor the same person as?')
    // Her own row offers nothing; the others offer themselves.
    await expect(rows.filter({ hasText: 'Maya Okafor' }).getByRole('button')).toHaveCount(0)
    await rows.filter({ hasText: 'Priya R' }).first().getByRole('button', { name: 'This one' }).click()
    const dialog = page.getByRole('dialog', { name: 'Two identities, one person' })
    await expect(dialog.getByRole('radio')).toHaveCount(2)
    await expect(dialog).toContainText('Maya Okafor')
    await expect(page.getByRole('status')).toHaveCount(0)
    // Cancelling the choice, on another try, puts the list back as it was.
    await dialog.getByRole('button', { name: 'Cancel' }).click()
    await rows.filter({ hasText: 'Maya Okafor' }).getByRole('button', { name: 'Same person as…' }).click()
    await page.getByRole('status').getByRole('button', { name: 'Cancel' }).click()
    await expect(page.getByRole('status')).toHaveCount(0)
  })
})
