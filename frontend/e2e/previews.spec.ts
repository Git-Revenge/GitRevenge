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
    await expect(inside.locator('#far')).not.toHaveAttribute('src', /./)
    await expect(page.locator('.html .note')).toContainText('Shown without 4 parts that would run or load something and 1 picture from the web.')
  })
})
