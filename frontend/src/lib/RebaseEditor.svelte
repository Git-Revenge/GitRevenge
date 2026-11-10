<script lang="ts">
  import { tick } from 'svelte'
  import { InteractiveRebase, PreviewRebasePlan, RebasePlan, RepoState } from '../../wailsjs/go/app/App.js'
  import type { app, git } from '../../wailsjs/go/models'
  import { errorText } from './errors'
  import { ACTIONS, actionForKey, actionLabel, changed, foldable, foldIn, foldTarget, move, newPlan, outcome, problem as problemOf, setAction, toRun, type Action, type PlanItem } from './rebasePlan'
  import { session } from './session.svelte'
  import Icon from './ui/Icon.svelte'

  /** The interactive rebase editor (issue #596): what happens to each commit after base, oldest
   * first as git applies them, with the branch as it will be afterwards beside it. */
  let { path, base, onClose }: { path: string; base: string; onClose: () => void } = $props()

  let items = $state<PlanItem[]>([])
  /** The commits in the order they came, for Reset and to know whether anything changed. */
  let fresh: PlanItem[] = []
  let error = $state('')
  let busy = $state(false)
  /** False until the plan has answered, so an empty list doesn't read as 0 commits. */
  let loaded = $state(false)
  let branch = $derived(session.tabs.find((t) => t.path === path)?.head ?? '')

  async function load() {
    loaded = false
    error = ''
    try {
      fresh = newPlan((await RebasePlan(path, base)) ?? [])
      items = fresh.map((it) => ({ ...it }))
    } catch (e) {
      error = errorText(e)
    } finally {
      loaded = true
    }
  }

  $effect(() => {
    void base
    void load()
  })

  let problem = $derived(problemOf(items))
  let after = $derived(outcome(items))
  let canFold = $derived(foldable(items))
  let dirty = $derived(
    changed(
      items,
      fresh.map((it) => it.commit),
    ) || items.some((it, i) => it.message !== fresh[i]?.message),
  )
  const short = (id: string) => id.slice(0, 7)
  /** The base as it's named: a branch or tag as it is, a commit id shortened. */
  let baseName = $derived(base.length === 40 && !base.includes('/') ? short(base) : base)
  const count = (n: number, one: string, many: string) => `${n} ${n === 1 ? one : many}`

  // The forecast for this order. It follows the plan a moment behind, so dragging a commit
  // through five places asks once.
  let forecast = $state<app.RebasePlanPreview | null>(null)
  let forecasting = $state(false)
  let asked = 0
  let order = $derived(items.map((it) => `${it.commit}:${it.action === 'drop' ? 'drop' : 'keep'}`).join(' '))
  $effect(() => {
    void order
    if (!loaded || !items.length || problem) {
      forecast = null
      return
    }
    const n = ++asked
    forecasting = true
    const timer = setTimeout(() => {
      void PreviewRebasePlan(path, base, toRun($state.snapshot(items)) as git.RebaseItem[])
        .then((f) => {
          if (n === asked) forecast = f
        })
        .catch(() => {
          if (n === asked) forecast = null
        })
        .finally(() => {
          if (n === asked) forecasting = false
        })
    }, 300)
    return () => clearTimeout(timer)
  })
  let droppedBy = $derived(new Map((forecast?.dropped ?? []).map((d) => [d.commit, d])))
  const filesOf = (d: app.DroppedCommit) => (d.files.length <= 2 ? d.files.join(' and ') : `${d.files[0]} and ${d.fileCount - 1} other files`)

  // Rows, focus and the keys that work on the focused commit.
  let list = $state<HTMLOListElement>()
  const rowAt = (i: number) => list?.children[i] as HTMLElement | undefined

  function choose(i: number, action: Action) {
    items = setAction(items, i, action)
    open = -1
    if (action === 'reword') void tick().then(() => rowAt(i)?.querySelector<HTMLInputElement>('input.subject')?.select())
  }

  async function shift(i: number, by: -1 | 1) {
    const to = i + by
    if (to < 0 || to >= items.length) return
    items = move(items, i, to)
    await tick()
    rowAt(to)?.focus()
  }

  /** Listens for keys on a row. */
  function keys(node: HTMLElement, handler: (e: KeyboardEvent) => void) {
    let current = handler
    const listen = (e: KeyboardEvent) => current(e)
    node.addEventListener('keydown', listen)
    return {
      update: (next: (e: KeyboardEvent) => void) => void (current = next),
      destroy: () => node.removeEventListener('keydown', listen),
    }
  }

  function rowKey(e: KeyboardEvent, i: number) {
    // Keys typed into a field are text, not commands.
    if (e.target !== e.currentTarget) return
    if (e.altKey && (e.key === 'ArrowUp' || e.key === 'ArrowDown')) {
      e.preventDefault()
      void shift(i, e.key === 'ArrowUp' ? -1 : 1)
    } else if (e.key === 'ArrowUp' || e.key === 'ArrowDown') {
      e.preventDefault()
      rowAt(i + (e.key === 'ArrowUp' ? -1 : 1))?.focus()
    } else if (!e.ctrlKey && !e.metaKey && !e.altKey && e.key.length === 1) {
      const action = actionForKey(e.key)
      if (action) {
        e.preventDefault()
        choose(i, action)
      }
    }
  }

  // The select: one open at a time, closed by a choice, Escape, or a click elsewhere.
  let open = $state(-1)
  async function toggle(i: number) {
    open = open === i ? -1 : i
    if (open < 0) return
    await tick()
    rowAt(i)?.querySelector<HTMLElement>('[role=option][aria-selected=true]')?.focus()
  }

  function menuKey(e: KeyboardEvent, i: number) {
    const options = [...(e.currentTarget as HTMLElement).querySelectorAll<HTMLElement>('[role=option]')]
    const at = options.indexOf(document.activeElement as HTMLElement)
    if (e.key === 'Escape') {
      e.preventDefault()
      e.stopPropagation()
      open = -1
      rowAt(i)?.querySelector<HTMLElement>('.sel')?.focus()
    } else if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault()
      options[(at + (e.key === 'ArrowDown' ? 1 : -1) + options.length) % options.length]?.focus()
    } else if (e.key.length === 1 && actionForKey(e.key)) {
      e.preventDefault()
      choose(i, actionForKey(e.key)!)
      rowAt(i)?.querySelector<HTMLElement>('.sel')?.focus()
    }
  }

  // Dragging by the grip: the row follows the pointer one place at a time.
  let dragging = $state(-1)
  function dragMove(e: PointerEvent) {
    if (dragging < 0 || !list) return
    const rows = [...list.children] as HTMLElement[]
    const over = rows.findIndex((r) => {
      const box = r.getBoundingClientRect()
      return e.clientY >= box.top && e.clientY < box.bottom
    })
    if (over >= 0 && over !== dragging) {
      items = move(items, dragging, over)
      dragging = over
    }
  }

  // A reword edits the subject in place; the rest of the message opens on demand.
  let expanded = $state<Record<string, boolean>>({})
  const subjectOf = (message: string) => message.split('\n', 1)[0]
  function setSubject(it: PlanItem, subject: string) {
    const rest = it.message.includes('\n') ? it.message.slice(it.message.indexOf('\n')) : ''
    it.message = subject + rest
  }

  function subjectKey(e: KeyboardEvent, it: PlanItem, i: number) {
    if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
      e.preventDefault()
      expanded[it.commit] = true
      void tick().then(() => rowAt(i)?.querySelector('textarea')?.focus())
    } else if (e.key === 'Enter') {
      e.preventDefault()
      rowAt(i)?.focus()
    } else if (e.key === 'Escape') {
      e.preventDefault()
      e.stopPropagation()
      setSubject(it, it.subject)
      rowAt(i)?.focus()
    }
  }

  function reset() {
    items = fresh.map((it) => ({ ...it }))
    expanded = {}
    open = -1
  }

  async function start() {
    if (busy || problem || !items.length) return
    busy = true
    error = ''
    try {
      await InteractiveRebase(path, base, toRun($state.snapshot(items)) as git.RebaseItem[])
      onClose()
    } catch (e) {
      // Stopped on a conflict: the rebase is under way, and the strip above takes it from here.
      const state = await RepoState(path).catch(() => null)
      if (state?.operation === 'rebase') onClose()
      else error = errorText(e)
    } finally {
      busy = false
    }
  }
</script>

<svelte:window
  onpointerdown={(e) => {
    if (open >= 0 && !(e.target as HTMLElement).closest?.('.pick')) open = -1
  }}
/>

<div class="rebase">
  <header>
    <button class="btn sm quiet back" onclick={onClose} title="Cancel and return to the graph"><Icon name="back" />Back</button>
    <div class="title">
      <span class="hand">tidy it up</span>
      <h2>
        {#if loaded}Rewrite {count(items.length, 'commit', 'commits')}{:else}Rewrite commits{/if}
        {#if branch}on <span class="mono">{branch}</span>{/if}
      </h2>
    </div>
    <span class="note onto">{#if loaded}onto <span class="mono">{baseName}</span> · oldest first{:else}Loading commits…{/if}</span>
    <button class="btn sm quiet" disabled={!dirty || busy} onclick={reset}>Reset the plan</button>
    <button class="btn quiet" onclick={onClose}>Cancel</button>
    <button class="btn lg primary" disabled={busy || !!problem || !items.length} title={problem || undefined} onclick={start}>
      Rewrite as {count(after.commits.length, 'commit', 'commits')}
      <span class="arrow" aria-hidden="true">→</span>
    </button>
  </header>

  {#if error}<p class="form-error banner" role="alert">{error}</p>{:else if problem}<p class="warn" role="status">{problem}</p>{:else if loaded && !items.length}<p class="warn" role="status">There are no commits to rebase onto this one.</p>{/if}

  <div class="body">
    <div class="main">
      {#if loaded && items.length}
        <div class="hint">
          {#if canFold}
            <Icon name="info" size={16} />
            <span>{canFold === 1 ? '1 commit is a fixup for another.' : `${canFold} commits are fixups for others.`}</span>
            <button class="btn sm text" onclick={() => (items = foldIn(items))}>Fold {canFold === 1 ? 'it' : 'them'} in for me</button>
          {/if}
          <span class="note keys">Drag to reorder · <kbd>Alt</kbd>+<kbd>↑</kbd><kbd>↓</kbd> moves a commit · a letter sets what happens to it</span>
        </div>
      {/if}
      <ol class="plan" bind:this={list} class:dragging={dragging >= 0}>
        {#each items as it, i (it.commit)}
          {@const into = foldTarget(items, i)}
          {@const folds = it.action === 'squash' || it.action === 'fixup'}
          {@const gone = droppedBy.get(it.commit)}
          <!-- A row takes focus so its keys work on it: a letter, Alt and an arrow. -->
          <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
          <li class={[it.action, folds && into >= 0 && 'child', dragging === i && 'held']} tabindex="0" aria-label="{i + 1}. {actionLabel(it.action)}: {it.subject}" use:keys={(e) => rowKey(e, i)}>
            <span
              class="grip"
              title="Drag to reorder"
              aria-hidden="true"
              onpointerdown={(e) => {
                dragging = i
                e.currentTarget.setPointerCapture(e.pointerId)
              }}
              onpointermove={dragMove}
              onpointerup={() => (dragging = -1)}
              onpointercancel={() => (dragging = -1)}><i></i><i></i><i></i><i></i><i></i><i></i></span
            >
            <span class="pick">
              <button class="sel a-{it.action}" aria-haspopup="listbox" aria-expanded={open === i} aria-label="What to do with {it.subject}: {actionLabel(it.action)}" onclick={() => toggle(i)}>
                {actionLabel(it.action)}
                <span class="caret" class:up={open === i}><Icon name="chevron" size={14} /></span>
              </button>
              {#if open === i}
                <div class="menu" role="listbox" tabindex="-1" aria-label="What to do with this commit" onkeydown={(e) => menuKey(e, i)}>
                  {#each ACTIONS as a (a.action)}
                    <button role="option" aria-selected={it.action === a.action} class:danger={a.action === 'drop'} onclick={() => choose(i, a.action)}>
                      <b>{a.label}</b><span>{a.does}</span><kbd>{a.key}</kbd>
                    </button>
                  {/each}
                </div>
              {/if}
            </span>
            <div class="what">
              {#if it.action === 'reword'}
                <input
                  class="field subject"
                  value={subjectOf(it.message)}
                  oninput={(e) => setSubject(it, e.currentTarget.value)}
                  onkeydown={(e) => subjectKey(e, it, i)}
                  aria-label="New subject for {it.subject}"
                  spellcheck="false"
                  autocomplete="off"
                />
                <span class="note">
                  was “{it.subject}”. Esc puts it back.
                  {#if !expanded[it.commit]}<button class="btn sm text" onclick={() => (expanded[it.commit] = true)}>Edit the whole message</button>{/if}
                </span>
              {:else}
                <span class="subject">{it.subject}</span>
                {#if it.action === 'fixup' && into >= 0}
                  <span class="note">folds into the commit above, message discarded</span>
                {:else if it.action === 'squash' && into >= 0}
                  <span class="note">
                    folds into the commit above, keeping both messages
                    {#if !expanded[it.commit]}<button class="btn sm text" onclick={() => (expanded[it.commit] = true)}>Edit the message</button>{/if}
                  </span>
                {:else if it.action === 'drop'}
                  <span class="note">
                    {#if gone?.fileCount}its changes to {filesOf(gone)} are thrown away{:else}its changes are thrown away{/if}
                  </span>
                {/if}
              {/if}
              {#if (it.action === 'reword' || it.action === 'squash') && expanded[it.commit]}
                <label class="form-field">
                  <span class="visually-hidden">New message for {it.subject}</span>
                  <textarea bind:value={it.message} rows="4" spellcheck="false"></textarea>
                </label>
              {/if}
            </div>
            <span class="sha">{short(it.commit)}</span>
          </li>
        {/each}
      </ol>
    </div>

    {#if loaded && items.length}
      <aside class="after" aria-label="What the branch will look like">
        <div>
          <span class="hand">afterwards</span>
          <h3>
            {#if after.commits.length === items.length}{count(items.length, 'commit', 'commits')}, as now{:else}{count(after.commits.length, 'commit', 'commits')}, where there were {items.length}{/if}
          </h3>
        </div>
        <ol class="result">
          {#each after.commits as c (c.commit)}
            <li>
              <span class="dot" aria-hidden="true"></span>
              <span class="text">
                <span class="subject">{c.subject}</span>
                {#if c.made > 1}<span class="note">{c.made} commits in one</span>{/if}
                {#if c.was}<span class="note">was “{c.was}”</span>{/if}
              </span>
              {#if c.made > 1}<span class="tag plum">folded</span>{/if}
              {#if c.was}<span class="tag">reworded</span>{/if}
            </li>
          {/each}
        </ol>

        {#if after.dropped.length}
          <div class="card">
            <strong>{count(after.dropped.length, 'commit', 'commits')} dropped</strong>
            {#each after.dropped as d (d.commit)}
              {@const gone = droppedBy.get(d.commit)}
              <p>
                <span class="mono">{short(d.commit)}</span>
                {d.subject}{#if gone?.fileCount}: {count(gone.added + gone.deleted, 'line', 'lines')} in {filesOf(gone)} {gone.added + gone.deleted === 1 ? 'goes' : 'go'} with it.{/if}
              </p>
            {/each}
            <p class="note">Undo brings the whole branch back as it was.</p>
          </div>
        {/if}

        {#if problem}
          <!-- The plan can't run yet, so there's nothing to forecast. -->
        {:else if forecast?.stopsAt}
          <div class="card warn" role="status">
            <div class="head">
              <span class="ic warn"><Icon name="warning" size={20} /></span>
              <strong>It will stop at {items.find((it) => it.commit === forecast?.stopsAt)?.subject ?? short(forecast.stopsAt)}</strong>
            </div>
            <ul>
              {#each forecast.conflicts as c (c.path)}
                <li><span class="mono">{c.path}</span>{#if c.places}<span class="note">{count(c.places, 'place', 'places')}</span>{/if}</li>
              {/each}
            </ul>
            <p class="note">You can resolve it when it stops, or try another order. Later commits can't be tried until this one is settled.</p>
          </div>
        {:else if forecast && !forecast.incomplete}
          <div class="card ok" role="status">
            <div class="head">
              <span class="ic ok"><Icon name="check" size={20} /></span>
              <span>No conflicts expected in this order.</span>
            </div>
          </div>
        {:else if forecast || !forecasting}
          <div class="card" role="status"><span class="note">Conflicts couldn't be checked ahead of time. If there are any, the rebase stops and shows them.</span></div>
        {:else}
          <div class="card" role="status"><span class="note">Checking this order for conflicts…</span></div>
        {/if}

        {#if forecast?.pushed}
          <div class="card danger">
            <strong>This branch is already pushed</strong>
            <p>Its commits get new ids, so <span class="mono">{forecast.upstream}</span> will need a force push afterwards.</p>
          </div>
        {:else if forecast}
          <p class="note foot">These commits aren't pushed, so nobody else has them.</p>
        {/if}
      </aside>
    {/if}
  </div>
</div>

<style>
  .rebase {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
    container: rebase / inline-size;
  }

  /* The title gives up its width, so the buttons stay together on one line. */
  header {
    display: flex;
    align-items: center;
    gap: 8px 10px;
    padding: 10px 16px;
    border-bottom: 1px solid var(--paper-edge);
  }

  header > button {
    flex: none;
  }

  .title {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .title .hand {
    line-height: 1.1;
    white-space: nowrap;
  }

  h2,
  h3 {
    margin: 0;
    font-weight: 500;
    letter-spacing: -0.01em;
  }

  h2 {
    font-size: 20px;
    line-height: 1.3;
    padding-top: 2px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  h3 {
    font-size: 18px;
  }

  .mono,
  .sha {
    font-family: var(--font-mono);
    font-size: 13px;
  }

  .note {
    font-size: 13px;
    color: var(--ink-mute);
  }

  .onto {
    flex: none;
  }

  .arrow {
    font-family: var(--font-hand);
    font-size: 22px;
    line-height: 1;
  }

  .banner,
  .warn {
    margin: 0;
    padding: 8px 16px;
    border-bottom: 1px solid var(--paper-edge);
  }

  .warn {
    background: var(--warning-wash);
    color: var(--ink-soft);
  }

  .body {
    flex: 1;
    min-height: 0;
    display: flex;
  }

  .main {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    overflow-y: auto;
  }

  .hint {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 6px 10px;
    min-height: 40px;
    padding: 4px 16px;
    border-bottom: 1px solid var(--paper-edge);
    background: var(--paper-warm);
    color: var(--ink-soft);
  }

  .keys {
    margin-left: auto;
  }

  kbd {
    padding: 0 5px;
    border-radius: var(--r-sm);
    background: var(--paper-deep);
    font-family: var(--font-mono);
    font-size: 12px;
    line-height: 18px;
    color: var(--ink-soft);
  }

  .plan {
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .plan li {
    display: grid;
    grid-template-columns: 22px 132px minmax(0, 1fr) auto;
    align-items: center;
    gap: 14px;
    min-height: 50px;
    padding: 6px 14px;
    border-top: 1px dashed var(--hairline);
  }

  .plan li:first-child {
    border-top: none;
  }

  .plan li:focus-visible {
    outline: none;
    background: var(--selected);
    box-shadow: inset 3px 0 0 var(--accent);
  }

  .plan li.child {
    padding-left: 44px;
    background: var(--wash-hover);
  }

  .plan li.held {
    background: var(--selected);
  }

  .plan.dragging {
    cursor: grabbing;
    user-select: none;
  }

  .grip {
    display: grid;
    grid-template-columns: repeat(2, 3px);
    gap: 3px;
    justify-content: center;
    padding: 8px 4px;
    cursor: grab;
    touch-action: none;
  }

  .grip i {
    width: 3px;
    height: 3px;
    border-radius: 50%;
    background: var(--ink-faint);
  }

  .pick {
    position: relative;
  }

  .sel {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    width: 132px;
    height: 32px;
    padding: 0 8px 0 12px;
    border: 1px solid var(--paper-edge);
    border-radius: var(--r-md);
    background: var(--paper);
    font-family: var(--font-serif);
    font-size: 14px;
    font-weight: 500;
    color: var(--ink);
  }

  .sel[aria-expanded='true'] {
    border-color: var(--link);
    box-shadow: 0 0 0 3px color-mix(in srgb, var(--link) 22%, transparent);
  }

  .sel.a-squash,
  .sel.a-fixup {
    color: var(--accent-b);
  }

  .sel.a-reword {
    color: var(--accent-a);
  }

  .sel.a-drop {
    color: var(--danger);
  }

  .caret {
    display: inline-flex;
    color: var(--ink-mute);
    transition: transform var(--dur-quick);
  }

  .caret.up {
    transform: rotate(180deg);
  }

  .menu {
    position: absolute;
    left: 0;
    top: 36px;
    z-index: 5;
    width: 340px;
    max-width: 80vw;
    display: flex;
    flex-direction: column;
    padding: 4px;
    border: 1px solid var(--paper-edge);
    border-radius: var(--r-md);
    background: var(--paper);
    box-shadow: 0 10px 36px rgba(26, 22, 20, 0.18);
  }

  .menu button {
    display: grid;
    grid-template-columns: 72px 1fr auto;
    align-items: baseline;
    gap: 10px;
    min-height: 36px;
    padding: 6px 10px;
    border: none;
    border-radius: var(--r-sm);
    background: none;
    text-align: left;
    font-family: var(--font-serif);
    font-size: 14px;
    color: var(--ink);
  }

  .menu button:hover,
  .menu button:focus-visible {
    background: var(--wash-hover);
    outline: none;
  }

  .menu button[aria-selected='true'] {
    background: var(--selected);
  }

  .menu b {
    font-weight: 500;
  }

  .menu button.danger b {
    color: var(--danger);
  }

  .menu button > span {
    font-size: 13px;
    color: var(--ink-mute);
  }

  .what {
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .what .subject {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  li.drop .what .subject {
    text-decoration: line-through;
    color: var(--ink-faint);
  }

  li.child .what .subject {
    color: var(--ink-soft);
  }

  input.subject {
    width: 100%;
  }

  textarea {
    width: 100%;
    font-family: var(--font-mono);
    font-size: 13px;
  }

  .sha {
    color: var(--ink-mute);
  }

  .after {
    flex: none;
    width: 360px;
    display: flex;
    flex-direction: column;
    gap: 14px;
    padding: 18px 20px;
    border-left: 1px solid var(--paper-edge);
    overflow-y: auto;
  }

  .result {
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .result li {
    display: flex;
    align-items: center;
    gap: 10px;
    min-height: 44px;
    padding: 4px 0;
    border-top: 1px dashed var(--hairline);
  }

  .result li:first-child {
    border-top: none;
  }

  .dot {
    flex: none;
    width: 10px;
    height: 10px;
    border-radius: 50%;
    background: var(--accent-a);
  }

  .result .text {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
  }

  .result .subject {
    overflow-wrap: anywhere;
  }

  .card {
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding: 12px 14px;
    border: 1px solid var(--paper-edge);
    border-radius: var(--r-md);
  }

  .card p,
  .card ul {
    margin: 0;
    padding: 0;
    list-style: none;
    color: var(--ink-soft);
    overflow-wrap: anywhere;
  }

  .card li {
    display: flex;
    justify-content: space-between;
    gap: 10px;
  }

  .card strong {
    font-weight: 600;
  }

  .card.warn {
    border-color: var(--warning);
    background: var(--warning-wash);
  }

  .card.ok {
    border-color: var(--success);
    background: var(--success-wash);
  }

  .card.danger {
    border-color: var(--danger);
    background: var(--danger-wash);
  }

  .head {
    display: flex;
    align-items: center;
    gap: 10px;
  }

  .ic {
    display: inline-flex;
    flex: none;
  }

  .ic.warn {
    color: var(--warning);
  }

  .ic.ok {
    color: var(--success);
  }

  .foot {
    margin: auto 0 0;
  }

  /* In a narrow column the branch-afterwards panel goes under the plan. */
  @container rebase (max-width: 820px) {
    .body {
      flex-direction: column;
      overflow-y: auto;
    }

    .main {
      flex: none;
      overflow: visible;
    }

    .after {
      width: auto;
      border-left: none;
      border-top: 1px solid var(--paper-edge);
      overflow: visible;
    }

    .keys,
    .onto {
      display: none;
    }
  }

  /* Cancel goes back too, so the Back button is the one to give way. */
  @container rebase (max-width: 620px) {
    .back {
      display: none;
    }
  }
</style>
