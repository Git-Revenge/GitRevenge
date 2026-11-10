import { expect, test } from '@playwright/test'
import { events, open } from './support'

// #613
test.describe('about dialog', () => {
  const built = { responses: { BuiltWith: { go: '1.27.3', wails: '2.17.0' } } }

  test('names the app, its version and what it is built with', async ({ page }) => {
    await open(page, 'about', built)
    const dialog = page.getByRole('dialog', { name: 'Git Revenge' })
    await expect(dialog).toContainText('Version 0.11.0')
    await expect(page.getByRole('button', { name: 'Done' })).toBeFocused()
    await expect(dialog.getByRole('listitem')).toHaveText([/Love.*Built for you with love and coffee/, /Go\s+1\.27\.3 · BSD-3-Clause/, /Svelte\s+\d+\.\d+\.\d+\S* · MIT/, /Wails\s+2\.17\.0 · MIT/])
  })

  test('opens every link in the browser, not in the app', async ({ page }) => {
    await open(page, 'about', built)
    for (const name of ['gitrevenge.com', 'risadams.com', 'Go', 'Svelte', 'Wails']) await page.getByRole('link', { name, exact: true }).click()
    await page.getByRole('button', { name: 'Sponsor on GitHub' }).click()
    expect(await events(page, 'open-url')).toEqual([['https://gitrevenge.com'], ['https://risadams.com'], ['https://go.dev'], ['https://svelte.dev'], ['https://wails.io'], ['https://github.com/sponsors/risadams']])
    expect(page.url()).toContain('/e2e/harness.html')
  })

  test('still credits each project when its version cannot be read', async ({ page }) => {
    await open(page, 'about', { responses: { BuiltWith: { go: '', wails: '' } } })
    await expect(page.getByRole('listitem').filter({ hasText: 'Wails' })).toContainText(/Wails\s+MIT/)
  })

  test('closes from Done', async ({ page }) => {
    await open(page, 'about', built)
    await page.getByRole('button', { name: 'Done' }).click()
    expect(await events(page, 'close')).toHaveLength(1)
  })
})
