<script lang="ts">
  // The history search's chip on the filter strip (#982): "adds or removes km/h", with ×.
  // A click opens its options: the text, Match case, Regular expression, what a commit has to
  // have done (git log -S or -G), Only in, As far back as, then Remove and Search again.
  // Match case, Regular expression and Only in mean what they mean in the Find panel.
  import Icon from '../ui/Icon.svelte'
  import { filePatterns } from '../find/find'
  import { chipName, chipVerb, countText, dayOfSince, lastYear, sinceChoice, sinceOfDay, type HistoryOptions } from './history'

  let {
    options,
    counts = {},
    onSearch,
    onRemove,
  }: {
    options: HistoryOptions
    /** How many commits each way of matching found, once a search that way finished: S for
     * appears or goes away, G for a line with it changes. */
    counts?: { S?: number; G?: number }
    onSearch: (o: HistoryOptions) => void
    onRemove: () => void
  } = $props()

  const uid = $props.id()
  let open = $state(false)
  let root = $state<HTMLElement>()
  let button = $state<HTMLButtonElement>()
  let form = $state<HTMLFormElement>()

  // The form, filled from the search each time it opens.
  let text = $state('')
  let matchCase = $state(false)
  let regex = $state(false)
  let lines = $state(false)
  let files = $state('')
  let back = $state<'all' | 'year' | 'date'>('all')
  let day = $state('')

  /** Opens the options, with the focus on the text, or on Only in to narrow to a folder. */
  export function show(field: 'text' | 'paths' = 'text') {
    text = options.text
    matchCase = options.matchCase
    regex = options.regex
    lines = options.lines
    files = (options.paths ?? []).join(', ')
    back = sinceChoice(options.since)
    day = back === 'date' ? dayOfSince(options.since) : ''
    open = true
    queueMicrotask(() => form?.querySelector<HTMLInputElement>(field === 'paths' ? 'input[name=paths]' : 'input[name=text]')?.focus())
  }

  function close() {
    open = false
    button?.focus()
  }

  function onKey(e: KeyboardEvent) {
    if (e.key !== 'Escape' || !open) return
    // Only these options close, not what's behind them.
    e.stopPropagation()
    close()
  }

  $effect(() => {
    if (!open) return
    const outside = (e: PointerEvent) => {
      if (!root?.contains(e.target as Node)) open = false
    }
    document.addEventListener('pointerdown', outside)
    return () => document.removeEventListener('pointerdown', outside)
  })

  let since = $derived(back === 'all' ? 0 : back === 'year' ? lastYear() : sinceOfDay(day))
  // A date that hasn't been picked yet searches nothing back: it waits for one.
  let ready = $derived(!!text.trim() && !/[\r\n]/.test(text) && (back !== 'date' || since > 0))

  function submit(e: SubmitEvent) {
    e.preventDefault()
    if (!ready) return
    open = false
    onSearch({ text: text.trim(), matchCase, regex, lines, paths: filePatterns(files), since })
    button?.focus()
  }

  let shown = $derived(text.trim() || options.text)
  /** "-S", or "-S · 7 commits" once a search that way has finished. */
  const wayLabel = (flag: string, n: number | undefined) => (n === undefined ? flag : `${flag} · ${countText(n)}`)
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<span class="history" bind:this={root} onkeydown={onKey}>
  <span class="chip">
    <button bind:this={button} class="open" aria-expanded={open} aria-haspopup="dialog" aria-label={chipName(options)} title="Change this search" onclick={() => (open ? close() : show())}>
      <Icon name="history" size={12} />
      <span class="verb">{chipVerb(options)}</span>
      <span class="text">{options.text}</span>
    </button>
    <button class="x" aria-label="Remove the search for commits that {options.lines ? 'change a line with' : 'add or remove'} {options.text}" title="Remove this filter" onclick={onRemove}><Icon name="close" size={11} /></button>
  </span>
  {#if open}
    <div class="popover" role="dialog" aria-label="Change the history search">
      <form bind:this={form} onsubmit={submit}>
        <label class="field">
          Text
          <input name="text" class="mono" bind:value={text} spellcheck="false" autocomplete="off" />
        </label>
        <div class="checks">
          <label><input type="checkbox" bind:checked={matchCase} /> Match case</label>
          <label><input type="checkbox" bind:checked={regex} /> Regular expression</label>
        </div>
        <fieldset>
          <legend>A commit counts when</legend>
          <label class="how">
            <input type="radio" name="{uid}-how" checked={!lines} onchange={() => (lines = false)} />
            <span>
              <span class="mono">{shown}</span> appears or goes away
              <span class="hint">the number of times it’s there changes</span>
            </span>
            <span class="meta count">{wayLabel('-S', counts.S)}</span>
          </label>
          <label class="how">
            <input type="radio" name="{uid}-how" checked={lines} onchange={() => (lines = true)} />
            <span>
              a line with <span class="mono">{shown}</span> changes
              <span class="hint">even if it stays put</span>
            </span>
            <span class="meta count">{wayLabel('-G', counts.G)}</span>
          </label>
        </fieldset>
        <div class="two">
          <label class="field">
            Only in
            <input name="paths" class="mono" bind:value={files} placeholder="every file" title="Files or folders, between commas: src/, *.js, !*.min.js" spellcheck="false" autocomplete="off" />
          </label>
          <label class="field">
            As far back as
            <select bind:value={back}>
              <option value="all">The whole history</option>
              <option value="year">The last year</option>
              <option value="date">A date…</option>
            </select>
          </label>
        </div>
        {#if back === 'date'}
          <label class="field">
            From
            <input type="date" bind:value={day} />
          </label>
        {/if}
        <div class="actions">
          <button type="button" class="btn sm" onclick={() => ((open = false), onRemove())}>Remove</button>
          <button type="submit" class="btn sm primary" disabled={!ready}>Search again</button>
        </div>
      </form>
    </div>
  {/if}
</span>

<style>
  .history {
    position: relative;
    display: inline-flex;
    max-width: 100%;
  }

  /* As a trailer's chip is, washed in rose, with the verb in small mono and the text in ink. */
  .chip {
    display: inline-flex;
    align-items: center;
    gap: 2px;
    max-width: 100%;
    padding: 0 4px 0 0;
    border-radius: 999px;
    background: var(--selected);
    color: var(--ink);
    font-size: 13px;
  }

  .open {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    min-width: 0;
    padding: 2px 4px 2px 9px;
    border: 0;
    border-radius: 999px;
    background: none;
    color: inherit;
    font: inherit;
    cursor: pointer;
  }

  .open:focus-visible {
    outline: 2px solid var(--ink-rose);
    outline-offset: 1px;
  }

  /* The accent, which each theme sets: --ink-rose-deep has no dark value, and was dark rose on
     the dark theme's rose wash. */
  .open :global(svg) {
    flex: none;
    color: var(--accent);
  }

  .verb {
    flex: none;
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-mute);
  }

  .text {
    min-width: 0;
    overflow: hidden;
    font-family: var(--font-mono);
    font-weight: 600;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .x {
    flex: none;
    display: grid;
    place-items: center;
    width: 18px;
    height: 18px;
    padding: 0;
    border: 0;
    border-radius: 50%;
    background: none;
    color: var(--ink-mute);
    cursor: pointer;
  }

  .x:hover,
  .x:focus-visible {
    background: var(--paper);
    color: var(--ink);
  }

  .popover {
    position: absolute;
    z-index: 25;
    top: calc(100% + 6px);
    left: 0;
    width: min(380px, 86vw);
    box-sizing: border-box;
    padding: 14px 16px;
    background: var(--paper);
    border: 1px solid var(--paper-edge);
    border-radius: var(--r-md);
    box-shadow: var(--shadow);
    animation: drop-in var(--dur-base) var(--ease-ink);
  }

  form {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .field {
    display: flex;
    flex-direction: column;
    gap: 4px;
    min-width: 0;
    font-size: 12px;
    color: var(--ink-mute);
  }

  .field input,
  .field select {
    min-width: 0;
    font-size: 13px;
  }

  .checks {
    display: flex;
    flex-wrap: wrap;
    gap: 6px 18px;
    font-size: 13px;
  }

  .checks label {
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }

  fieldset {
    display: flex;
    flex-direction: column;
    gap: 8px;
    margin: 0;
    padding: 0;
    border: 0;
  }

  legend {
    margin-bottom: 6px;
    padding: 0;
    font-size: 12px;
    color: var(--ink-mute);
  }

  .how {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr) auto;
    gap: 8px;
    align-items: start;
    font-size: 13px;
  }

  .how input {
    margin-top: 3px;
  }

  .hint {
    display: block;
    font-size: 12px;
    color: var(--ink-mute);
  }

  .count {
    text-align: right;
    white-space: nowrap;
  }

  .mono {
    font-family: var(--font-mono);
  }

  /* Only in has what As far back as leaves: the select is as wide as its longest choice, so
     "The whole history" is never cut. */
  .two {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 12px;
  }

  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
  }
</style>
