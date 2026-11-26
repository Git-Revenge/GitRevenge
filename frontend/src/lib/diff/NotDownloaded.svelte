<script lang="ts">
  // A file a partial clone hasn't downloaded yet (#1003). One file downloads by itself, as
  // the rule says: this starts it, says so, has Stop, and asks the view to load again when
  // it's in. Nothing else waits for it.
  import { onDestroy } from 'svelte'
  import { CancelDownload, DownloadFiles } from '../../../wailsjs/go/app/App.js'
  import { errorText } from '../errors'
  import Busy from '../ui/Busy.svelte'

  let {
    path,
    ids,
    about,
    onDone,
    onStopped,
    inline = false,
  }: {
    path: string
    /** Sits in a dialog or a card, rather than filling a view. */
    inline?: boolean
    /** The versions to download, as the backend named them. */
    ids: string[]
    /** What they are, in words ("the files both sides changed"); a file's versions when unset. */
    about?: string
    /** Called once they're in, to load the file again. */
    onDone: () => void
    /** Called when the download stops or fails, for a view that can go on without them. */
    onStopped?: () => void
  } = $props()

  let token = ''
  let phase = $state<'downloading' | 'stopped' | 'failed'>('downloading')
  let error = $state('')

  async function download(list: string[]) {
    const mine = `${Date.now()}-${Math.random().toString(36).slice(2)}`
    token = mine
    phase = 'downloading'
    error = ''
    try {
      await DownloadFiles(path, mine, list)
      if (token === mine) onDone()
    } catch (err) {
      if (token !== mine) return
      const text = errorText(err)
      // A stopped download is the person's choice, not a failure.
      if (/cancel|stopp?ed|killed|interrupt/i.test(text)) {
        phase = 'stopped'
      } else {
        phase = 'failed'
        error = text
      }
      onStopped?.()
    }
  }

  function stop() {
    phase = 'stopped'
    void CancelDownload(token)
    onStopped?.()
  }

  // Keyed on the list's contents: a view asking again with the same list (the rebase editor,
  // as commits move) doesn't start a second download.
  let wanted = $derived(ids.join(' '))
  $effect(() => {
    void download(wanted.split(' '))
  })

  onDestroy(() => {
    // Moving to another file leaves this one's download to finish: it's still wanted next
    // time. Only Stop stops it.
    token = ''
  })

  let what = $derived(about ?? `${ids.length === 1 ? 'this version' : 'both versions'} of the file`)
</script>

<div class="not-downloaded" class:inline role="status">
  {#if phase === 'downloading'}
    <p class="line"><Busy /> This partial clone hasn’t got {what} yet. Downloading…</p>
    <button class="btn secondary" onclick={stop}>Stop</button>
  {:else if phase === 'stopped'}
    <p class="line">The download was stopped. This partial clone hasn’t got {what} yet.</p>
    <button class="btn secondary" onclick={() => void download(wanted.split(' '))}>Download</button>
  {:else}
    <p class="line">The download failed: {error}</p>
    <button class="btn secondary" onclick={() => void download(wanted.split(' '))}>Try again</button>
  {/if}
</div>

<style>
  .not-downloaded {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 10px;
    padding: 24px;
    color: var(--code-text, var(--ink));
  }

  .not-downloaded.inline {
    padding: 0;
    color: inherit;
  }

  .line {
    display: flex;
    align-items: center;
    gap: 8px;
    margin: 0;
    font-size: 14px;
  }
</style>
