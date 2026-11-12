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
