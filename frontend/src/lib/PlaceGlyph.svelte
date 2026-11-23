<script lang="ts">
  // Where a remote URL points, as one small glyph: its forge's mark where it opens on the web,
  // or the app's own cloud, server or folder.
  import type { PlaceLook } from './remoteHeader'
  import ForgeMark from './ui/ForgeMark.svelte'
  import Icon from './ui/Icon.svelte'

  let { look, size = 14, brand = false }: { look: PlaceLook; size?: number; brand?: boolean } = $props()
</script>

{#if look.mark}<ForgeMark mark={look.mark} {size} {brand} />{:else}<Icon name={look.icon} {size} />{/if}
