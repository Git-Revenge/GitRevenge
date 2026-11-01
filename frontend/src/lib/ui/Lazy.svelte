<script lang="ts">
  // A view that loads its code the first time it's shown, to keep the app's main bundle small.
  import type { Component } from 'svelte'
  import { errorText } from '../errors'

  // eslint-disable-next-line @typescript-eslint/no-explicit-any -- views take any props
  type View = Component<any>

  let { load, ...props }: { load: () => Promise<{ default: View }>; [prop: string]: unknown } = $props()
</script>

{#await load() then view}
  <view.default {...props} />
{:catch e}
  <p class="failed" role="alert">Couldn't load this view: {errorText(e)}</p>
{/await}

<style>
  .failed {
    margin: 0;
    padding: 12px;
    color: var(--danger);
  }
</style>
