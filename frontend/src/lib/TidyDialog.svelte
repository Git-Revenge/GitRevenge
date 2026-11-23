<script lang="ts">
  // Tidy (#924): a remote's branches that are dead, in three kinds, with a line on each saying
  // what deleting it does. Only gone branches start ticked: nothing that deletes for other
  // people is ever ticked for you. Deleting on the remote is a push, one per branch.
  import { PushRef, RemoteTidy, SetQuietDays, TidyHere } from '../../wailsjs/go/app/App.js'
  import type { app } from '../../wailsjs/go/models'
  import { errorText } from './errors'
  import { fetchAge } from './remoteHeader'
  import Empty from './ui/Empty.svelte'
  import InkButton from './ui/InkButton.svelte'
  import Modal from './ui/Modal.svelte'

  let { path, remote, onClose, onDone }: { path: string; remote: string; onClose: () => void; onDone: () => void } = $props()

  const WINDOWS = [
    { days: 0, label: 'Off' },
    { days: 90, label: '3 months' },
    { days: 182, label: '6 months' },
    { days: 365, label: 'A year' },
  ]

  let report = $state<app.TidyReport | null>(null)
  let ticked = $state<Record<string, boolean>>({})
  let error = $state('')
  let running = $state(false)
  let results = $state<{ name: string; ok: boolean; says: string }[] | null>(null)

  async function load() {
    try {
      report = await RemoteTidy(path, remote)
      ticked = Object.fromEntries(report.items.map((it) => [it.name, it.ticked]))
      error = ''
    } catch (e) {
      error = errorText(e)
    }
  }
  void load()

  async function setWindow(days: number) {
    try {
      await SetQuietDays(path, days)
    } catch (e) {
      error = errorText(e)
    }
    await load()
  }

  const of = (kind: string) => report?.items.filter((it) => it.kind === kind) ?? []
  let count = $derived(Object.values(ticked).filter(Boolean).length)
  const now = Date.now() / 1000
  const commits = (n: number) => `${n} ${n === 1 ? 'commit' : 'commits'}`

  /** What deleting a branch does, in one line. */
  function says(it: app.TidyItem): string {
    if (it.local) {
      return it.unpushed
        ? `Its upstream was deleted. ${commits(it.unpushed)} on it ${it.unpushed === 1 ? "isn't" : "aren't"} on any remote: deleting it leaves ${it.unpushed === 1 ? 'that' : 'them'} only in the reflog.`
        : 'Its upstream was deleted, and everything on it is on a remote. Deletes the local branch only.'
    }
    if (it.stale) return `${remote} no longer has it. Deletes the leftover here only.`
    const last = it.lastDate ? `Last commit ${fetchAge(it.lastDate, now)}${it.lastAuthor ? `, by ${it.lastAuthor}` : ''}.` : ''
    if (it.kind === 'merged') return `Every commit is on ${remote}/${report?.base}. Deleting it removes it from ${remote} for everyone. ${last}`
    const only = it.onlyHere ? ` ${commits(it.onlyHere)} ${it.onlyHere === 1 ? 'is' : 'are'} on no other branch and would live only in the reflog.` : ' Every commit is on another branch too.'
    return `${last} Deleting it removes it from ${remote} for everyone.${only}`
  }

  async function tidy() {
    if (!report) return
    running = true
    const picked = report.items.filter((it) => ticked[it.name])
    const out: { name: string; ok: boolean; says: string }[] = []
    try {
      const locals = picked.filter((it) => it.local).map((it) => it.name)
      const stale = picked.filter((it) => it.stale).map((it) => it.name)
      if (locals.length || stale.length) {
        for (const r of await TidyHere(path, locals, stale)) out.push({ name: r.name, ok: !r.error, says: r.error || (locals.includes(r.name) ? 'Deleted here' : 'Leftover removed') })
      }
      // On the remote: a push each, through the same flow as a branch's own Delete.
      for (const it of picked.filter((x) => !x.local && !x.stale)) {
        const branch = it.name.slice(remote.length + 1)
        try {
          await PushRef(path, remote, 'delete-branch', branch)
          out.push({ name: it.name, ok: true, says: `Deleted from ${remote}` })
        } catch (e) {
          out.push({ name: it.name, ok: false, says: errorText(e) })
        }
      }
    } catch (e) {
      error = errorText(e)
    }
    results = out
    running = false
    onDone()
  }

  const sections = [
    { kind: 'gone', title: 'Gone', note: 'Deleted on the remote. Tidying removes only what’s left here.' },
    { kind: 'merged', title: 'Merged', note: 'Already on the default branch. Never ticked for you: deleting removes it for everyone.' },
    { kind: 'quiet', title: 'Quiet', note: 'No commits for a while. Never ticked for you.' },
  ]
</script>

<Modal title="Tidy {remote}" eyebrow="sweep up" width={620} scrimCloses={!running} dismissable={!running} {onClose}>
  {#if error}<p class="form-error" role="alert">{error}</p>{/if}
  {#if results}
    <ul class="results" aria-label="What happened">
      {#each results as r (r.name)}
        <li class:failed={!r.ok}><span class="name">{r.name}</span><span class="state">{r.ok ? 'DONE' : 'FAILED'}</span><span class="says">{r.says}</span></li>
      {/each}
    </ul>
    <div class="form-actions"><InkButton onclick={onClose}>Close</InkButton></div>
  {:else if report}
    {#each sections as sec (sec.kind)}
      {@const items = of(sec.kind)}
      <section>
        <h3 class="meta">{sec.title} <span class="n">{items.length}</span></h3>
        {#if sec.kind === 'quiet'}
          <label class="window">
            <span>Quiet after</span>
            <select value={report.quietDays} onchange={(e) => setWindow(Number(e.currentTarget.value))}>
              {#each WINDOWS as w (w.days)}<option value={w.days}>{w.label}</option>{/each}
              {#if !WINDOWS.some((w) => w.days === report?.quietDays)}<option value={report.quietDays}>{report.quietDays} days</option>{/if}
            </select>
          </label>
        {/if}
        <p class="note">{sec.kind === 'quiet' && !report.quietDays ? 'Choose how long counts as quiet. None is assumed for you.' : sec.note}</p>
        {#each items as it (it.name)}
          <label class="item">
            <input type="checkbox" bind:checked={ticked[it.name]} disabled={running} />
            <span class="t"><span class="name">{it.name}</span><span class="says">{says(it)}</span></span>
          </label>
        {/each}
      </section>
    {/each}
    {#if report.items.length === 0}<Empty note="all tidy" compact>Nothing on {remote} needs tidying.</Empty>{/if}
    {#if !report.base}<p class="note">Git doesn't know {remote}'s default branch, so nothing counts as merged. <code>git remote set-head {remote} --auto</code> asks it.</p>{/if}
    <div class="form-actions">
      <InkButton variant="secondary" onclick={onClose} disabled={running}>Cancel</InkButton>
      <InkButton onclick={tidy} disabled={count === 0 || running} busy={running}>Tidy {count} {count === 1 ? 'branch' : 'branches'}</InkButton>
    </div>
  {/if}
</Modal>

<style>
  section {
    margin-bottom: 14px;
  }

  h3 {
    display: flex;
    align-items: baseline;
    gap: 8px;
    margin: 0 0 2px;
  }

  .n {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-mute);
  }

  .note {
    margin: 0 0 6px;
    font-size: 13px;
    color: var(--ink-soft);
  }

  .window {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    font-size: 13px;
    margin-bottom: 4px;
  }

  .item {
    display: grid;
    grid-template-columns: 20px minmax(0, 1fr);
    gap: 8px;
    padding: 6px 0;
    border-bottom: 1px dashed var(--hairline);
  }

  .item input {
    accent-color: var(--accent);
    margin: 2px 0 0;
  }

  .t {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }

  .name {
    font-family: var(--font-mono);
    font-size: 13px;
    overflow-wrap: anywhere;
  }

  .says {
    font-size: 13px;
    color: var(--ink-soft);
  }

  .results {
    list-style: none;
    margin: 0;
    padding: 0;
  }

  .results li {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 2px 12px;
    padding: 6px 0;
    border-bottom: 1px dashed var(--hairline);
  }

  .results .says {
    grid-column: 1 / -1;
  }

  .state {
    font-family: var(--font-mono);
    font-size: 12px;
    letter-spacing: 0.06em;
  }

  .failed .state {
    color: var(--danger);
  }
</style>
