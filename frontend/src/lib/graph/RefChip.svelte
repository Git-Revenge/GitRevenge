<script lang="ts">
  import { kindInfo } from '../branchKinds'
  import Icon from '../ui/Icon.svelte'
  import type { RefLabel } from './draw'

  let {
    label,
    lane = 'var(--ink-mute)',
  }: {
    label: RefLabel
    /** The colour of the commit's lane, for a branch with no kind of its own. */
    lane?: string
  } = $props()

  let ref = $derived(label.ref)
  let isBranch = $derived(ref.kind === 'local' || ref.kind === 'remote')
  let kind = $derived(isBranch ? kindInfo(ref.branchKind) : undefined)
  let color = $derived(ref.kind === 'tag' ? 'var(--lane-4)' : kind ? `var(${kind.token})` : lane)
</script>

<span class="chip" class:head={ref.head} style:--chip={color}>
  {#if kind}<Icon name={kind.icon} size={12} />{/if}
  <span class="name">{label.text}</span>
  {#if ref.worktree}<span class="worktree" title="Checked out in the worktree at {ref.worktree}"><Icon name="worktree" size={12} /><span class="visually-hidden">, in a worktree</span></span>{/if}
  {#if label.places.length}<span class="places">{label.places.join(' · ')}</span>{/if}
</span>

<style>
  .chip {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    box-sizing: border-box;
    max-width: 100%;
    min-height: 22px;
    padding: 2px 7px;
    border-radius: var(--r-sm);
    font-family: var(--font-mono);
    font-size: 12px;
    line-height: 1.4;
    color: var(--ink-soft);
    background: color-mix(in srgb, var(--chip) 16%, transparent);
  }

  .chip :global(svg) {
    flex: none;
    color: var(--chip);
  }

  .name {
    min-width: 0;
    overflow-wrap: anywhere;
  }

  .worktree {
    flex: none;
    display: inline-flex;
  }

  .places {
    flex: none;
    margin-left: 4px;
    color: var(--ink-mute);
  }

  .head {
    font-weight: 600;
    color: var(--on-accent);
    background: var(--chip);
  }

  .head :global(svg),
  .head .places {
    color: inherit;
    font-weight: 400;
  }
</style>
