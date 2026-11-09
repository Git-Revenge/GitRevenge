<script lang="ts">
  // A Mermaid diagram, drawn by mermaid at the strict security level (no scripts, no click
  // handlers, sanitised labels) with SVG-text labels, then shown through <img>: an image
  // never runs scripts or handles events, whatever the SVG holds. Mermaid loads on first use.
  import { theme } from '../theme.svelte'

  let { code }: { code: string } = $props()

  let src = $state('')
  let error = $state('')

  let seq = 0
  $effect(() => {
    const text = code
    const dark = theme.resolved === 'dark'
    const mine = ++seq
    src = ''
    error = ''
    void (async () => {
      try {
        const { default: mermaid } = await import('mermaid')
        mermaid.initialize({
          startOnLoad: false,
          securityLevel: 'strict',
          htmlLabels: false,
          theme: dark ? 'dark' : 'neutral',
          fontFamily: 'Fraunces Variable, Georgia, serif',
        })
        const { svg } = await mermaid.render(`mermaid-${crypto.randomUUID()}`, text)
        if (mine === seq) src = `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}`
      } catch (e) {
        if (mine === seq) error = e instanceof Error ? e.message : String(e)
      }
    })()
  })
</script>

<figure class="mermaid">
  {#if src}
    <img {src} alt="Mermaid diagram" />
  {:else if error}
    <p class="error" role="alert">Couldn't draw this diagram: {error}</p>
    <pre><code>{code}</code></pre>
  {:else}
    <p class="muted">Drawing…</p>
  {/if}
</figure>

<style>
  .mermaid {
    margin: 12px 0;
    overflow-x: auto;
  }

  img {
    max-width: 100%;
    height: auto;
  }

  .error {
    color: var(--danger);
    font-size: 13px;
  }

  pre {
    padding: 10px 12px;
    background: var(--paper-warm);
    border-radius: 4px;
    font-family: var(--font-mono);
    font-size: 13px;
    overflow-x: auto;
  }

  .muted {
    color: var(--ink-mute);
    font-style: italic;
  }
</style>
