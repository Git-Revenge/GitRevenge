import { expect, test, type Page } from '@playwright/test'
import { calls, events, open } from './support'

// A commit's place among the tags, in the commit panel's Details tab (#940).

/** A timeline row's height: in the repository view the first commit starts this far down. */
const ROW = 52

const person = { name: 'Ana Cruz', email: 'ana@example.com', time: 1793500000 }
const commit = { id: '0'.repeat(40), subject: 'Commit 0', body: '', trailers: [], parents: ['a'.repeat(40)], author: person, committer: person }
const identity = { name: 'A', email: 'a@example.com' }
const base = { Refs: [], CommitSignatures: [], CommitNotes: [], WorkingTree: { status: { files: [] }, identity, signing: { enabled: false } }, CommitDetail: { commit, files: [] }, CommitRow: 1 }

async function openCommit(page: Page, described: object) {
  await open(page, 'repoView', { graphRows: 3, responses: { ...base, DescribeCommit: described } })
  const box = (await page.locator('.scroller').boundingBox())!
  await page.mouse.click(box.x + 500, box.y + ROW + 26)
  await expect(page.getByRole('heading', { name: 'Commit 0' })).toBeVisible()
}
const details = (page: Page) => page.getByRole('tabpanel')

test('Details says which tag a commit comes after and the first that has it, and copies either', async ({ page }) => {
  await openCommit(page, { name: 'v1.1.0-12-gabc1234', after: 'v1.1.0', since: 12, firstIn: 'v1.2.0', contains: 'v1.2.0~3' })
  // Only Details asks: on the kernel it takes about a second.
  await expect(page.getByRole('tab', { name: 'Message' })).toHaveAttribute('aria-selected', 'true')
  expect(await calls(page, 'DescribeCommit')).toEqual([])

  await page.getByRole('tab', { name: 'Details' }).click()
  await expect(details(page)).toContainText('v1.1.0-12-gabc1234')
  await expect(details(page)).toContainText('12 commits after v1.1.0')
  await expect(details(page)).toContainText('v1.2.0')
  await expect(details(page)).toContainText('v1.2.0~3')
  expect(await calls(page, 'DescribeCommit')).toEqual([['/r', '0'.repeat(40)]])

  await details(page).getByRole('button', { name: 'Copy v1.1.0-12-gabc1234' }).click()
  await details(page).getByRole('button', { name: 'Copy v1.2.0' }).click()
  await expect.poll(() => events(page, 'copy')).toEqual([['v1.1.0-12-gabc1234'], ['v1.2.0']])
})

test('a tagged commit is named by its tag alone', async ({ page }) => {
  await openCommit(page, { name: 'v1.1.0', after: 'v1.1.0', since: 0, firstIn: 'v1.1.0', contains: 'v1.1.0' })
  await page.getByRole('tab', { name: 'Details' }).click()
  await expect(details(page)).toContainText('It’s the commit v1.1.0 tags.')
  // The command's own answer is the tag itself, so it isn't said twice.
  await expect(details(page).locator('.described .says[title]')).toHaveCount(0)
})

test('a commit with no tags around it', async ({ page }) => {
  await openCommit(page, { name: '', after: '', since: 0, firstIn: '', contains: '' })
  await page.getByRole('tab', { name: 'Details' }).click()
  await expect(details(page)).toContainText('No tag comes before it.')
  await expect(details(page)).toContainText('No tag has it yet.')
  await expect(details(page).getByRole('button', { name: /^Copy v/ })).toHaveCount(0)
})
