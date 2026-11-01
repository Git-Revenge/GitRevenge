<script lang="ts">
  // An image in rendered markdown, loaded from the repository by resolve (a data URL), so a
  // document never makes a network request. Until it loads, or if it can't, the alt text
  // shows in its place.
  let { src, alt, resolve }: { src: string; alt: string; resolve: (src: string) => Promise<string> } = $props()

  let url = $state('')

  $effect(() => {
    const s = src
    url = ''
    void resolve(s)
      .then((u) => {
        if (s === src) url = u
      })
      // The placeholder below shows the image's name instead.
      .catch(() => {})
  })
</script>

{#if url}<img src={url} {alt} />{:else}<span class="missing" title={src}>[{alt || src}]</span>{/if}

<style>
  img {
    max-width: 100%;
    height: auto;
  }

  .missing {
    color: var(--ink-mute);
    font-style: italic;
  }
</style>
