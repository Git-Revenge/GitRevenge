import { expect, test } from '@playwright/test'
import { branch, open } from './support'

// These are about what a scrollbar's width does to the layout, and Playwright hides them.
test.use({ launchOptions: { ignoreDefaultArgs: ['--hide-scrollbars'] } })

// #514: the column header took height from the canvas, which overflowed for a frame. The page's
// scrollbar narrowed the graph below the width the header needs, and that went round for ever.
test('widening the graph past where the columns appear settles at once', async ({ page }) => {
  test.setTimeout(120_000)
  await page.setViewportSize({ width: 620, height: 600 })
  await open(page, 'graph', { graphRows: 200, responses: { CommitSignatures: [], Refs: [branch('main', { head: true, branchKind: 'trunk' })], RepoPreferences: { hideKinds: [] } } })
  await expect(page.getByText('200 commits')).toBeVisible()
  const flapping: number[] = []
  for (let width = 630; width <= 670; width++) {
    await page.setViewportSize({ width, height: 600 })
    const flips = await page.evaluate(
      () =>
        new Promise<number>((resolve) => {
          let last: boolean | null = null
          let flips = 0
          let frames = 0
          const tick = () => {
            const shown = !!document.querySelector('.columns')
            if (last !== null && shown !== last) flips++
            last = shown
            if (++frames < 20) requestAnimationFrame(tick)
            else resolve(flips)
          }
          requestAnimationFrame(tick)
        }),
    )
    // One flip is the columns arriving.
    if (flips > 1) flapping.push(width)
  }
  expect(flapping).toEqual([])
})
