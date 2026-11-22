import { expect, test, type Page } from '@playwright/test'
import { calls, events, open } from './support'

// The search box suggests trailer keys and values (#830). The text search is always the
// first row and where the keys start, so Enter keeps moving between matches; a picked value
// joins the graph's smart filter.

const value = (key: string, keyLabel: string, label: string, kind: string, commits: number) => ({ key, keyLabel, value: `${kind}:${label}`, label, kind, commits })
const sarah = value('reviewed-by', 'Reviewed-by', 'Sarah Lind <sarah@tidewater.example>', 'person', 12)
const sam = value('reviewed-by', 'Reviewed-by', 'Sam Ortiz <sam@tidewater.example>', 'person', 9)
const changelog = { key: 'changelog', keyLabel: 'Changelog', value: '', label: '', kind: 'label', commits: 140 }
const done = { reading: false, read: 212, total: 212, done: true, commits: 140 }
const offered = (items: object[], match: string, status: object = done) => ({ items, match, status })

const scenario = (suggest: object) => ({ graphRows: 30, suggests: true, responses: { Refs: [], CommitSignatures: [], SearchGraph: { rows: [2, 5], total: 2 }, TrailerSuggest: suggest } })
const box = (page: Page) => page.getByRole('combobox', { name: 'Search commits' })
const list = (page: Page) => page.getByRole('listbox', { name: 'Search suggestions' })
const rows = (page: Page) => list(page).getByRole('option')
const respond = (page: Page, r: object) => page.evaluate((x) => (window as unknown as { __respond: (r: object) => void }).__respond(x), r)

test.describe('search suggestions', () => {
  test('offers the values that hold what is typed, under the text search', async ({ page }) => {
    await open(page, 'graph', scenario(offered([sarah, sam], 'sa')))
    await expect(box(page)).toHaveAttribute('aria-expanded', 'false')
    await box(page).fill('sa')
    await expect(rows(page)).toHaveCount(3)
    await expect(box(page)).toHaveAttribute('aria-expanded', 'true')
    await expect.poll(async () => (await calls(page, 'TrailerSuggest')).at(-1)).toEqual(['/repo', 'sa', 0])

    // The text search comes first, and it's the row the keys start on.
    await expect(rows(page).nth(0)).toHaveText(/Search for\s*“sa”/)
    await expect(rows(page).nth(0)).toHaveAttribute('aria-selected', 'true')
    // A person shows by name and address, with the count, and what was typed is marked.
    const first = rows(page).nth(1)
    await expect(first).toHaveAccessibleName('Reviewed-by: Sarah Lind, 12 commits')
    await expect(first.locator('.key')).toHaveText('Reviewed-by')
    await expect(first.locator('.email')).toHaveText('sarah@tidewater.example')
    await expect(first.locator('.count')).toHaveText('12')
    await expect(first.locator('mark').first()).toHaveText('Sa')
  })

  test('Enter moves between matches until an arrow key reaches a suggestion', async ({ page }) => {
    await open(page, 'graph', scenario(offered([sarah, sam], 'sa')))
    await box(page).fill('sa')
    await expect(rows(page)).toHaveCount(3)
    await box(page).press('Enter')
    // Nothing joined the filter: Enter went to a match, as it always has.
    expect(await events(page, 'filter-add')).toEqual([])
    await expect(box(page)).toHaveValue('sa')

    await box(page).press('ArrowDown')
    await expect(rows(page).nth(1)).toHaveAttribute('aria-selected', 'true')
    await expect(box(page)).toHaveAttribute('aria-activedescendant', (await rows(page).nth(1).getAttribute('id')) ?? '')
    await box(page).press('ArrowDown')
    await box(page).press('ArrowDown') // the last row: it stops there
    await expect(rows(page).nth(2)).toHaveAttribute('aria-selected', 'true')
    await box(page).press('Enter')
    expect(await events(page, 'filter-add')).toEqual([['reviewed-by', 'person:Sam Ortiz <sam@tidewater.example>']])
    // The box empties for the next value, keeps the focus, and the list goes.
    await expect(box(page)).toHaveValue('')
    await expect(box(page)).toBeFocused()
    await expect(list(page)).toHaveCount(0)
  })

  test('a click picks a value without taking the focus from the box', async ({ page }) => {
    await open(page, 'graph', scenario(offered([sarah, sam], 'sa')))
    await box(page).fill('sa')
    await rows(page).nth(1).click()
    expect(await events(page, 'filter-add')).toEqual([['reviewed-by', 'person:Sarah Lind <sarah@tidewater.example>']])
    await expect(box(page)).toBeFocused()
    await expect(box(page)).toHaveValue('')
  })

  test('a key is written in the box, where its values are offered', async ({ page }) => {
    await open(page, 'graph', scenario(offered([changelog], 'change')))
    await box(page).fill('change')
    await expect(rows(page)).toHaveCount(2)
    await expect(rows(page).nth(1)).toHaveAccessibleName('Changelog, a trailer on 140 commits')
    await respond(page, { TrailerSuggest: offered([value('changelog', 'Changelog', 'fixed', 'label', 80), value('changelog', 'Changelog', 'added', 'label', 60)], '') })
    await box(page).press('ArrowDown')
    await box(page).press('Enter')
    // Nothing is filtered yet: a key alone isn't a filter.
    expect(await events(page, 'filter-add')).toEqual([])
    await expect(box(page)).toHaveValue('Changelog: ')
    await expect.poll(async () => (await calls(page, 'TrailerSuggest')).at(-1)).toEqual(['/repo', 'Changelog: ', 0])
    await expect(rows(page)).toHaveCount(3)
    await expect(rows(page).nth(1).locator('.value')).toHaveText('fixed')
  })

  test('Tab completes with the first suggestion, or the one the keys are on', async ({ page }) => {
    await open(page, 'graph', scenario(offered([sarah, sam], 'sa')))
    await box(page).fill('sa')
    await expect(rows(page)).toHaveCount(3)
    await box(page).press('Tab')
    await expect(box(page)).toHaveValue('Reviewed-by: Sarah Lind')
    await expect(box(page)).toBeFocused()

    await box(page).fill('sa')
    // The list is asked for again for the new text before the keys go to it.
    await expect.poll(async () => (await calls(page, 'TrailerSuggest')).filter((c) => c[1] === 'sa')).toHaveLength(2)
    await expect(rows(page)).toHaveCount(3)
    await box(page).press('ArrowDown')
    await box(page).press('ArrowDown')
    await box(page).press('Tab')
    await expect(box(page)).toHaveValue('Reviewed-by: Sam Ortiz')
  })

  test('Escape leaves the list first, and the search next', async ({ page }) => {
    await open(page, 'graph', scenario(offered([sarah], 'sa')))
    await box(page).fill('sa')
    await expect(rows(page)).toHaveCount(2)
    await box(page).press('ArrowDown')
    await expect(rows(page).nth(1)).toHaveAttribute('aria-selected', 'true')
    await box(page).press('Escape')
    await expect(rows(page).nth(0)).toHaveAttribute('aria-selected', 'true')
    await expect(box(page)).toHaveValue('sa')
    await box(page).press('Escape')
    await expect(box(page)).toHaveValue('')
    await expect(list(page)).toHaveCount(0)
  })

  test('Backspace in an empty box takes the last value out of the filter', async ({ page }) => {
    await open(page, 'graph', scenario(offered([], '')))
    await box(page).focus()
    await box(page).press('Backspace')
    expect(await events(page, 'filter-drop')).toHaveLength(1)
    // With text in the box it's only a Backspace.
    await box(page).fill('sa')
    await box(page).press('Backspace')
    await expect(box(page)).toHaveValue('s')
    expect(await events(page, 'filter-drop')).toHaveLength(1)
  })

  test('says the rows are so far while trailers are still being read', async ({ page }) => {
    await open(page, 'graph', scenario(offered([sarah], 'sa', { ...done, reading: true, done: false, read: 90 })))
    await box(page).fill('sa')
    await expect(list(page)).toContainText('Trailers are still being read: these are the ones found so far.')
  })

  test('shows no list when nothing is offered, or when the graph has no filter to add to', async ({ page }) => {
    await open(page, 'graph', scenario(offered([], 'zzz')))
    await box(page).fill('zzz')
    await expect.poll(async () => (await calls(page, 'TrailerSuggest')).length).toBeGreaterThan(0)
    await expect(list(page)).toHaveCount(0)
    await expect(box(page)).toHaveAttribute('aria-expanded', 'false')

    const plain = scenario(offered([sarah], 'sa'))
    await open(page, 'graph', { ...plain, suggests: false })
    await box(page).fill('sa')
    await expect(page.getByRole('button', { name: 'Next match' })).toBeEnabled()
    expect(await calls(page, 'TrailerSuggest')).toEqual([])
    await expect(list(page)).toHaveCount(0)
  })

  test('the list goes when the box loses the focus', async ({ page }) => {
    await open(page, 'graph', scenario(offered([sarah], 'sa')))
    await box(page).fill('sa')
    await expect(rows(page)).toHaveCount(2)
    await box(page).evaluate((e: HTMLElement) => e.blur())
    await expect(list(page)).toHaveCount(0)
    // And comes back with it.
    await box(page).focus()
    await expect(rows(page)).toHaveCount(2)
  })
})
