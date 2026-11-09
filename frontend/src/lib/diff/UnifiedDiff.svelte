<script lang="ts">
  import { SvelteSet } from 'svelte/reactivity'
  import { highlightLines, languageFor, type Token } from './highlight'
  import { applyChanges, diffLines, splitLines, toHunks, type Hunk } from './lineDiff'
  import Icon from '../ui/Icon.svelte'
  import { wrap } from './wrap.svelte'

  /** What the hunk and line buttons do, and how the result is written. */
  export type Actions = {
    /** Label for the apply action, e.g. "Stage". */
    apply?: { label: string; run: (content: string) => Promise<void> }
    /** Label for the revert action, e.g. "Unstage" or "Discard". */
    revert?: { label: string; danger?: boolean; run: (content: string) => Promise<void> }
  }

  let {
    path,
    oldText,
    newText,
    actions = {},
  }: { path: string; oldText: string; newText: string; actions?: Actions } = $props()

  let oldLines = $derived(splitLines(oldText))
  let newLines = $derived(splitLines(newText))
  let ops = $derived(diffLines(oldLines, newLines))
  let hunks = $derived(toHunks(ops))

  let oldTokens = $state<Token[][]>([])
  let newTokens = $state<Token[][]>([])
  $effect(() => {
    const [o, n] = [oldText, newText]
    oldTokens = highlightLines(o, null)
    newTokens = highlightLines(n, null)
    void languageFor(path).then((lang) => {
      if (!lang || o !== oldText || n !== newText) return
      oldTokens = highlightLines(o, lang)
      newTokens = highlightLines(n, lang)
    })
  })

  /** Selected change ops (by op index), for line-level actions. */
  const selected = new SvelteSet<number>()
  let anchor = -1
  let busy = $state(false)
  let confirming = $state<string | null>(null)

  // A new diff invalidates any selection.
  $effect(() => {
    void ops
    selected.clear()
    anchor = -1
    confirming = null
  })

  function toggle(i: number, e: MouseEvent) {
    if (!actions.apply && !actions.revert) return
    if (e.shiftKey && anchor >= 0) {
      const [a, b] = anchor < i ? [anchor, i] : [i, anchor]
      for (let j = a; j <= b; j++) if (ops[j].kind !== 'eq') selected.add(j)
    } else if (selected.has(i)) selected.delete(i)
    else selected.add(i)
    anchor = i
  }

  function inHunk(h: Hunk) {
    return (i: number) => i >= h.start && i < h.end
  }

  /** "Apply" applies the chosen changes; "revert" removes them and keeps the rest. */
  async function run(which: 'apply' | 'revert', chosen: (i: number) => boolean, key: string) {
    const action = actions[which]
    if (!action) return
    if (which === 'revert' && actions.revert?.danger && confirming !== key) {
      confirming = key
      return
    }
    confirming = null
    const content =
      which === 'apply'
        ? applyChanges(ops, oldLines, newLines, chosen)
        : applyChanges(ops, oldLines, newLines, (i) => !chosen(i))
    busy = true
    try {
      await action.run(content)
    } finally {
      busy = false
    }
  }

  const opIndexes = (h: Hunk) => Array.from({ length: h.end - h.start }, (_, k) => h.start + k)
  const header = (h: Hunk) => `@@ -${h.oldStart},${h.oldCount} +${h.newStart},${h.newCount} @@`
</script>

{#snippet line(tokens: Token[] | undefined, fallback: string)}
  {#if tokens}{#each tokens as t, k (k)}<span class={t.cls}>{t.text}</span>{/each}{:else}{fallback.replace(
      /\r?\n$/,
      '',
    )}{/if}
{/snippet}

<div class="unified">
  {#if hunks.length === 0}
    <p class="muted"><Icon name="diff" size={24} wash="teal" />No changes.</p>
  {/if}
  {#each hunks as h (h.start)}
    <section class="hunk">
      <header>
        <span class="mono">{header(h)}</span>
        {#if actions.apply}
          <button class="btn sm secondary" disabled={busy} onclick={() => run('apply', inHunk(h), 'h' + h.start)}>{actions.apply.label} hunk</button>
        {/if}
        {#if actions.revert}
          <button
            class={['btn sm', actions.revert.danger && confirming === 'h' + h.start ? 'danger' : 'quiet']}
            disabled={busy}
            onclick={() => run('revert', inHunk(h), 'h' + h.start)}
            >{confirming === 'h' + h.start ? `Really ${actions.revert.label.toLowerCase()}?` : `${actions.revert.label} hunk`}</button
          >
        {/if}
      </header>
      <table class:wrap={wrap.on}>
        <tbody>
          {#each opIndexes(h) as i (i)}
            {@const op = ops[i]}
            <tr class={op.kind} class:sel={selected.has(i)} onclick={(e) => op.kind !== 'eq' && toggle(i, e)}>
              <td class="num">{op.oldIndex >= 0 ? op.oldIndex + 1 : ''}</td>
              <td class="num">{op.newIndex >= 0 ? op.newIndex + 1 : ''}</td>
              <td class="sign">{op.kind === 'add' ? '+' : op.kind === 'del' ? '-' : ' '}</td>
              <td class="code"
                >{#if op.kind === 'del'}{@render line(oldTokens[op.oldIndex], oldLines[op.oldIndex])}{:else}{@render line(
                    newTokens[op.newIndex],
                    newLines[op.newIndex],
                  )}{/if}</td
              >
            </tr>
          {/each}
        </tbody>
      </table>
    </section>
  {/each}

  {#if selected.size > 0}
    <footer>
      <span>{selected.size} {selected.size === 1 ? 'line' : 'lines'} selected</span>
      {#if actions.apply}
        <button class="btn sm secondary" disabled={busy} onclick={() => run('apply', (i) => selected.has(i), 'sel')}>{actions.apply.label} lines</button>
      {/if}
      {#if actions.revert}
        <button
          class={['btn sm', actions.revert.danger && confirming === 'sel' ? 'danger' : 'quiet']}
          disabled={busy}
          onclick={() => run('revert', (i) => selected.has(i), 'sel')}
          >{confirming === 'sel' ? `Really ${actions.revert.label.toLowerCase()}?` : `${actions.revert.label} lines`}</button
        >
      {/if}
      <button class="btn sm text" onclick={() => selected.clear()}>Clear</button>
    </footer>
  {/if}
</div>

<style>
  .unified {
    font-family: var(--font-mono);
    font-size: 13px;
    line-height: 1.6;
    tab-size: 4;
  }

  .hunk {
    margin-bottom: 10px;
  }

  header {
    position: sticky;
    top: 0;
    z-index: 1;
    display: flex;
    gap: 6px;
    align-items: center;
    min-height: 32px;
    padding: 3px 8px 3px 14px;
    background: var(--paper-warm);
    border-bottom: 1px solid var(--paper-edge);
    color: var(--accent-b);
  }

  header .mono {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  table {
    border-collapse: collapse;
    width: 100%;
  }

  td {
    padding: 0 8px;
    white-space: pre;
    vertical-align: top;
  }

  .num {
    width: 1%;
    text-align: right;
    color: var(--ink-faint);
    user-select: none;
  }

  .sign {
    width: 1%;
    padding: 0 4px;
    user-select: none;
  }

  .code {
    width: 100%;
  }

  /* Wrapped lines keep their numbers and sign on the first visual line (cells align top). */
  .wrap .code {
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }

  tr.add,
  tr.del {
    cursor: pointer;
  }

  tr.add {
    background: var(--success-wash);
  }

  tr.del {
    background: var(--danger-wash);
  }

  tr.add .sign {
    color: var(--success);
  }

  tr.del .sign {
    color: var(--danger);
  }

  tr.sel {
    outline: 1px solid var(--link);
    outline-offset: -1px;
    background-image: linear-gradient(var(--selection), var(--selection));
  }

  footer {
    position: sticky;
    bottom: 0;
    display: flex;
    gap: 8px;
    align-items: center;
    padding: 8px 12px;
    background: var(--paper-warm);
    border-top: 1px solid var(--paper-edge);
    font-family: var(--font-serif);
    font-size: 14px;
  }

  footer span {
    flex: 1;
  }

  .muted {
    display: flex;
    align-items: center;
    gap: 12px;
    margin: 0;
    padding: 10px 14px;
    color: var(--ink-mute);
    font-family: var(--font-serif);
    font-style: italic;
  }
</style>
