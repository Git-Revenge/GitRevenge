<script lang="ts">
  // Where a remote's pushes go (#926): git's own remote.<name>.pushurl, edited in the Remotes
  // dialog and written on its Save. A push URL that's another remote's URL follows that remote:
  // when the app moves it, the push URL moves too. Nothing extra is stored to say so.
  import PlaceGlyph from './PlaceGlyph.svelte'
  import { placeLook, type URLPlace } from './remoteHeader'
  import Icon from './ui/Icon.svelte'

  let {
    remote,
    fetchUrl,
    urls = $bindable(),
    others,
    places,
    brand = false,
  }: {
    remote: string
    fetchUrl: string
    /** Where pushes go, in order; the remote's own URL among them. */
    urls: string[]
    /** The other remotes, to push to as well and to tell which a URL follows. */
    others: { name: string; url: string }[]
    places: Record<string, URLPlace>
    brand?: boolean
  } = $props()

  const bare = (u: string) => u.trim().replace(/(\.git)?\/*$/, '')
  const same = (a: string, b: string) => bare(a) === bare(b)
  const follows = (u: string) => others.find((o) => same(o.url, u))?.name

  let adding = $state('')
  let typed = $state('')
  let offer = $derived(others.filter((o) => !urls.some((u) => same(u, o.url))))

  function add() {
    const u = adding === '…' ? typed.trim() : (others.find((o) => o.name === adding)?.url ?? '')
    if (u && !urls.some((x) => same(x, u))) urls = [...urls, u]
    adding = ''
    typed = ''
  }
</script>

<fieldset class="pushing">
  <legend class="meta">Pushing</legend>
  <p class="lead">Pushing to {remote} sends to:</p>
  {#each urls as u (u)}
    {@const own = same(u, fetchUrl)}
    {@const f = own ? undefined : follows(u)}
    <div class="dest">
      <PlaceGlyph look={placeLook(places[u])} size={14} {brand} />
      <span class="t">
        <span class="url" title={u}><bdi>{u}</bdi></span>
        {#if own}<span class="d">its own URL</span>{:else if f}<span class="d"><Icon name="link" size={12} /> follows {f}: moves when {f}'s URL does</span>{/if}
      </span>
      {#if !own}
        <button type="button" class="btn sm quiet icon" aria-label="Stop pushing to {u}" title="Stop pushing here" onclick={() => (urls = urls.filter((x) => x !== u))}><Icon name="close" size={12} /></button>
      {/if}
    </div>
  {/each}
  <div class="add">
    <select bind:value={adding} aria-label="Also push to">
      <option value="">Also push to…</option>
      {#each offer as o (o.name)}<option value={o.name}>{o.name} ({o.url})</option>{/each}
      <option value="…">Another URL…</option>
    </select>
    {#if adding === '…'}<input class="mono" bind:value={typed} placeholder="git@example.com:you/repo.git" aria-label="Another URL to push to" />{/if}
    {#if adding}<button type="button" class="btn sm" onclick={add} disabled={adding === '…' && !typed.trim()}>Add</button>{/if}
  </div>
  <p class="d">Every push to {remote} goes to each in turn. <code>remote.{remote}.pushurl</code></p>
</fieldset>

<style>
  .pushing {
    display: flex;
    flex-direction: column;
    gap: 4px;
    margin: 12px 0 0;
    padding: 0;
    border: none;
    min-width: 0;
  }

  .lead {
    margin: 0;
    font-size: 14px;
  }

  .dest {
    display: grid;
    grid-template-columns: 16px minmax(0, 1fr) auto;
    gap: 8px;
    align-items: center;
    padding: 6px 0;
    border-bottom: 1px dashed var(--hairline);
  }

  .t {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }

  /* Cut from the start: the end of an address (the repository's name) tells them apart. */
  .url {
    font-family: var(--font-mono);
    font-size: 13px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    direction: rtl;
    text-align: left;
  }

  .d {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    font-size: 12px;
    color: var(--ink-mute);
  }

  .add {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
    align-items: center;
    padding-top: 6px;
  }

  .add select,
  .mono {
    font-size: 13px;
    max-width: 320px;
  }

  .mono {
    font-family: var(--font-mono);
  }

  code {
    font-family: var(--font-mono);
    font-size: 12px;
    margin-left: auto;
  }
</style>
