<script lang="ts">
  import { InteractiveRebase, RebasePlan } from '../../wailsjs/go/app/App.js'
  import type { git } from '../../wailsjs/go/models'
  import { errorText } from './errors'
  import Icon from './ui/Icon.svelte'
  import InkButton from './ui/InkButton.svelte'

  let { path, base, onClose }: { path: string; base: string; onClose: () => void } = $props()

  type Action = 'pick' | 'reword' | 'squash' | 'fixup' | 'drop'
  interface Item {
    commit: string
    subject: string
    original: string
    message: string
    action: Action
  }

  // Shown newest first, like the graph; git wants oldest first, so it's reversed on start.
  let items = $state<Item[]>([])
  let error = $state('')
  let busy = $state(false)
  /** False until the plan has answered, so an empty list doesn't read as 0 commits. */
  let loaded = $state(false)

  async function load() {
    loaded = false
    try {
      const plan = (await RebasePlan(path, base)) ?? []
      items = plan
        .map((p) => ({ commit: p.commit, subject: p.subject, original: p.message, message: p.message, action: 'pick' as Action }))
        .reverse()
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

  function move(i: number, d: -1 | 1) {
    const j = i + d
    if (j < 0 || j >= items.length) return
    ;[items[i], items[j]] = [items[j], items[i]]
  }

  /** The commit a squash/fixup at display index i folds into (the next older kept one). */
  function target(i: number): Item | undefined {
    for (let j = i + 1; j < items.length; j++) if (items[j].action !== 'drop') return items[j]
  }

  function setAction(i: number, a: Action) {
    const it = items[i]
    it.action = a
    if (a === 'squash') {
      // Start from both messages, as git's squash does.
      const into = target(i)
      it.message = into ? `${into.original}\n\n${it.original}` : it.original
    } else if (a === 'reword') {
      it.message = it.original
    }
  }

  let problem = $derived.by(() => {
    if (items.length && items.every((it) => it.action === 'drop')) return 'Every commit is dropped. Use Reset instead.'
    for (let i = 0; i < items.length; i++) {
      const it = items[i]
      if ((it.action === 'squash' || it.action === 'fixup') && !target(i)) return 'The oldest kept commit can’t be squashed or fixed up: there’s nothing older to fold it into.'
      if ((it.action === 'reword' || it.action === 'squash') && !it.message.trim()) return `${it.subject} needs a message.`
    }
    return ''
  })

  async function start() {
    busy = true
    error = ''
    try {
      const plan = [...items].reverse().map((it) => ({ commit: it.commit, subject: it.subject, message: it.message, action: it.action }))
      await InteractiveRebase(path, base, plan as git.RebaseItem[])
      onClose()
    } catch (e) {
      error = errorText(e)
    } finally {
      busy = false
    }
  }

  const actions: Action[] = ['pick', 'reword', 'squash', 'fixup', 'drop']
</script>

<div class="rebase">
  <header>
    <button class="btn quiet back" onclick={onClose} title="Cancel and return to the graph"><Icon name="back" />Back</button>
    <div class="title">
      <h2>Interactive rebase</h2>
      <span class="meta">Onto {base.slice(0, 7)} · {#if loaded}{items.length} {items.length === 1 ? 'commit' : 'commits'} · newest first{:else}Loading commits…{/if}</span>
    </div>
    <InkButton variant="secondary" onclick={onClose}>Cancel</InkButton>
    <InkButton disabled={busy || !!problem || items.length === 0} onclick={start} title={problem}>Start rebase</InkButton>
  </header>
  {#if error}<p class="form-error banner" role="alert">{error}</p>{:else if problem}<p class="warn" role="status">{problem}</p>{:else if loaded && !items.length}<p class="warn" role="status">There are no commits to rebase onto this one.</p>{/if}

  <ol>
    {#each items as it, i (it.commit)}
      <li class={it.action}>
        <span class="move">
          <button class="btn sm quiet icon up" disabled={i === 0} onclick={() => move(i, -1)} aria-label="Move {it.subject} up" title="Move up"><Icon name="chevron" size={14} /></button>
          <button class="btn sm quiet icon" disabled={i === items.length - 1} onclick={() => move(i, 1)} aria-label="Move {it.subject} down" title="Move down"
            ><Icon name="chevron" size={14} /></button
          >
        </span>
        <select
          class="action a-{it.action}"
          value={it.action}
          onchange={(e) => setAction(i, (e.currentTarget as HTMLSelectElement).value as Action)}
          aria-label="Action for {it.subject}"
        >
          {#each actions as a (a)}<option value={a}>{a}</option>{/each}
        </select>
        <span class="sha">{it.commit.slice(0, 7)}</span>
        <div class="text">
          <span class="subject">{it.subject}</span>
          {#if it.action === 'squash' && target(i)}
            <span class="note">folds into {target(i)?.subject}</span>
          {:else if it.action === 'fixup' && target(i)}
            <span class="note">folds into {target(i)?.subject}, discarding this message</span>
          {/if}
          {#if it.action === 'reword' || it.action === 'squash'}
            <label class="form-field">
              <span class="visually-hidden">New message for {it.subject}</span>
              <textarea bind:value={it.message} rows="3"></textarea>
            </label>
          {/if}
        </div>
      </li>
    {/each}
  </ol>
</div>

<style>
  .rebase {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
  }

  header {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 12px 16px;
    border-bottom: 1px solid var(--paper-edge);
  }

  /* The title gives up its width, so the buttons stay together on one line. */
  header > :global(button) {
    flex: none;
  }

  .back {
    margin-right: 6px;
  }

  .title {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .title .meta {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  h2 {
    margin: 0;
    font-size: 20px;
    font-weight: 500;
    letter-spacing: -0.015em;
  }

  .banner {
    margin: 10px 16px 0;
  }

  .warn {
    margin: 10px 16px 0;
    padding: 8px 12px;
    border-radius: var(--r-sm);
    background: var(--warning-wash);
    font-size: 13px;
  }

  ol {
    list-style: none;
    margin: 0;
    padding: 4px 0;
    overflow-y: auto;
  }

  li {
    display: flex;
    align-items: flex-start;
    gap: 10px;
    padding: 10px 16px;
    border-bottom: 1px dotted var(--paper-edge);
  }

  li.drop .subject {
    text-decoration: line-through;
    color: var(--ink-mute);
  }

  .move {
    display: flex;
  }

  /* Up is the down arrow turned over. */
  .move .up {
    transform: rotate(180deg);
  }

  .move .up:hover:not(:disabled) {
    transform: rotate(180deg) translateY(1px);
  }

  .action {
    height: 28px;
    padding: 0 6px;
    font-family: var(--font-mono);
    font-size: 12px;
    border-radius: var(--r-sm);
  }

  .a-reword {
    color: var(--accent-a);
  }

  .a-squash,
  .a-fixup {
    color: var(--accent-b);
  }

  .a-drop {
    color: var(--danger);
  }

  .sha {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-mute);
    padding-top: 6px;
  }

  .text {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding-top: 3px;
  }

  .subject {
    font-size: 15px;
  }

  .note {
    font-size: 13px;
    font-style: italic;
    color: var(--ink-mute);
  }
</style>
