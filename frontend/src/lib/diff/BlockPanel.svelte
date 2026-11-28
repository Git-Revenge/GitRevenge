<script lang="ts">
  // Rewind these lines (#1081): a block's own history beside the code. Only the versions that
  // changed those lines, newest first, each with what it did to them, from git log -L, which
  // follows them as they move down the file.
  import type { git } from '../../../wailsjs/go/models'
  import Icon from '../ui/Icon.svelte'
  import Loading from '../ui/Loading.svelte'
  import { formatTime } from '../graph/draw'

  let {
    from,
    to,
    versions,
    problem = '',
    fileVersions = 0,
    onShow,
    onClose,
    onReveal,
    onRestore,
  }: {
    /** The lines rewound, counting from 1, in the version shown. */
    from: number
    to: number
    /** Their versions, newest first; null while they're read. */
    versions: git.BlockVersion[] | null
    problem?: string
    /** How many versions the whole file has on the track, to say how few touched these. */
    fileVersions?: number
    /** Shows the whole file at a version, when the track has it. */
    onShow?: (commit: string) => void
    onClose: () => void
    onReveal?: (commit: string) => void
    /** Puts the lines back as a version had them, in the working tree (#1082). */
    onRestore?: (v: git.BlockVersion) => void
  } = $props()

  let title = $derived(from === to ? `Line ${from}, through time` : `Lines ${from} to ${to}, through time`)
  let count = $derived.by(() => {
    if (!versions) return ''
    const n = versions.length
    const of = fileVersions > 1 ? ` of the file’s ${fileVersions}` : ''
    const which = from === to ? 'it' : 'them'
    return n === 1 ? `1${of} versions changed ${which}.` : `${n.toLocaleString()}${of} versions changed ${which}.`
  })
</script>

<aside class="block" aria-label={title}>
  <header>
    <span class="hand" aria-hidden="true">only what touched these lines</span>
    <div class="title">
      <h2>{title}</h2>
      <button class="btn sm quiet icon" onclick={onClose} aria-label="Close" title="Close"><Icon name="close" size={13} /></button>
    </div>
    {#if count}<p class="count">{count}</p>{/if}
  </header>
  {#if problem}
    <p class="problem" role="alert">{problem}</p>
  {:else if !versions}
    <Loading />
  {:else}
    <ol>
      {#each versions as v, i (v.id)}
        <li>
          <span class="meta">{versions.length - i} of {versions.length} · {formatTime(v.time)}{i === versions.length - 1 ? ' · where they began' : ''}</span>
          <span class="what">
            <button class="sha" onclick={() => onReveal?.(v.id)} title="Show in graph">{v.id.slice(0, 7)}</button>
            {v.subject} · {v.author}
          </span>
          {#if v.lines.length}
            <pre class="change">{#each v.lines as l, k (k)}<span class="l {l.kind === '+' ? 'add' : l.kind === '-' ? 'del' : ''}"><span class="sign" aria-hidden="true">{l.kind === ' ' ? ' ' : l.kind === '-' ? '−' : '+'}</span>{#if l.kind === '-'}<span class="visually-hidden">Taken out:&nbsp;</span>{:else if l.kind === '+'}<span class="visually-hidden">Added:&nbsp;</span>{/if}{l.text}
</span>{/each}</pre>
          {/if}
          <span class="acts">
            {#if onShow}<button class="btn sm text" onclick={() => onShow(v.id)}>Show the whole file at {v.id.slice(0, 7)}</button>{/if}
            {#if onRestore && i > 0 && v.lines.some((l) => l.kind !== '-')}<button class="btn sm text" onclick={() => onRestore(v)}>Restore these lines</button>{/if}
          </span>
        </li>
      {/each}
    </ol>
  {/if}
</aside>

<style>
  .block {
    display: flex;
    flex-direction: column;
    gap: 12px;
    width: 400px;
    max-width: 45%;
    flex: none;
    box-sizing: border-box;
    padding: 14px 16px;
    overflow: auto;
    border-left: 1px solid var(--paper-edge);
    background: var(--paper-warm);
    color: var(--ink);
  }

  header {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .hand {
    font-family: var(--font-hand);
    font-size: 20px;
    color: var(--accent-a);
    transform: rotate(-1.5deg);
    transform-origin: left;
  }

  .title {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  h2 {
    flex: 1;
    margin: 0;
    font-size: 17px;
    font-weight: 500;
  }

  .count,
  .problem {
    margin: 0;
    font-size: 13px;
    color: var(--ink-soft);
  }

  .problem {
    color: var(--danger);
  }

  ol {
    display: flex;
    flex-direction: column;
    gap: 10px;
    margin: 0;
    padding: 0;
    list-style: none;
  }

  li {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 6px;
    padding: 10px 12px;
    border-radius: var(--r-md);
    background: var(--paper);
  }

  .meta {
    font-family: var(--font-mono);
    font-size: 12px;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--ink-mute);
  }

  .what {
    font-size: 14px;
  }

  .sha {
    padding: 0;
    border: 0;
    background: none;
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--accent-b);
    cursor: pointer;
  }

  .sha:hover {
    color: var(--link);
  }

  .change {
    align-self: stretch;
    margin: 0;
    padding: 6px 0;
    overflow-x: auto;
    border-radius: var(--r-sm);
    font-family: var(--font-mono);
    font-size: 12px;
    line-height: 1.6;
  }

  .acts {
    display: flex;
    flex-wrap: wrap;
    gap: 2px 12px;
  }

  .l {
    display: block;
    padding: 0 8px;
  }

  .sign {
    display: inline-block;
    width: 14px;
    user-select: none;
  }

  .l.add {
    background: color-mix(in srgb, var(--accent) 16%, transparent);
  }

  .l.del {
    color: var(--ink-mute);
    text-decoration: line-through;
  }
</style>
