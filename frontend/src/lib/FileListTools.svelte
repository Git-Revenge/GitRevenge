<script lang="ts">
  import { saveTreeChoice } from './fileList'

  let { query = $bindable(''), tree = $bindable(false) }: { query?: string; tree?: boolean } = $props()
</script>

<div class="tools">
  <input type="search" bind:value={query} placeholder="Filter files" aria-label="Filter files" />
  <button
    class="btn sm quiet"
    class:on={tree}
    aria-pressed={tree}
    title="Switch between a flat list and a folder tree"
    onclick={() => {
      tree = !tree
      saveTreeChoice(tree)
    }}>Tree</button
  >
</div>

<style>
  .tools {
    display: flex;
    align-items: center;
    gap: 6px;
  }

  input {
    flex: 1;
    width: 0;
    min-width: 0;
    height: 28px;
    padding: 0 9px;
    font-size: 13px;
    border-radius: 5px;
  }

  .on {
    background: color-mix(in srgb, var(--link) 14%, transparent);
    color: var(--link);
  }
</style>
