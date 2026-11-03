<script lang="ts">
  import { kindInfo } from '../branchKinds'
  import Icon from '../ui/Icon.svelte'
  import type { Ref } from './rowCache'

  let {
    ref,
    lane = 'var(--ink-mute)',
  }: {
    ref: Ref
    /** The colour of the commit's lane, for a branch with no kind of its own. */
    lane?: string
  } = $props()

  let isBranch = $derived(ref.kind === 'local' || ref.kind === 'remote')
  let kind = $derived(isBranch ? kindInfo(ref.branchKind) : undefined)
  let color = $derived(ref.kind === 'tag' ? 'var(--lane-4)' : kind ? `var(${kind.token})` : lane)
  let mark = $derived(ref.kind === 'tag' ? '◆ ' : ref.kind === 'stash' ? '≡ ' : '')
</script>

<span class="chip" class:head={ref.head} style:--chip={color}>
  {#if kind}<Icon name={kind.icon} size={12} />{/if}
  <span class="name">{mark}{ref.short}</span>
  {#if ref.kind === 'remote'}<Icon name="remote" size={12} />{/if}
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
    border-radius: 4px;
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

  .head {
    font-weight: 600;
    color: var(--on-accent);
    background: var(--chip);
  }

  .head :global(svg) {
    color: inherit;
  }
</style>
