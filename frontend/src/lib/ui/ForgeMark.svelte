<script lang="ts">
  // A forge's mark, filled, unlike the app's stroked icons. Ink unless the brand colours
  // setting is on; GitHub's near-black stays ink either way, so it reads on dark paper.
  import type { ForgeMark } from './forgeMarks'

  let { mark, size = 14, brand = false }: { mark: ForgeMark; size?: number; brand?: boolean } = $props()

  let fill = $derived(brand && mark.color !== '#181717' ? mark.color : 'currentColor')
</script>

<svg width={size} height={size} viewBox="0 0 24 24" aria-hidden="true" focusable="false"><path d={mark.d} {fill} /></svg>

<style>
  svg {
    flex: none;
  }

  @media (forced-colors: active) {
    path {
      fill: currentColor;
    }
  }
</style>
