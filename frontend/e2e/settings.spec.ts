import { expect, test } from '@playwright/test'
import { calls, open } from './support'

// #808: a repository says which authors are bots, by name or email.
test.describe("a repository's bot authors in settings", () => {
  const prefs = { theme: '', graphDensity: '', experimentalViews: [], updateChecks: true, gitPath: '', gitInUse: '/usr/bin/git', cloneDir: '', zoom: 100 }
  const repo = { kinds: [], hide: [], hideKinds: [], showKinds: [], foldableKinds: ['bot', 'docs'], botAuthors: ['Release Bot'], builtInBotAuthor: '*[bot]', cleanupOnPull: false, syncNotes: false }
  const saved = async (page: Parameters<typeof calls>[0]) => ((await calls(page, 'SetRepoPreferences'))[0]?.[1] as { botAuthors: string[] } | undefined)?.botAuthors

  test('shows the patterns in use and says which one is built in', async ({ page }) => {
    await open(page, 'settings', { responses: { Preferences: prefs, RepoPreferences: repo } })
    const field = page.getByRole('textbox', { name: 'Bot authors' })
    await expect(field).toHaveValue('Release Bot')
    await expect(field).toHaveAccessibleDescription(/A name ending in \[bot\] is always a bot/)
  })

  test('saves patterns parted by commas, keeping the spaces inside a name', async ({ page }) => {
    await open(page, 'settings', { responses: { Preferences: prefs, RepoPreferences: repo } })
    await page.getByRole('textbox', { name: 'Bot authors' }).fill(' Release Bot ,*@bots.example.com,, Deploy * ')
    await page.getByRole('button', { name: 'Save' }).click()
    await expect.poll(() => saved(page)).toEqual(['Release Bot', '*@bots.example.com', 'Deploy *'])
  })

  test('saves none when the field is emptied', async ({ page }) => {
    await open(page, 'settings', { responses: { Preferences: prefs, RepoPreferences: repo } })
    await page.getByRole('textbox', { name: 'Bot authors' }).fill('')
    await page.getByRole('button', { name: 'Save' }).click()
    await expect.poll(() => saved(page)).toEqual([])
  })
})
