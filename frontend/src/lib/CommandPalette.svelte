<script lang="ts">
  import { tick } from 'svelte'
  import { commands, formatShortcut, type Command } from './commands.svelte'
  import { fuzzy } from './fuzzy'
  import Icon from './ui/Icon.svelte'
  import { menuOut, scrim } from './ui/motion'

  /** Sections in the order they're listed when nothing is typed. */
  const GROUP_ORDER = ['Repository', 'Branch', 'Go to', 'Tabs', 'Recent', 'View', 'App']
  /** Rows shown at most; typing narrows the rest. */
  const LIMIT = 80

  interface Row {
    cmd: Command
    /** The title cut into plain and matched pieces. */
    parts: { text: string; hit: boolean }[]
    /** The first row of a section, when listing without a query. */
    heading?: string
  }

  const id = $props.id()
  let input = $state<HTMLInputElement | null>(null)
  let list = $state<HTMLUListElement | null>(null)
  let query = $state('')
  let active = $state(0)
  let all = $state<Command[]>([])
  let restore: HTMLElement | null = null

  // Take a fresh snapshot each time the palette opens, so titles reflect current state.
  $effect(() => {
    if (!commands.paletteOpen) return
    restore = document.activeElement as HTMLElement | null
    all = commands.all().filter((c) => !c.hidden)
    query = ''
    active = 0
    void tick().then(() => input?.focus())
  })

  function split(title: string, positions: number[]): Row['parts'] {
    const parts: Row['parts'] = []
    const hits = new Set(positions)
    for (let i = 0; i < title.length; i++) {
      const hit = hits.has(i)
      const last = parts[parts.length - 1]
      if (last && last.hit === hit) last.text += title[i]
      else parts.push({ text: title[i], hit })
    }
    return parts
  }

  let rows = $derived.by((): Row[] => {
    const q = query.trim()
    if (!q) {
      const rank = (g: string) => (GROUP_ORDER.indexOf(g) + 1 || GROUP_ORDER.length + 1)
      const sorted = [...all].sort((a, b) => rank(a.group) - rank(b.group))
      return sorted.slice(0, LIMIT).map((cmd, i) => ({
        cmd,
        parts: [{ text: cmd.title, hit: false }],
        heading: i === 0 || sorted[i - 1].group !== cmd.group ? cmd.group : undefined,
      }))
    }
    const scored: { cmd: Command; score: number; positions: number[] }[] = []
    for (const cmd of all) {
      const m = fuzzy(q, cmd.title)
      if (m) scored.push({ cmd, score: m.score, positions: m.positions })
      else if (cmd.keywords) {
        // Hidden keywords match only when one starts with the query ("orig" finds Manage
        // remotes through "origin"), so letters scattered across them don't add noise.
        const words = cmd.keywords.toLowerCase().split(/\s+/)
        const lq = q.toLowerCase()
        if (words.some((w) => w.startsWith(lq))) scored.push({ cmd, score: lq.length, positions: [] })
      }
    }
    scored.sort((a, b) => b.score - a.score)
    return scored.slice(0, LIMIT).map((s) => ({ cmd: s.cmd, parts: split(s.cmd.title, s.positions) }))
  })

  $effect(() => {
    void query
    active = 0
  })

  function close() {
    commands.paletteOpen = false
    restore?.focus?.()
  }

  async function run(row: Row | undefined) {
    if (!row) return
    commands.paletteOpen = false
    await tick()
    await row.cmd.run()
  }

  function move(to: number) {
    if (rows.length === 0) return
    active = (to + rows.length) % rows.length
    void tick().then(() => list?.querySelector(`[data-i="${active}"]`)?.scrollIntoView({ block: 'nearest' }))
  }

  function onKey(e: KeyboardEvent) {
    const page = 8
    if (e.key === 'ArrowDown') move(active + 1)
    else if (e.key === 'ArrowUp') move(active - 1)
    else if (e.key === 'PageDown') move(Math.min(rows.length - 1, active + page))
    else if (e.key === 'PageUp') move(Math.max(0, active - page))
    else if (e.key === 'Enter') void run(rows[active])
    else if (e.key === 'Escape') close()
    else if (e.key === 'Tab') {
      // Focus stays in the field; there's nothing else to tab to.
    } else return
    e.preventDefault()
    e.stopPropagation()
  }
</script>

{#if commands.paletteOpen}
  <div class="scrim" role="presentation" out:scrim={{ out: true }} onmousedown={(e) => e.target === e.currentTarget && close()}>
    <div class="palette" out:menuOut role="dialog" aria-modal="true" aria-label="Command palette">
      <label class="field">
        <Icon name="search" size={17} />
        <input
          bind:this={input}
          bind:value={query}
          onkeydown={onKey}
          placeholder="Type a command, branch or repository"
          role="combobox"
          aria-expanded="true"
          aria-controls="{id}-list"
          aria-activedescendant={rows.length ? `${id}-${active}` : undefined}
          aria-autocomplete="list"
          autocomplete="off"
          spellcheck="false"
        />
        <kbd>Esc</kbd>
      </label>
      <ul id="{id}-list" role="listbox" aria-label="Commands" bind:this={list}>
        {#each rows as row, i (row.cmd.id)}
          {#if row.heading}<li class="heading meta" role="presentation">{row.heading}</li>{/if}
          <!-- The combobox input handles every key and points at this option with
               aria-activedescendant, so the option itself only needs the click. -->
          <!-- svelte-ignore a11y_click_events_have_key_events -->
          <li
            id="{id}-{i}"
            data-i={i}
            role="option"
            aria-selected={i === active}
            class:active={i === active}
            onmousemove={() => (active = i)}
            onmousedown={(e) => e.preventDefault()}
            onclick={() => run(row)}
          >
            <span class="title"
              >{#each row.parts as p, k (k)}{#if p.hit}<mark>{p.text}</mark>{:else}{p.text}{/if}{/each}</span
            >
            {#if query.trim()}<span class="group meta">{row.cmd.group}</span>{/if}
            {#if row.cmd.shortcut}<kbd>{formatShortcut(row.cmd.shortcut)}</kbd>{/if}
          </li>
        {:else}
          <li class="empty" role="presentation">No matching commands.</li>
        {/each}
      </ul>
    </div>
  </div>
{/if}

<style>
  .scrim {
    position: fixed;
    inset: 0;
    z-index: 250;
    display: flex;
    justify-content: center;
    align-items: flex-start;
    padding: 12vh 16px 16px;
    background: var(--scrim);
    animation: fade-in var(--dur-base) var(--ease-ink);
  }

  .palette {
    width: min(620px, 100%);
    max-height: 70vh;
    display: flex;
    flex-direction: column;
    background: var(--paper);
    border: 1px solid var(--paper-edge);
    border-radius: var(--r-lg);
    box-shadow: var(--shadow);
    overflow: hidden;
    animation: drop-in var(--dur-move) var(--ease-ink);
  }

  .field {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 0 16px;
    border-bottom: 1px solid var(--paper-edge);
    color: var(--ink-mute);
  }

  input {
    flex: 1;
    min-width: 0;
    height: 54px;
    padding: 0;
    background: none;
    border: none;
    outline: none;
    font-size: 18px;
    color: var(--ink);
  }

  input::placeholder {
    color: var(--ink-faint);
  }

  ul {
    list-style: none;
    margin: 0;
    padding: 6px;
    overflow-y: auto;
  }

  .heading {
    padding: 10px 10px 4px;
  }

  li[role='option'] {
    display: flex;
    align-items: center;
    gap: 12px;
    min-height: 36px;
    padding: 0 10px;
    border-radius: var(--r-sm);
    cursor: pointer;
  }

  li.active {
    background: var(--selected);
  }

  .title {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: 15px;
  }

  mark {
    background: none;
    color: var(--link);
    font-weight: 600;
  }

  .group {
    flex: none;
  }

  kbd {
    flex: none;
    padding: 1px 6px;
    border: 1px solid var(--paper-edge);
    border-radius: var(--r-sm);
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-mute);
  }

  .empty {
    padding: 18px 12px;
    font-style: italic;
    color: var(--ink-mute);
  }

  @media (prefers-reduced-motion: reduce) {
    .scrim,
    .palette {
      animation: none;
    }
  }
</style>
