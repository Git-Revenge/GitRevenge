<script lang="ts">
  // The status bar's word on the trailer index: every commit's trailers are read once, in
  // the background, and on a long history that takes a while the first time. This says so
  // for the repository on screen, with how far it has got, and says nothing once it's done.
  import { TrailerStatus as ask } from '../../wailsjs/go/app/App.js'
  import { onBackend } from './events'
  import { progressPart, progressText, type TrailerStatus } from './trailerProgress'

  let { path }: { path: string } = $props()

  /** How far each repository's read has got, by path, as the backend last said. */
  let known = $state<Record<string, TrailerStatus>>({})
  $effect(() => onBackend('trailers:progress', (p) => (known[p.path] = p)))

  // A read may have begun before this was listening: at launch, or in a tab that was
  // behind another.
  $effect(() => {
    const repo = path
    if (!repo || known[repo]) return
    void ask(repo)
      .then((p) => {
        if (!known[repo]) known[repo] = p
      })
      .catch(() => {})
  })

  let status = $derived(path ? known[path] : undefined)
  let reading = $derived(!!status?.reading)

  // A read of a few new commits is over before it could be read about. The line appears
  // only for one that's taking a moment.
  let shown = $state(false)
  $effect(() => {
    if (!reading) {
      shown = false
      return
    }
    const timer = setTimeout(() => (shown = true), 600)
    return () => clearTimeout(timer)
  })
</script>

{#if shown && status}
  <span class="reading" role="group" aria-label="Reading trailers">
    <span class="meta">Reading trailers</span>
    <progress aria-label="Reading trailers" max="1" value={status.total > 0 ? progressPart(status) : undefined}></progress>
    <span class="meta how">{progressText(status)}</span>
  </span>
{/if}

<style>
  .reading {
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 0;
  }

  .how {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    /* Numbers that keep their width, so the line doesn't shiver as they climb. */
    font-variant-numeric: tabular-nums;
  }

  progress {
    flex: none;
    width: 96px;
    height: 4px;
    border: 0;
    border-radius: 2px;
    background: var(--paper-edge);
    appearance: none;
    overflow: hidden;
  }

  progress::-webkit-progress-bar {
    background: var(--paper-edge);
  }

  progress::-webkit-progress-value {
    background: var(--accent);
    transition: width 200ms ease-out;
  }

  progress::-moz-progress-bar {
    background: var(--accent);
  }

  @media (prefers-reduced-motion: reduce) {
    progress::-webkit-progress-value {
      transition: none;
    }
  }
</style>
