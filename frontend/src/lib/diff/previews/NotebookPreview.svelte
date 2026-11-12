<script lang="ts">
  // A Jupyter notebook, read-only: its Markdown cells rendered, its code cells with the outputs
  // the notebook stored. Nothing is run, and nothing from an output is put in the page as HTML:
  // an HTML table (a pandas DataFrame, say) is read into a table of our own, and anything else
  // in HTML shows as the plain text stored beside it.
  import Markdown from '../../Markdown.svelte'
  import { attachmentSrc, readNotebook, type Cell, type Notebook, type Output } from './notebook'
  import { resolveInRepo } from './paths'
  import type { PreviewProps } from './registry'

  let { path, text, loadImage, openFile }: PreviewProps = $props()

  let parsed = $derived.by((): { book: Notebook | null; error: string } => {
    try {
      return { book: readNotebook(text), error: '' }
    } catch (e) {
      return { book: null, error: e instanceof Error ? e.message : String(e) }
    }
  })

  function imagesFor(cell: Cell) {
    return async (src: string): Promise<string> => {
      if (cell.kind === 'markdown') {
        const attached = attachmentSrc(src, cell.attachments)
        if (attached) return attached
      }
      const file = resolveInRepo(path, src)
      return file ? loadImage(file) : ''
    }
  }

  function onLink(href: string) {
    const file = resolveInRepo(path, href)
    if (file) openFile(file)
  }

  /** An HTML output that is only a table, read into rows of text; null for anything else. The
   * browser's own parser reads it, which loads nothing and runs nothing. */
  function tableOf(html: string): { head: string[][]; body: string[][] } | null {
    const doc = new DOMParser().parseFromString(html, 'text/html')
    const tables = doc.querySelectorAll('table')
    if (tables.length !== 1) return null
    const table = tables[0]
    const rows = (sel: string) => [...table.querySelectorAll(sel)].map((tr) => [...tr.querySelectorAll('th, td')].map((c) => (c.textContent ?? '').trim()))
    const head = rows('thead tr')
    const body = rows('tbody tr')
    const all = head.length || body.length ? null : rows('tr')
    return { head, body: all ?? body }
  }

  const prompt = (count: number | null) => `[${count ?? ' '}]`
  let lang = $derived(parsed.book?.language ? parsed.book.language[0].toUpperCase() + parsed.book.language.slice(1) : '')
</script>

{#snippet output(o: Output)}
  {#if o.kind === 'text'}
    <pre class="out" class:stderr={o.stream === 'stderr'}>{o.text}</pre>
  {:else if o.kind === 'image'}
    <img class="out-img" src={o.src} alt={o.alt} />
  {:else if o.kind === 'markdown'}
    <div class="out-md"><Markdown text={o.text} math /></div>
  {:else if o.kind === 'html'}
    {@const table = tableOf(o.html)}
    {#if table && (table.head.length || table.body.length)}
      <div class="out-table">
        <table>
          {#if table.head.length}<thead>{#each table.head as row, r (r)}<tr>{#each row as cell, c (c)}<th>{cell}</th>{/each}</tr>{/each}</thead>{/if}
          <tbody>{#each table.body as row, r (r)}<tr>{#each row as cell, c (c)}{#if c === 0 && table.head.length}<th scope="row">{cell}</th>{:else}<td>{cell}</td>{/if}{/each}</tr>{/each}</tbody>
        </table>
      </div>
    {:else if o.fallback}
      <pre class="out">{o.fallback}</pre>
    {:else}
      <p class="hidden-out">An HTML output, which isn't shown here.</p>
    {/if}
  {:else if o.kind === 'error'}
    <div class="out error" role="group" aria-label="Error: {o.name}">
      <p class="err-head"><b>{o.name}</b>{#if o.value}: {o.value}{/if}</p>
      {#if o.trace}<pre>{o.trace}</pre>{/if}
    </div>
  {:else}
    <p class="hidden-out">{o.what}.</p>
  {/if}
{/snippet}

<div class="notebook">
  {#if parsed.error}
    <p class="problem" role="alert">{parsed.error}</p>
  {:else if parsed.book}
    <p class="facts">{parsed.book.cells.length} {parsed.book.cells.length === 1 ? 'cell' : 'cells'}{lang ? ` · ${lang}` : ''} · as last saved: nothing here has been run</p>
    {#each parsed.book.cells as cell, i (i)}
      {#if cell.kind === 'markdown'}
        <section class="cell md"><Markdown text={cell.text} document math resolveImage={imagesFor(cell)} {onLink} /></section>
      {:else if cell.kind === 'code'}
        <section class="cell code" aria-label="Code cell {cell.count ?? ''}">
          <div class="in"><span class="prompt" aria-hidden="true">In {prompt(cell.count)}</span><pre class="source"><code>{cell.source}</code></pre></div>
          {#if cell.outputs.length}
            <div class="outs">
              <span class="prompt" aria-hidden="true">{cell.count !== null ? `Out ${prompt(cell.count)}` : ''}</span>
              <div class="out-list">{#each cell.outputs as o, k (k)}{@render output(o)}{/each}</div>
            </div>
          {/if}
        </section>
      {:else}
        <section class="cell raw"><pre>{cell.text}</pre></section>
      {/if}
    {/each}
  {/if}
</div>

<style>
  .notebook {
    box-sizing: border-box;
    width: 100%;
    display: flex;
    flex-direction: column;
    gap: 14px;
    padding: 20px 28px 40px;
    background: var(--paper);
  }

  .facts {
    margin: 0;
    font-family: var(--font-mono);
    font-size: 12px;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--ink-mute);
  }

  .cell {
    min-width: 0;
  }

  .in,
  .outs {
    display: grid;
    grid-template-columns: 64px minmax(0, 1fr);
    gap: 10px;
    align-items: start;
  }

  .outs {
    margin-top: 6px;
  }

  .prompt {
    padding-top: 9px;
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-mute);
    text-align: right;
    white-space: pre;
  }

  .source {
    margin: 0;
    padding: 8px 12px;
    overflow-x: auto;
    border-radius: var(--r-sm);
    background: var(--code-bg);
    color: var(--code-text);
    font-family: var(--font-mono);
    font-size: 13px;
    line-height: 1.5;
  }

  .out-list {
    display: flex;
    flex-direction: column;
    gap: 6px;
    min-width: 0;
  }

  .out,
  .raw pre {
    margin: 0;
    padding: 6px 2px;
    overflow-x: auto;
    font-family: var(--font-mono);
    font-size: 13px;
    line-height: 1.45;
    color: var(--ink);
    white-space: pre;
  }

  .out.stderr {
    padding: 6px 10px;
    border-radius: var(--r-sm);
    background: color-mix(in srgb, var(--warning) 12%, transparent);
  }

  .out.error {
    padding: 8px 12px;
    border-radius: var(--r-sm);
    background: color-mix(in srgb, var(--danger) 10%, transparent);
    white-space: normal;
  }

  .err-head {
    margin: 0 0 4px;
    color: var(--danger);
  }

  .out.error pre {
    margin: 0;
    overflow-x: auto;
    font-size: 12.5px;
  }

  /* A plot keeps the size it was saved at, up to the column's width. */
  .out-img {
    display: block;
    align-self: flex-start;
    max-width: 100%;
    height: auto;
    background: #fff;
  }

  .out-table {
    overflow-x: auto;
  }

  .out-table table {
    border-collapse: collapse;
    font-size: 13px;
  }

  .out-table th,
  .out-table td {
    padding: 4px 10px;
    border-bottom: 1px solid var(--hairline);
    text-align: right;
    white-space: nowrap;
  }

  .out-table thead th {
    border-bottom: 1px solid var(--paper-edge);
    font-weight: 600;
  }

  .hidden-out {
    margin: 0;
    font-size: 13px;
    font-style: italic;
    color: var(--ink-mute);
  }

  .problem {
    margin: 0;
    color: var(--danger);
  }
</style>
