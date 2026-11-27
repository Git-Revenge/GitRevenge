<script lang="ts">
  // The Repository panel's Storage section (#937): what the objects take on disk and how
  // they're kept, git maintenance on request, and whether git's schedule keeps it tidy. It
  // suggests maintenance only when git's own thresholds say so; otherwise it only offers it.
  // Maintenance runs as a fetch does: progress and Stop in the toolbar.
  import { RepoStorage, RunMaintenance, ScheduleMaintenance } from '../../wailsjs/go/app/App.js'
  import type { app } from '../../wailsjs/go/models'
  import type { DialogSpec } from './Dialog.svelte'
  import { errorText } from './errors'
  import { network } from './network.svelte'
  import { session } from './session.svelte'
  import { dueSaid, garbageSaid, storageLines } from './thisClone'

  let { path, openDialog }: { path: string; openDialog: (spec: DialogSpec) => void } = $props()

  let storage = $state<app.RepoStorage | null>(null)
  let error = $state('')
  async function load() {
    try {
      storage = await RepoStorage(path)
      error = ''
    } catch (e) {
      error = errorText(e)
    }
  }
  $effect(() => {
    void path
    storage = null
    void load()
  })

  let busy = $derived(!!network.running[path])
  let lines = $derived(storage ? storageLines(storage) : null)

  async function runMaintenance() {
    await network.run(path, 'maintenance', () => RunMaintenance(path))
    await load()
  }

  function schedule() {
    openDialog({
      title: 'Put it on git’s maintenance schedule?',
      message:
        'git maintenance start adds this repository to the list in your global git config, and has your system’s scheduler (Task Scheduler, launchd, or cron or systemd) run git maintenance on every repository in the list: hourly, daily and weekly, in the background. git then keeps the repository tidy as it grows, so every git command stays quick.',
      submit: 'Put it on the schedule',
      run: async () => {
        const done = await session.run(async () => {
          await ScheduleMaintenance(path, true)
          return true
        })
        if (done) await load()
      },
    })
  }

  async function unschedule() {
    const done = await session.run(async () => {
      await ScheduleMaintenance(path, false)
      return true
    })
    if (done) await load()
  }
</script>

<section aria-labelledby="storage-{path}">
  <div class="head">
    <h3 class="meta" id="storage-{path}">Storage</h3>
  </div>
  {#if error}
    <p class="note" role="alert">Its storage couldn’t be read: {error}</p>
  {:else if !storage || !lines}
    <p class="note">Reading…</p>
  {:else}
    <ul class="parts">
      <li>
        <p class="what">{lines.what}</p>
        <p class="kind">{lines.tag}</p>
        {#if dueSaid(storage)}<p class="due" role="status">{dueSaid(storage)}</p>{/if}
        {#if garbageSaid(storage)}<p class="hint">{garbageSaid(storage)}</p>{/if}
        <div class="acts">
          <button class={['btn', 'sm', storage.due ? 'secondary' : 'quiet']} disabled={busy} onclick={() => void runMaintenance()} title="git maintenance run: the tasks your git config enables, or gc"
            >Run maintenance</button
          >
        </div>
      </li>
      <li>
        <p class="what">{storage.registered ? 'On git’s maintenance schedule' : 'Not on git’s maintenance schedule'}</p>
        <p class="kind">git maintenance</p>
        {#if storage.registered}<p class="hint">git runs maintenance on it hourly, daily and weekly.</p>{/if}
        <div class="acts">
          {#if storage.registered}
            <button class="btn sm quiet" disabled={busy} onclick={() => void unschedule()} title="git maintenance unregister">Take it off the schedule</button>
          {:else}
            <button class="btn sm quiet" disabled={busy} aria-haspopup="dialog" onclick={schedule} title="git maintenance start">Put it on the schedule…</button>
          {/if}
        </div>
      </li>
    </ul>
  {/if}
</section>

<style>
  section {
    border-bottom: 1px dotted var(--paper-edge);
    margin-bottom: 4px;
    padding-bottom: 4px;
  }

  .head {
    display: flex;
    align-items: center;
    min-height: 32px;
    padding: 0 6px 0 10px;
  }

  h3 {
    margin: 0;
    font-weight: 400;
    color: var(--ink-soft);
  }

  .parts {
    list-style: none;
    margin: 0;
    padding: 0 10px 6px;
    display: flex;
    flex-direction: column;
  }

  .parts li {
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: 6px 0 9px;
  }

  .parts li + li {
    border-top: 1px dashed var(--paper-edge);
  }

  p {
    margin: 0;
  }

  .what {
    font-size: 14px;
    color: var(--ink);
    line-height: 1.35;
  }

  .kind {
    font-family: var(--font-mono);
    font-size: 12px;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--ink-mute);
  }

  .due,
  .hint,
  .note {
    font-size: 13px;
    line-height: 1.45;
    color: var(--ink-soft);
  }

  .due {
    color: var(--accent-b);
  }

  .note {
    padding: 0 10px 8px;
  }

  .acts {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    margin-top: 4px;
  }
</style>
