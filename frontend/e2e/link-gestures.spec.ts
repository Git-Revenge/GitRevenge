import { expect, test, type Page } from '@playwright/test'
import { events, open } from './support'

// GHSA-8crh-7f69-q64m: a link in a repository's document is the repository's to write. Whatever
// is done to it, the app's window stays on the app, no second window opens, and only a web
// address ever reaches the browser.
const readme = [
  '# Links',
  '',
  '[web](https://example.com/page)',
  '[file](file:///C:/Windows/win.ini)',
  '[script](javascript:document.title=%22ran%22)',
  '[other](custom-scheme://do/something)',
  '[relative](docs/guide.md)',
].join('\n')

const show = (page: Page) => open(page, 'markdownPreview', { responses: { File: { path: 'README.md', text: readme } } })

/** Fails the test if anything opens a second window. */
function noNewWindows(page: Page): () => number {
  let opened = 0
  page.context().on('page', (p) => {
    opened++
    void p.close()
  })
  return () => opened
}

test.describe('links in a rendered document', () => {
  test('carry no address the web view could follow, except a web one', async ({ page }) => {
    await show(page)
    const href = (name: string) => page.getByRole('link', { name, exact: true }).getAttribute('href')
    expect(await href('web')).toBe('https://example.com/page')
    for (const name of ['file', 'script', 'other', 'relative']) expect(await href(name), name).toBeNull()
  })

  test('a click opens a web address in the browser, a relative one in the app, and nothing else', async ({ page }) => {
    await show(page)
    const windows = noNewWindows(page)
    for (const name of ['web', 'file', 'script', 'other', 'relative']) await page.getByRole('link', { name, exact: true }).click()
    expect(await events(page, 'open-url')).toEqual([['https://example.com/page']])
    expect(await events(page, 'openFile')).toEqual([['docs/guide.md']])
    expect(page.url()).toContain('/e2e/harness.html')
    expect(await page.title()).not.toBe('ran')
    expect(windows()).toBe(0)
  })

  test('the keyboard opens a link as a click does', async ({ page }) => {
    await show(page)
    await page.getByRole('link', { name: 'relative', exact: true }).focus()
    await page.keyboard.press('Enter')
    await page.getByRole('link', { name: 'web', exact: true }).focus()
    await page.keyboard.press('Enter')
    expect(await events(page, 'openFile')).toEqual([['docs/guide.md']])
    expect(await events(page, 'open-url')).toEqual([['https://example.com/page']])
  })

  test('a middle click or a modified click opens no window and leaves the app where it is', async ({ page }) => {
    await show(page)
    const windows = noNewWindows(page)
    for (const name of ['web', 'file', 'script', 'other', 'relative']) {
      const link = page.getByRole('link', { name, exact: true })
      await link.click({ button: 'middle' })
      await link.click({ modifiers: ['ControlOrMeta'] })
      await link.click({ modifiers: ['Shift'] })
    }
    await page.waitForTimeout(300)
    expect(windows()).toBe(0)
    expect(page.url()).toContain('/e2e/harness.html')
    expect(await page.title()).not.toBe('ran')
    // A web address still goes to the browser, once for each of the three; nothing else does.
    expect(await events(page, 'open-url')).toEqual([['https://example.com/page'], ['https://example.com/page'], ['https://example.com/page']])
  })

  test('a link can’t be dragged, and nothing dropped on the window is followed', async ({ page }) => {
    await show(page)
    const windows = noNewWindows(page)
    const started = await page.getByRole('link', { name: 'web', exact: true }).evaluate((a) => a.dispatchEvent(new DragEvent('dragstart', { bubbles: true, cancelable: true })))
    expect(started, 'dragstart was not cancelled').toBe(false)
    for (const type of ['dragover', 'drop']) {
      const allowed = await page.evaluate((t) => document.body.dispatchEvent(new DragEvent(t, { bubbles: true, cancelable: true })), type)
      expect(allowed, `${type} was not cancelled`).toBe(false)
    }
    expect(windows()).toBe(0)
    expect(page.url()).toContain('/e2e/harness.html')
  })
})
