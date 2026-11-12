<script lang="ts">
  // Audio or video from the repository, played by the web view's own player. The bytes are read
  // at the version shown and handed over as a Blob: nothing is fetched, and the player only
  // starts when asked.
  import { clock, mediaBlob, mediaOf } from './media'
  import type { PreviewProps } from './registry'

  let { path, loadData }: PreviewProps = $props()

  let media = $derived(mediaOf(path))
  let url = $state('')
  let size = $state(0)
  let error = $state('')
  /** What the player found out once it read the file. */
  let duration = $state(0)
  let frame = $state<{ w: number; h: number } | null>(null)
  /** The player couldn't decode it: the format, or its codec, isn't one this system plays. */
  let unplayable = $state(false)

  $effect(() => {
    const m = media
    let made = ''
    let cancelled = false
    url = ''
    error = ''
    unplayable = false
    duration = 0
    frame = null
    void (async () => {
      try {
        const b64 = loadData ? await loadData() : ''
        if (!b64) throw new Error('The file is empty or missing.')
        if (cancelled || !m) return
        const blob = mediaBlob(b64, m.mime)
        size = blob.size
        made = URL.createObjectURL(blob)
        url = made
      } catch (e) {
        if (!cancelled) error = e instanceof Error ? e.message : String(e)
      }
    })()
    return () => {
      cancelled = true
      if (made) URL.revokeObjectURL(made)
    }
  })

  function loaded(e: Event) {
    const el = e.currentTarget as HTMLMediaElement
    duration = el.duration
    if (el instanceof HTMLVideoElement && el.videoWidth) frame = { w: el.videoWidth, h: el.videoHeight }
  }

  const mb = (n: number) => (n >= 1 << 20 ? `${(n / (1 << 20)).toFixed(1)} MB` : `${Math.max(1, Math.round(n / 1024))} KB`)
  let facts = $derived([clock(duration), frame ? `${frame.w} × ${frame.h}` : '', size ? mb(size) : ''].filter(Boolean).join(' · '))
</script>

<div class="media">
  {#if error}
    <p class="error" role="alert">Couldn't open this file: {error}</p>
  {:else if !url}
    <p class="muted">Opening…</p>
  {:else if unplayable}
    <p class="muted">This {media?.kind} can't be played here: its format or codec isn't one this system's player knows.{size ? ` It's ${mb(size)}.` : ''}</p>
  {:else if media?.kind === 'video'}
    <!-- A file from the repository has the captions it has: there's no caption file to add. -->
    <!-- svelte-ignore a11y_media_has_caption -->
    <video src={url} controls preload="metadata" onloadedmetadata={loaded} onerror={() => (unplayable = true)} aria-label={path}></video>
  {:else}
    <audio src={url} controls preload="metadata" onloadedmetadata={loaded} onerror={() => (unplayable = true)} aria-label={path}></audio>
  {/if}
  {#if facts && !unplayable && !error}<p class="meta">{facts}</p>{/if}
</div>

<style>
  .media {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 10px;
    padding: 24px 32px 40px;
    background: var(--paper);
  }

  video {
    display: block;
    max-width: 100%;
    max-height: 70vh;
    border-radius: var(--r-sm);
    background: var(--code-bg);
  }

  audio {
    width: min(560px, 100%);
  }

  .meta {
    margin: 0;
    color: var(--ink-mute);
  }

  .error {
    margin: 0;
    color: var(--danger);
  }

  .muted {
    margin: 0;
    max-width: 60ch;
    color: var(--ink-mute);
    text-align: center;
  }
</style>
