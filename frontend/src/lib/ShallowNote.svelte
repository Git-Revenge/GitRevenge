<script lang="ts">
  // Says when a shallow clone's history starts (#1006), wherever a view counts that history:
  // an author quiet since then may only be out of reach, not gone. Nothing for a whole clone.
  import { CloneShape } from '../../wailsjs/go/app/App.js'
  import { session } from './session.svelte'
  import { day } from './thisClone'

  let { path, name = '' }: { path: string; /** The repository's name, in a view of several. */ name?: string } = $props()

  let since = $state(0)
  $effect(() => {
    const p = path
    void session.revision[p]
    void CloneShape(p)
      .then((s) => {
        if (p === path) since = s?.shallow?.since ?? 0
      })
      .catch(() => {})
  })
</script>

{#if since}
  <p class="shallow-note" title="A shallow clone: its older history is on the server only. Fetch more from This clone, in the Repository panel">
    {name ? `${name}: ` : ''}a shallow clone · history since {day(since)}
  </p>
{/if}

<style>
  .shallow-note {
    margin: 0;
    font-family: var(--font-mono);
    font-size: 12px;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--ink-mute);
  }
</style>
