<script lang="ts">
  // The card the graph opens on an author: who they are as the mailmap reads them, what this
  // commit says when that differs, how much they wrote, and the ways on from here. It loads
  // the first time one is opened (the main chunk has a size budget; see
  // scripts/bundle-budget.mjs).
  //
  // Two questions are asked, apart. Who wrote this commit is quick. How much they wrote
  // reads the whole history the first time, which on a long one takes seconds, so the card
  // shows what it has and says it's counting.
  import { onMount } from 'svelte'
  import { AuthorCard, PersonCard } from '../../../wailsjs/go/app/App.js'
  import type { app } from '../../../wailsjs/go/models'
  import { errorText } from '../errors'
  import { initials } from '../graph/draw'
  import Icon from '../ui/Icon.svelte'
  import { fadeUp, menuOut } from '../ui/motion'
  import { faceColor, linesIn, wrote } from './mailmap'
  import { identityText } from './suggest'
  import { peopleTab } from './tab.svelte'

  let {
    repo,
    sha,
    name,
    x,
    y,
    above = y,
    opener,
    onSearch,
    onClose,
  }: {
    repo: string
    /** The commit whose author this is. */
    sha: string
    /** The author as the graph shows them, to say at once. */
    name: string
    /** Where the card goes: its top left corner, in the window. */
    x: number
    y: number
    /** The top of what it was opened from, to sit above it when there's no room below. */
    above?: number
    /** What it was opened from, when that's a button: a click doesn't focus one in every
     * browser, so the focus alone doesn't say where to go back to. */
    opener?: HTMLElement
    onSearch?: (name: string) => void
    onClose: () => void
  } = $props()

  let card = $state<app.AuthorCard | null>(null)
  let person = $state<app.Person | null>(null)
  let error = $state('')
  let shown = $derived(card?.shown ?? { name, email: '' })
  /** What the commit has, when the mailmap reads it as someone else. */
  let says = $derived(card && (card.written.name !== card.shown.name || card.written.email !== card.shown.email) ? identityText(card.written) : '')

  let el: HTMLDivElement
  let width = $state(0)
  let height = $state(0)
  // Kept inside the window: beside the edge when it's near one, and above what it was
  // opened from when there's no room below.
  let left = $derived(Math.max(8, Math.min(x, window.innerWidth - width - 8)))
  let top = $derived(y + height + 8 <= window.innerHeight ? y : Math.max(8, above - height - 6))

  /** What gets the focus back when the card closes: what opened it, or what had the focus. */
  let back: Element | null = null

  onMount(() => {
    back = opener ?? document.activeElement
    el.focus()
    let gone = false
    void (async () => {
      try {
        const c = await AuthorCard(repo, sha)
        if (gone) return
        card = c
        const p = await PersonCard(repo, c.shown)
        if (!gone) person = p
      } catch (e) {
        if (!gone) error = errorText(e)
      }
    })()
    return () => {
      gone = true
    }
  })

  /** Closes, giving the focus back to where it came from. */
  function close() {
    if (back instanceof HTMLElement && back.isConnected) back.focus()
    onClose()
  }

  function outside(e: Event) {
    if (!(e.target instanceof Node && el.contains(e.target))) onClose()
  }

  function search() {
    close()
    onSearch?.(shown.name || shown.email)
  }

  /** The person to hand the People view: everything known, or who they are while it's counted. */
  function whom(): app.Person {
    return person ?? ({ name: shown.name, email: shown.email, commits: 0, coAuthored: 0, bot: !!card?.bot, identities: [] } as unknown as app.Person)
  }

  function openPeople(pick: boolean) {
    onClose()
    void peopleTab.show(repo, { person: whom(), pick })
  }
</script>

<svelte:window
  onmousedown={outside}
  onwheel={outside}
  onblur={onClose}
  onkeydowncapture={(e) => {
    if (e.key !== 'Escape') return
    // The card is what Escape closes, not whatever is under it.
    e.stopPropagation()
    e.preventDefault()
    close()
  }}
/>

<div
  class="card"
  role="dialog"
  aria-label="About {shown.name || shown.email || 'this author'}"
  tabindex="-1"
  bind:this={el}
  bind:clientWidth={width}
  bind:clientHeight={height}
  style:left="{left}px"
  style:top="{top}px"
  in:fadeUp
  out:menuOut|global
  onfocusout={(e) => {
    // Tabbing out of the card leaves it.
    if (e.relatedTarget instanceof Node && !el.contains(e.relatedTarget)) onClose()
  }}
>
  <div class="top">
    <span class="face" style:--c={faceColor(shown.name)} aria-hidden="true">{initials(shown.name)}</span>
    <div class="who">
      <span class="name">
        <span class="n">{shown.name || '(no name)'}</span>
        {#if card?.bot}<span class="tag"><Icon name="bot" size={12} />bot</span>{/if}
      </span>
      <span class="mail">{shown.email}</span>
    </div>
  </div>

  {#if says && card}
    <!-- A mailmap changes who is shown, never the commit. -->
    <div class="says">
      <span class="label">This commit says</span>
      <span class="as">{says}</span>
      <span class="by">Read as {card.shown.name || card.shown.email} by {linesIn(card.lines)}.</span>
    </div>
  {/if}

  {#if error}
    <p class="much failed" role="alert">{error}</p>
  {:else if !person}
    <p class="much" role="status">Counting their commits…</p>
  {:else if person.commits || person.coAuthored}
    <p class="much" role="status">{wrote(person.commits, person.coAuthored)}</p>
  {/if}

  <div class="acts">
    {#if onSearch}<button class="btn sm primary" onclick={search}>Search the timeline for them</button>{/if}
    <button class="btn sm secondary" disabled={!person} title={person ? 'Say in People that this is the same person as someone else' : 'Once their commits are counted'} onclick={() => openPeople(true)}>Same person as…</button>
  </div>
  <button class="btn sm text people" disabled={!card} onclick={() => openPeople(false)}>Open People</button>
</div>

<style>
  .card {
    position: fixed;
    z-index: 90;
    width: min(400px, calc(100vw - 16px));
    box-sizing: border-box;
    display: flex;
    flex-direction: column;
    gap: 12px;
    padding: 16px 18px;
    border: 1px solid var(--paper-edge);
    border-radius: var(--r-lg);
    background: var(--paper);
    box-shadow: var(--shadow);
    color: var(--ink);
    outline: none;
  }

  .top {
    display: flex;
    align-items: center;
    gap: 12px;
    min-width: 0;
  }

  .face {
    flex: none;
    width: 40px;
    height: 40px;
    display: flex;
    align-items: center;
    justify-content: center;
    border: 2px solid var(--c);
    border-radius: 50%;
    background: var(--paper-warm);
    font-family: var(--font-mono);
    font-size: 14px;
  }

  .who {
    min-width: 0;
    display: flex;
    flex-direction: column;
  }

  .name {
    display: flex;
    align-items: baseline;
    gap: 8px;
    min-width: 0;
  }

  .n {
    min-width: 0;
    overflow: hidden;
    font-size: 18px;
    font-weight: 500;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .tag {
    flex: none;
    display: inline-flex;
    align-items: center;
    gap: 4px;
    font-family: var(--font-mono);
    font-size: 12px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--ink-mute);
  }

  .mail {
    /* Holds its line while the address is on its way, so the card doesn't jump. */
    min-height: 18px;
    overflow: hidden;
    font-family: var(--font-mono);
    font-size: 13px;
    color: var(--ink-soft);
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .says {
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: 10px 12px;
    border-radius: var(--r-md);
    background: var(--paper-warm);
  }

  .label {
    font-family: var(--font-mono);
    font-size: 12px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--ink-mute);
  }

  .as {
    font-family: var(--font-mono);
    font-size: 13px;
    overflow-wrap: anywhere;
  }

  .by {
    font-size: 14px;
    color: var(--ink-soft);
  }

  .much {
    margin: 0;
    color: var(--ink-soft);
  }

  .failed {
    color: var(--danger);
  }

  .acts {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }

  .people {
    align-self: flex-start;
  }
</style>
