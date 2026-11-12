<script lang="ts">
  // An HTML file shown as its page, in a frame sandboxed to the full: no scripts, no forms, no
  // pop-ups, no navigating, and a separate origin that can't reach the app. What's put in it has
  // also been cleaned (see sandboxedPage), so it loads nothing from the network.
  import { sandboxedPage } from './html'
  import type { PreviewProps } from './registry'

  let { path, text, loadImage }: PreviewProps = $props()

  let page = $state<{ html: string; removed: number; remote: number } | null>(null)
  let error = $state('')

  $effect(() => {
    const t = text
    const p = path
    let cancelled = false
    page = null
    error = ''
    sandboxedPage(t, p, loadImage)
      .then((res) => {
        if (!cancelled) page = res
      })
      .catch((e: unknown) => {
        if (!cancelled) error = e instanceof Error ? e.message : String(e)
      })
    return () => {
      cancelled = true
    }
  })

  /** What was held back, said once above the page. */
  let note = $derived.by(() => {
    if (!page) return ''
    const parts = []
    if (page.removed) parts.push(`${page.removed} ${page.removed === 1 ? 'part that would run or load something' : 'parts that would run or load something'}`)
    if (page.remote) parts.push(`${page.remote} ${page.remote === 1 ? 'picture' : 'pictures'} from the web`)
    return parts.length ? `Shown without ${parts.join(' and ')}. Nothing on this page runs, and it loads nothing from the network.` : 'Nothing on this page runs, and it loads nothing from the network.'
  })
</script>

<div class="html">
  {#if error}
    <p class="error" role="alert">Couldn't show this page: {error}</p>
  {:else if page}
    <p class="note">{note}</p>
    <!-- sandbox with no allowances: no scripts, forms, pop-ups or navigation, and an origin of its own. -->
    <iframe title="Page preview of {path}" sandbox="" referrerpolicy="no-referrer" srcdoc={page.html}></iframe>
  {/if}
</div>

<style>
  .html {
    display: flex;
    flex-direction: column;
    gap: 10px;
    padding: 16px 20px 24px;
    background: var(--paper);
  }

  .note {
    margin: 0;
    font-size: 13px;
    font-style: italic;
    color: var(--ink-mute);
  }

  /* A page is drawn on white, as a browser would: it brings its own colours. */
  iframe {
    display: block;
    width: 100%;
    height: max(420px, calc(100vh - 200px));
    border: 1px solid var(--paper-edge);
    border-radius: var(--r-sm);
    background: #fff;
  }

  .error {
    margin: 0;
    color: var(--danger);
  }
</style>
