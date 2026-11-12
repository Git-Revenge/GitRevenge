import { describe, expect, it } from 'vitest'
import { clock, mediaBlob, mediaOf } from './media'

describe('mediaOf', () => {
  it('knows common audio and video by extension, in any case', () => {
    expect(mediaOf('sounds/ping.MP3')).toEqual({ kind: 'audio', mime: 'audio/mpeg' })
    expect(mediaOf('a/b/clip.webm')).toEqual({ kind: 'video', mime: 'video/webm' })
    expect(mediaOf('demo.mp4')?.kind).toBe('video')
    expect(mediaOf('voice.opus')?.kind).toBe('audio')
  })

  it('is null for anything else', () => {
    expect(mediaOf('README.md')).toBeNull()
    expect(mediaOf('mp4')).toBeNull()
    expect(mediaOf('archive.mp4.zip')).toBeNull()
  })
})

describe('mediaBlob', () => {
  it('turns base64 back into the same bytes, with the type given', async () => {
    const blob = mediaBlob(btoa('ID3\u0000ÿ'), 'audio/mpeg')
    expect(blob.type).toBe('audio/mpeg')
    expect([...new Uint8Array(await blob.arrayBuffer())]).toEqual([73, 68, 51, 0, 255])
  })
})

describe('clock', () => {
  it('reads as a player shows time', () => {
    expect(clock(7.4)).toBe('0:07')
    expect(clock(205)).toBe('3:25')
    expect(clock(3729)).toBe('1:02:09')
  })

  it('says nothing for a length the player doesn’t know', () => {
    expect(clock(Infinity)).toBe('')
    expect(clock(NaN)).toBe('')
  })
})
