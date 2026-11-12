import { expect, test, type Page } from '@playwright/test'
import { branch, calls, open, pickView } from './support'

// #650: the density switch became a menu, so experimental views have somewhere to go.
test.describe('the graph view menu', () => {
  const scenario = { graphRows: 30, responses: { CommitSignatures: [], Refs: [branch('main', { head: true, branchKind: 'trunk' })], RepoPreferences: { hideKinds: [] } } }
  const button = (page: Page) => page.getByRole('button', { name: /^Graph view/ })
  const items = (page: Page) => page.getByRole('menuitemradio')

  test('shows the current view on its button, and lists the rows at both densities', async ({ page }) => {
    await open(page, 'graph', scenario)
    await expect(button(page)).toHaveAccessibleName('Graph view: Timeline')
    await expect(button(page)).toHaveAttribute('aria-expanded', 'false')
    await button(page).click()
    await expect(button(page)).toHaveAttribute('aria-expanded', 'true')
    await expect(items(page)).toHaveText(['Timeline', 'Compact'])
    await expect(page.getByRole('menuitemradio', { name: 'Timeline' })).toHaveAttribute('aria-checked', 'true')
    // Nothing experimental is switched on, so the menu has no such heading.
    await expect(page.getByRole('menu')).not.toContainText('experimental')
  })

  test('choosing Compact shows compact rows, saves that, and closes the menu', async ({ page }) => {
    await open(page, 'graph', scenario)
    await pickView(page, 'Compact')
    await expect(page.getByRole('menu')).toHaveCount(0)
    await expect(button(page)).toHaveAccessibleName('Graph view: Compact')
    await expect(button(page)).toBeFocused()
    // Compact is the table with named columns.
    await expect(page.locator('.columns')).toContainText('Message')
    expect(await calls(page, 'SetGraphDensity')).toEqual([['compact']])
    // The view itself didn't change: both are the graph's rows.
    expect(await calls(page, 'SetGraphView')).toEqual([])
  })

  test('works from the keyboard, and Escape gives the focus back', async ({ page }) => {
    await open(page, 'graph', scenario)
    await button(page).focus()
    await page.keyboard.press('Enter')
    await expect(page.getByRole('menuitemradio', { name: 'Timeline' })).toBeFocused()
    await page.keyboard.press('ArrowDown')
    await expect(page.getByRole('menuitemradio', { name: 'Compact' })).toBeFocused()
    await page.keyboard.press('ArrowDown')
    await expect(page.getByRole('menuitemradio', { name: 'Timeline' })).toBeFocused()
    await page.keyboard.press('End')
    await expect(page.getByRole('menuitemradio', { name: 'Compact' })).toBeFocused()
    await page.keyboard.press('Escape')
    await expect(page.getByRole('menu')).toHaveCount(0)
    await expect(button(page)).toBeFocused()
    expect(await calls(page, 'SetGraphDensity')).toEqual([])
    // Down on the button opens it too, and Enter picks.
    await page.keyboard.press('ArrowDown')
    await page.keyboard.press('ArrowDown')
    await page.keyboard.press('Enter')
    await expect(button(page)).toHaveAccessibleName('Graph view: Compact')
  })

  test('a press outside closes it without choosing', async ({ page }) => {
    await open(page, 'graph', scenario)
    await button(page).click()
    await page.locator('.scroller').click({ position: { x: 300, y: 200 } })
    await expect(page.getByRole('menu')).toHaveCount(0)
    await expect(button(page)).toHaveAccessibleName('Graph view: Timeline')
  })

  test('loses its word when the bar is short of room, and keeps its name', async ({ page }) => {
    await page.setViewportSize({ width: 1200, height: 700 })
    await open(page, 'graph', scenario)
    await expect(button(page)).toContainText('Timeline')
    await page.setViewportSize({ width: 500, height: 700 })
    await expect(button(page)).not.toContainText('Timeline')
    await expect(button(page)).toHaveAccessibleName('Graph view: Timeline')
  })
})
