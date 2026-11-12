/** Audio and video the preview can hand to the web view's own player, by extension. */
const TYPES: Record<string, { kind: 'audio' | 'video'; mime: string }> = {
  mp3: { kind: 'audio', mime: 'audio/mpeg' },
  wav: { kind: 'audio', mime: 'audio/wav' },
  ogg: { kind: 'audio', mime: 'audio/ogg' },
  oga: { kind: 'audio', mime: 'audio/ogg' },
  opus: { kind: 'audio', mime: 'audio/ogg; codecs=opus' },
  flac: { kind: 'audio', mime: 'audio/flac' },
  m4a: { kind: 'audio', mime: 'audio/mp4' },
  aac: { kind: 'audio', mime: 'audio/aac' },
  weba: { kind: 'audio', mime: 'audio/webm' },
  mp4: { kind: 'video', mime: 'video/mp4' },
  m4v: { kind: 'video', mime: 'video/mp4' },
  webm: { kind: 'video', mime: 'video/webm' },
  ogv: { kind: 'video', mime: 'video/ogg' },
  mov: { kind: 'video', mime: 'video/quicktime' },
}

/** What kind of media a file is, from its extension, or null when it isn't one. */
export function mediaOf(path: string): { kind: 'audio' | 'video'; mime: string } | null {
  const ext = /\.([a-z0-9]+)$/i.exec(path)?.[1].toLowerCase()
  return (ext && TYPES[ext]) || null
}

/** A file's bytes, given as base64, as a Blob the player can read. */
export function mediaBlob(base64: string, mime: string): Blob {
  const raw = atob(base64)
  const bytes = new Uint8Array(raw.length)
  for (let i = 0; i < raw.length; i++) bytes[i] = raw.charCodeAt(i)
  return new Blob([bytes], { type: mime })
}

/** A duration as a player shows it: "0:07", "3:25", "1:02:09". */
export function clock(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds < 0) return ''
  const s = Math.round(seconds)
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const pad = (n: number) => String(n).padStart(2, '0')
  return h ? `${h}:${pad(m)}:${pad(s % 60)}` : `${m}:${pad(s % 60)}`
}
