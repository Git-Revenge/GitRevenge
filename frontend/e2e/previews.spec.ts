import { expect, test } from '@playwright/test'
import { open } from './support'

/** A file's bytes as base64, as the backend sends them. */
const b64 = (bytes: Uint8Array) => btoa(String.fromCharCode(...bytes))

/** A real one-second 440 Hz tone, 8 kHz mono 16-bit WAV: something the player can decode. */
function tone(): Uint8Array {
  const rate = 8000
  const out = new Uint8Array(44 + rate * 2)
  const v = new DataView(out.buffer)
  const ascii = (at: number, s: string) => [...s].forEach((c, i) => v.setUint8(at + i, c.charCodeAt(0)))
  ascii(0, 'RIFF')
  v.setUint32(4, 36 + rate * 2, true)
  ascii(8, 'WAVEfmt ')
  v.setUint32(16, 16, true)
  v.setUint16(20, 1, true)
  v.setUint16(22, 1, true)
  v.setUint32(24, rate, true)
  v.setUint32(28, rate * 2, true)
  v.setUint16(32, 2, true)
  v.setUint16(34, 16, true)
  ascii(36, 'data')
  v.setUint32(40, rate * 2, true)
  for (let i = 0; i < rate; i++) v.setInt16(44 + i * 2, Math.round(8000 * Math.sin((2 * Math.PI * 440 * i) / rate)), true)
  return out
}

// #162: audio and video play from the repository.
test.describe('audio and video', () => {
  test('a sound file gets a player, and says how long it is', async ({ page, browserName }) => {
    await open(page, 'markdownPreview', { responses: { File: { path: 'sounds/ping.wav', text: '', data: b64(tone()) } } })
    const player = page.locator('audio')
    await expect(player).toHaveAttribute('controls', '')
    await expect(player).toHaveAttribute('src', /^blob:/)
    await expect(page.locator('.media .meta')).toContainText('16 KB')
    // How long it is comes from decoding it, which rests on the system's codecs: Chromium's are
    // the same everywhere.
    if (browserName === 'chromium') await expect(page.locator('.media .meta')).toContainText('0:01')
    // Nothing plays until asked.
    expect(await player.evaluate((a: HTMLAudioElement) => a.paused)).toBe(true)
  })

  test('says so when the file can’t be decoded, instead of a dead player', async ({ page }) => {
    await open(page, 'markdownPreview', { responses: { File: { path: 'clips/broken.mp4', text: '', data: b64(new Uint8Array([1, 2, 3, 4, 5, 6, 7, 8])) } } })
    await expect(page.getByText("This video can't be played here")).toBeVisible()
    await expect(page.locator('video')).toHaveCount(0)
  })

  test('says what went wrong when the bytes can’t be read', async ({ page }) => {
    await open(page, 'markdownPreview', { responses: { File: { path: 'clips/gone.webm', text: '', data: '' } } })
    await expect(page.getByRole('alert')).toContainText('The file is empty or missing.')
  })
})

// #160: an HTML file shows as its page, and nothing on it runs or reaches the network.
test.describe('HTML pages', () => {
  const PIXEL = 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=='
  const page1 = [
    '<!doctype html><html><head><title>Docs</title>',
    '<meta http-equiv="refresh" content="0; url=https://example.com/away">',
    '<link rel="stylesheet" href="https://example.com/site.css">',
    '<style>h1 { color: rgb(200, 0, 0) }</style>',
    '<script>document.title = "ran"; fetch("https://example.com/beacon")</script>',
    '</head><body>',
    '<h1 onclick="alert(1)">Hello</h1>',
    '<p>Read <a href="https://example.com/more">the guide</a> or <a href="#top">go up</a>.</p>',
    '<img id="local" src="img/logo.png" alt="our logo">',
    '<img id="far" src="https://example.com/track.gif" alt="a tracker">',
    '<iframe src="https://example.com/embed"></iframe>',
    '<form action="https://example.com/post"><input name="q"></form>',
    '</body></html>',
  ].join('\n')
  const scenario = { responses: { File: { path: 'site/index.html', text: page1 }, Images: { 'site/img/logo.png': PIXEL } } }

  test('shows the page in a frame that allows nothing', async ({ page }) => {
    const requests: string[] = []
    page.on('request', (r) => {
      if (r.url().includes('example.com')) requests.push(r.url())
    })
    await open(page, 'markdownPreview', scenario)
    const frame = page.locator('iframe')
    await expect(frame).toHaveAttribute('sandbox', '')
    const inside = page.frameLocator('iframe')
    await expect(inside.getByRole('heading', { name: 'Hello' })).toBeVisible()
    // The page's own styles apply.
    await expect(inside.getByRole('heading', { name: 'Hello' })).toHaveCSS('color', 'rgb(200, 0, 0)')
    // Its script never ran, and nothing went to the network.
    await page.waitForTimeout(500)
    expect(requests).toEqual([])
    const doc = (await frame.getAttribute('srcdoc')) ?? ''
    expect(doc).not.toContain('<script')
    expect(doc).not.toContain('onclick')
    expect(doc).not.toContain('example.com/away')
    expect(doc).toContain('Content-Security-Policy')
  })

  test('links keep their words and lose their address, so the frame goes nowhere', async ({ page }) => {
    await open(page, 'markdownPreview', scenario)
    const link = page.frameLocator('iframe').getByText('the guide')
    await expect(link).toHaveAttribute('title', 'https://example.com/more')
    await expect(link).not.toHaveAttribute('href', /./)
  })

  test('a picture in the repository shows; one on the web is left out', async ({ page }) => {
    await open(page, 'markdownPreview', scenario)
    const inside = page.frameLocator('iframe')
    await expect(inside.locator('#local')).toHaveAttribute('src', PIXEL)
    // One on the web becomes its alt text, with what it was in the tooltip.
    await expect(inside.locator('#far')).toHaveText('[a tracker]')
    await expect(inside.locator('#far')).toHaveAttribute('title', "https://example.com/track.gif, which isn't fetched")
    await expect(inside.locator('img#far')).toHaveCount(0)
    await expect(page.locator('.html .note')).toContainText('Shown without 4 parts that would run or load something and 1 picture from the web.')
  })
})

// #158: a notebook shows its cells and stored outputs, and nothing in it runs.
test.describe('Jupyter notebooks', () => {
  const PIXEL = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=='
  const frame = '<div><style scoped>.dataframe { color: red }</style><table class="dataframe"><thead><tr><th></th><th>name</th><th>score</th></tr></thead><tbody><tr><th>0</th><td>Ana</td><td>3</td></tr><tr><th>1</th><td>Bo</td><td>5</td></tr></tbody></table></div>'
  const book = {
    nbformat: 4,
    nbformat_minor: 5,
    metadata: { language_info: { name: 'python' } },
    cells: [
      { cell_type: 'markdown', source: ['# Scores\n', 'A *short* look, with $x^2$ and a plot: ![plot](attachment:p.png)'], attachments: { 'p.png': { 'image/png': PIXEL } } },
      { cell_type: 'code', execution_count: 1, source: 'df = load()\ndf', outputs: [{ output_type: 'execute_result', execution_count: 1, data: { 'text/html': frame, 'text/plain': '  name  score\n0  Ana      3' } }] },
      { cell_type: 'code', execution_count: 2, source: 'print("hi")', outputs: [{ output_type: 'stream', name: 'stdout', text: 'hi\n' }] },
      { cell_type: 'code', execution_count: 3, source: 'plot()', outputs: [{ output_type: 'display_data', data: { 'image/png': PIXEL, 'text/plain': '<Figure>' } }] },
      { cell_type: 'code', execution_count: 4, source: '1/0', outputs: [{ output_type: 'error', ename: 'ZeroDivisionError', evalue: 'division by zero', traceback: ['\u001b[0;31mZeroDivisionError\u001b[0m: division by zero'] }] },
      { cell_type: 'code', execution_count: 5, source: 'widget()', outputs: [{ output_type: 'display_data', data: { 'application/javascript': 'window.ran = true', 'application/vnd.jupyter.widget-view+json': {} } }] },
      { cell_type: 'code', execution_count: 6, source: 'html()', outputs: [{ output_type: 'display_data', data: { 'text/html': '<script>window.ran = true</script><p>a page</p>', 'text/plain': '<IPython.core.display.HTML object>' } }] },
    ],
  }

  test('renders its Markdown, code and stored outputs, in order', async ({ page }) => {
    await open(page, 'markdownPreview', { responses: { File: { path: 'analysis/scores.ipynb', text: JSON.stringify(book) } } })
    await expect(page.locator('.notebook .facts')).toContainText('7 cells · Python · as last saved')
    await expect(page.getByRole('heading', { name: 'Scores' })).toBeVisible()
    await expect(page.locator('.notebook em')).toHaveText('short')
    // The cell's attached picture.
    await expect(page.locator('.cell.md img')).toHaveAttribute('src', `data:image/png;base64,${PIXEL}`)
    // A DataFrame is a real table, read out of its HTML, with none of the HTML's styles.
    const table = page.locator('.out-table table')
    await expect(table.locator('thead th')).toHaveText(['', 'name', 'score'])
    await expect(table.locator('tbody tr').nth(1)).toHaveText(/1\s*Bo\s*5/)
    await expect(page.locator('.notebook style')).toHaveCount(0)
    await expect(page.locator('.out').filter({ hasText: 'hi' })).toBeVisible()
    await expect(page.locator('.out-img')).toHaveAttribute('src', `data:image/png;base64,${PIXEL}`)
    const error = page.getByRole('group', { name: 'Error: ZeroDivisionError' })
    await expect(error).toContainText('ZeroDivisionError: division by zero')
    await expect(error).not.toContainText('[0;31m')
    await expect(page.locator('.prompt').first()).toHaveText('In [1]')
  })

  test('never runs a script from an output', async ({ page }) => {
    await open(page, 'markdownPreview', { responses: { File: { path: 'analysis/scores.ipynb', text: JSON.stringify(book) } } })
    await expect(page.getByText('An interactive output, which only runs in Jupyter.')).toBeVisible()
    // An HTML output that isn't a table shows as the text stored beside it.
    await expect(page.locator('.out').filter({ hasText: '<IPython.core.display.HTML object>' })).toBeVisible()
    expect(await page.evaluate(() => (window as unknown as { ran?: boolean }).ran)).toBeUndefined()
    await expect(page.locator('.notebook script')).toHaveCount(0)
  })

  test('says so when the file isn’t a notebook it can read', async ({ page }) => {
    await open(page, 'markdownPreview', { responses: { File: { path: 'broken.ipynb', text: '{"cells": ' } } })
    await expect(page.getByRole('alert')).toHaveText("This isn't valid JSON, so it can't be read as a notebook.")
  })
})

// #159: AsciiDoc and reStructuredText render as documents, read into Markdown.
test.describe('AsciiDoc and reStructuredText', () => {
  const PIXEL = 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=='

  test('an AsciiDoc page renders its headings, marks, lists, code, notes and tables', async ({ page }) => {
    const adoc = [
      '= Getting Started',
      ':project: Git Revenge',
      '',
      'Welcome to *{project}*, the _friendly_ client. See https://example.com[the site].',
      '',
      'image::img/shot.png[A screenshot]',
      '',
      '== Install',
      '',
      '* Download it',
      '* Run it',
      '',
      '[source,bash]',
      '----',
      'make install',
      '----',
      '',
      'WARNING: It needs git 2.40.',
      '',
      '|===',
      '|Platform |Status',
      '',
      '|Linux |Yes',
      '|===',
      '',
      '++++',
      '<script>window.ran = true</script>',
      '++++',
    ].join('\n')
    await open(page, 'markdownPreview', { responses: { File: { path: 'docs/start.adoc', text: adoc }, Images: { 'docs/img/shot.png': PIXEL } } })
    await expect(page.getByRole('heading', { level: 2, name: 'Getting Started' })).toBeVisible()
    await expect(page.getByRole('heading', { level: 3, name: 'Install' })).toBeVisible()
    await expect(page.locator('strong').first()).toHaveText('Git Revenge')
    await expect(page.locator('em').first()).toHaveText('friendly')
    await expect(page.getByRole('link', { name: 'the site' })).toHaveAttribute('href', 'https://example.com')
    await expect(page.getByRole('img', { name: 'A screenshot' })).toHaveAttribute('src', PIXEL)
    await expect(page.getByRole('listitem')).toHaveText(['Download it', 'Run it'])
    await expect(page.locator('pre code').first()).toHaveText('make install')
    await expect(page.locator('blockquote')).toContainText('Warning: It needs git 2.40.')
    await expect(page.getByRole('table').getByRole('columnheader')).toHaveText(['Platform', 'Status'])
    // A passthrough block shows as code: its script is text.
    await expect(page.locator('pre code').last()).toHaveText('<script>window.ran = true</script>')
    expect(await page.evaluate(() => (window as unknown as { ran?: boolean }).ran)).toBeUndefined()
  })

  test('a reStructuredText page renders its headings, links, code, notes and math', async ({ page }) => {
    const rst = [
      'Getting Started',
      '===============',
      '',
      'Welcome to **Git Revenge**. Read the guide_ or `the site <https://example.com>`_.',
      '',
      'Install',
      '-------',
      '',
      'Run this::',
      '',
      '    make install',
      '',
      '.. note:: It needs git 2.40.',
      '',
      'The area is :math:`\\pi r^2`.',
      '',
      '.. _guide: https://example.com/guide',
    ].join('\n')
    await open(page, 'markdownPreview', { responses: { File: { path: 'docs/start.rst', text: rst } } })
    await expect(page.getByRole('heading', { level: 2, name: 'Getting Started' })).toBeVisible()
    await expect(page.getByRole('heading', { level: 3, name: 'Install' })).toBeVisible()
    await expect(page.getByRole('link', { name: 'guide' })).toHaveAttribute('href', 'https://example.com/guide')
    await expect(page.getByRole('link', { name: 'the site' })).toHaveAttribute('href', 'https://example.com')
    await expect(page.getByText('Run this:', { exact: true })).toBeVisible()
    await expect(page.locator('pre code')).toHaveText('make install')
    await expect(page.locator('blockquote')).toContainText('Note: It needs git 2.40.')
    await expect(page.locator('.katex').first()).toBeVisible()
  })
})
