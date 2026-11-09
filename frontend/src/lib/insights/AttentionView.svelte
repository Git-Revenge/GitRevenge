<script lang="ts">
  // The Needs attention view of the Insights tab (#251): open branches past a threshold, or
  // that would conflict with the target, measured at the as-of point.
  import type { app } from '../../../wailsjs/go/models'
  import { session } from '../session.svelte'
  import { insights } from './insights.svelte'
  import { attention } from './model'

  let flagged = $derived(attention(insights.branches))
  let multi = $derived(insights.repos.length > 1)
  let w = $derived(insights.resolved)
  // Edits start from the saved thresholds and reset when the scope's change.
  let stale = $derived(insights.thresholds.stale)
  let longLived = $derived(insights.thresholds.longLived)
  let behind = $derived(insights.thresholds.behind)
  let size = $derived(insights.thresholds.size)
  let draft = $derived({ stale, longLived, behind, size } as app.Thresholds)
  let changed = $derived(
    stale !== insights.thresholds.stale || longLived !== insights.thresholds.longLived || behind !== insights.thresholds.behind || size !== insights.thresholds.size,
  )
  let valid = $derived([stale, longLived, behind, size].every((v) => Number.isInteger(v) && v >= 1))
  const num = (e: Event & { currentTarget: HTMLInputElement }) => e.currentTarget.valueAsNumber

  const repoName = (path: string) => session.tabs.find((t) => t.path === path)?.name ?? path.split(/[\\/]/).pop() ?? path
  const days = (n: number) => `${n} ${n === 1 ? 'day' : 'days'}`
  const target = $derived(w?.target.replace(/^origin\//, '') ?? 'the target')
  import Empty from '../ui/Empty.svelte'
</script>

<div class="attention">
  <p class="summary">
    {flagged.length} of {insights.branches.length} open branches · conflicts first, then the most flags{#if insights.pending}
      · <span class="checking">{insights.pending} conflict {insights.pending === 1 ? 'check' : 'checks'} still running</span>{/if}
  </p>

  <details class="thresholds" open>
    <summary>Thresholds <span class="sub">· saved for this {insights.scope?.kind === 'group' ? 'group' : 'repository'}, and printed with every export</span></summary>
    <form
      onsubmit={(e) => {
        e.preventDefault()
        if (valid && changed) void insights.setThresholds({ ...draft } as app.Thresholds)
      }}
    >
      <label><span class="meta">Stale after</span><span class="in"><input type="number" min="1" value={stale} oninput={(e) => (stale = num(e))} /> days</span></label>
      <label><span class="meta">Long-lived after</span><span class="in"><input type="number" min="1" value={longLived} oninput={(e) => (longLived = num(e))} /> days</span></label>
      <label><span class="meta">Drifting past</span><span class="in"><input type="number" min="1" value={behind} oninput={(e) => (behind = num(e))} /> behind</span></label>
      <label><span class="meta">Big batch over</span><span class="in"><input type="number" min="1" value={size} oninput={(e) => (size = num(e))} /> lines</span></label>
      {#if changed}<button class="btn secondary" type="submit" disabled={!valid}>Apply</button>{/if}
    </form>
  </details>

  {#if flagged.length === 0}
    <Empty note="all moving">{insights.pending ? 'Nothing flagged yet; conflict checks are still running.' : 'No open branch is past a threshold or conflicting.'}</Empty>
  {:else}
    <div class="scroll">
      <table>
        <thead>
          <tr>
            <th scope="col">Branch</th>
            <th scope="col"><span class="why" title="git merge-tree --write-tree {target} <branch>: a dry-run merge that touches nothing">Merge</span></th>
            <th scope="col" class="num"><span class="why" title="Whole days from the branch's last commit to the as-of point">Last commit</span></th>
            <th scope="col" class="num"><span class="why" title="Whole days from its first commit not on {target} to the as-of point">Age</span></th>
            <th scope="col" class="num"><span class="why" title="Commits on {target} since the branch's merge base">Behind</span></th>
            <th scope="col" class="num"><span class="why" title="Lines added and removed against the merge base">Size</span></th>
          </tr>
        </thead>
        <tbody>
          {#each flagged as b (b.repo + b.value.name)}
            {@const s = b.value}
            <tr>
              <td>
                <code class="branch">{s.name}</code>
                <span class="sub">{s.author}{multi ? ` · ${repoName(b.repo)}` : ''}{s.local ? ' · local only' : ''}</span>
              </td>
              <td>
                {#if !b.conflict}
                  <span class="checking">checking…</span>
                {:else if b.conflict.conflicts}
                  <span class="tag rose">conflicts</span>
                  <span class="files">{b.conflict.files.slice(0, 2).join(', ')}{b.conflict.files.length > 2 ? ` +${b.conflict.files.length - 2}` : ''}</span>
                {:else}
                  <span class="clean">clean</span>
                {/if}
              </td>
              <td class="num"><span class="val" class:over={s.flags.includes('stale')}>{days(s.staleDays)}</span></td>
              <td class="num"><span class="val" class:over={s.flags.includes('long-lived')}>{days(s.ageDays)}</span></td>
              <td class="num"><span class="val" class:over={s.flags.includes('drifting')}>{s.behind}</span></td>
              <td class="num"><span class="val" class:over={s.flags.includes('big-batch')}>{s.added + s.removed}</span></td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}

  <p class="foot">
    <span class="hand">only as fresh as your fetch</span>Remote branches are compared as of each repository's last fetch. Branches already merged into {target}, and local branches that
    track a remote, aren't listed.
  </p>
</div>

<style>
  .attention {
    display: flex;
    flex-direction: column;
    gap: 22px;
  }

  .summary {
    margin: 0;
    color: var(--ink-mute);
  }

  .checking {
    font-style: italic;
    color: var(--ink-mute);
  }

  .thresholds {
    padding: 14px 18px;
    background: var(--paper-warm);
    border-radius: var(--r-md);
  }

  .thresholds summary {
    cursor: pointer;
    font-size: 15px;
  }

  form {
    display: flex;
    flex-wrap: wrap;
    align-items: flex-end;
    gap: 14px 24px;
    margin-top: 12px;
  }

  label {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .in {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 13px;
    color: var(--ink-mute);
  }

  input {
    width: 84px;
    min-height: 34px;
    padding: 6px 8px;
    border-radius: var(--r-sm);
    font-family: var(--font-mono);
    font-size: 13px;
  }

  .scroll {
    overflow-x: auto;
  }

  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 14px;
  }

  th {
    padding: 8px 10px;
    border-bottom: 1px solid var(--paper-edge);
    font-family: var(--font-mono);
    font-size: 12px;
    font-weight: 400;
    letter-spacing: 0.08em;
    text-align: left;
    text-transform: uppercase;
    color: var(--ink-mute);
    white-space: nowrap;
  }

  td {
    padding: 12px 10px;
    border-bottom: 1px dotted var(--paper-edge);
    vertical-align: top;
  }

  .num {
    text-align: right;
  }

  .why {
    border-bottom: 1px dotted var(--ink-faint);
    cursor: help;
  }

  .branch {
    display: block;
    font-family: var(--font-mono);
    font-size: 14px;
    color: var(--accent-b);
    overflow-wrap: anywhere;
  }

  .sub,
  .files {
    display: block;
    font-size: 13px;
    color: var(--ink-mute);
  }

  .files {
    margin-top: 4px;
    font-family: var(--font-mono);
    font-size: 12px;
  }

  .clean {
    font-size: 13px;
    color: var(--success);
  }

  .val {
    padding: 1px 7px;
    border-radius: var(--r-sm);
    font-family: var(--font-mono);
    font-size: 13px;
    color: var(--ink-mute);
    white-space: nowrap;
  }

  /* Past its threshold. */
  .val.over {
    background: color-mix(in srgb, var(--ink-ochre) 22%, transparent);
    color: var(--warning);
  }

  .foot {
    display: flex;
    gap: 12px;
    align-items: baseline;
    margin: 0;
    font-size: 13px;
    color: var(--ink-mute);
  }

  .hand {
    font-size: 20px;
    color: var(--accent-c);
    white-space: nowrap;
  }
</style>
