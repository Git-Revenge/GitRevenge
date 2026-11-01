<script lang="ts">
  import Icon from './ui/Icon.svelte'
  import { reveal } from './ui/motion'
  import { LFS, LFSAction, LFSLocks, Submodules, UpdateSubmodules } from '../../wailsjs/go/app/App.js'
  import type { app, git } from '../../wailsjs/go/models'
  import type { DialogSpec } from './Dialog.svelte'
  import { errorText } from './errors'
  import { session } from './session.svelte'

  let {
    path,
    openDialog,
    onStatus,
  }: {
    path: string
    openDialog: (spec: DialogSpec) => void
    /** Reports how many submodules need updating, for the rail's badge, and whether there's
     * anything to show: false when there are no submodules and LFS isn't available. */
    onStatus?: (status: { attention: number; hasContent: boolean }) => void
  } = $props()

  let subs = $state<git.Submodule[]>([])
  let lfs = $state<app.LFSInfo | null>(null)
  /** Locks from the server, by file path; null until checked (it needs the network). */
  let locks = $state<Record<string, git.LFSLock> | null>(null)
  let open = $state<Record<string, boolean>>({ subs: true, lfs: true, files: false })
  let busy = $state(false)

  const stateLabel: Record<string, string> = {
    ok: '',
    uninitialized: 'not initialized',
    modified: 'different commit',
    conflict: 'conflict',
  }

  async function load() {
    const [s, l] = await Promise.all([session.run(() => Submodules(path)), session.run(() => LFS(path))])
    subs = s ?? []
    lfs = l ?? null
    onStatus?.({ attention: subs.filter((x) => x.state !== 'ok').length, hasContent: subs.length > 0 || !!lfs?.available })
  }

  $effect(() => {
    void session.revision[path]
    void load()
  })

  async function run(fn: () => Promise<unknown>) {
    busy = true
    await session.run(fn)
    busy = false
    await load()
  }

  async function checkLocks() {
    busy = true
    const list = await session.run(() => LFSLocks(path))
    busy = false
    if (list) locks = Object.fromEntries(list.map((l) => [l.path, l]))
  }

  async function lockAction(file: string, action: 'lock' | 'unlock', force = false) {
    await run(() => LFSAction(path, action, [file, force ? 'true' : '']))
    await checkLocks()
  }

  function track() {
    openDialog({
      title: 'Track files with LFS',
      message: 'Matching files are stored in Git LFS from now on. This edits .gitattributes, which you then commit.',
      fields: [
        { key: 'pattern', label: 'Pattern', placeholder: '*.psd' },
        { key: 'lockable', label: 'Lockable (read-only until locked)', type: 'checkbox' },
      ],
      submit: 'Track',
      run: async (v) => {
        try {
          await LFSAction(path, 'track', [String(v.pattern).trim(), v.lockable ? 'true' : ''])
          await load()
        } catch (e) {
          return errorText(e)
        }
      },
    })
  }

  const join = (base: string, rel: string) => base.replace(/[\\/]+$/, '') + '/' + rel
</script>

{#if subs.length}
  <section>
    <div class="head">
      <button class="group" onclick={() => (open.subs = !open.subs)}>
        <span class="caret" class:open={open.subs}><Icon name="right" size={12} /></span> <span class="meta title">Submodules</span> <span class="meta count">{subs.length}</span>
      </button>
      <button class="btn sm quiet" disabled={busy} onclick={() => run(() => UpdateSubmodules(path))} title="Initialize and update all submodules"
        >Update</button
      >
    </div>
    {#if open.subs}
      <ul in:reveal out:reveal={{ out: true }}>
        {#each subs as s (s.path)}
          <li>
            <button class="item" onclick={() => session.open(join(path, s.path))} title="Open {s.path} as a tab ({s.commit.slice(0, 7)})">
              <span class="name">{s.path}</span>
              {#if stateLabel[s.state]}<span class="badge s-{s.state}">{stateLabel[s.state]}</span>{/if}
            </button>
          </li>
        {/each}
      </ul>
    {/if}
  </section>
{/if}

{#if lfs?.available}
  <section>
    <div class="head">
      <button class="group" onclick={() => (open.lfs = !open.lfs)}>
        <span class="caret" class:open={open.lfs}><Icon name="right" size={12} /></span> <span class="meta title">LFS</span> <span class="meta count">{lfs.status.files.length}</span>
      </button>
      <button class="btn sm quiet icon" onclick={track} title="Track a pattern with LFS" aria-label="Track a pattern with LFS"><Icon name="plus" size={13} /></button>
    </div>
    {#if open.lfs}
      <ul in:reveal out:reveal={{ out: true }}>
        {#each lfs.status.patterns as p (p.pattern)}
          <li class="row">
            <span class="name mono" title="from {p.source}">{p.pattern}</span>
            {#if p.lockable}<span class="badge">lockable</span>{/if}
            <button class="btn sm quiet icon" disabled={busy} onclick={() => run(() => LFSAction(path, 'untrack', [p.pattern]))} aria-label="Stop tracking {p.pattern}"
              ><Icon name="close" /></button
            >
          </li>
        {:else}
          <li class="muted pad">No patterns tracked.</li>
        {/each}
      </ul>
      {#if lfs.status.files.length}
        <div class="head">
          <button class="group sub" onclick={() => (open.files = !open.files)}>
            <span class="caret" class:open={open.files}><Icon name="right" size={12} /></span> <span class="meta title">Files</span> <span class="meta count">{lfs.status.files.length}</span>
          </button>
          <button class="btn sm quiet" disabled={busy} onclick={checkLocks} title="Ask the LFS server which files are locked">Check locks</button>
        </div>
        {#if open.files}
          <ul in:reveal out:reveal={{ out: true }}>
            {#each lfs.status.files as f (f.name)}
              {@const lock = locks?.[f.name]}
              <li class="row">
                <span class="name" title={f.name}>{f.name}</span>
                {#if !f.downloaded}<span class="badge" title="Only the pointer is here; content not downloaded">pointer</span>{/if}
                {#if lock}
                  <span class="badge" class:mine={lock.ours} title="Locked {lock.lockedAt}">locked · {lock.ours ? 'you' : lock.owner}</span>
                  <button class="btn sm quiet" disabled={busy} onclick={() => lockAction(f.name, 'unlock', !lock.ours)}
                    >{lock.ours ? 'Unlock' : 'Force unlock'}</button
                  >
                {:else if locks}
                  <button class="btn sm quiet" disabled={busy} onclick={() => lockAction(f.name, 'lock')}>Lock</button>
                {/if}
              </li>
            {/each}
          </ul>
        {/if}
      {/if}
    {/if}
  </section>
{/if}

<style>
  section {
    border-top: 1px dashed var(--hairline);
  }

  .head {
    display: flex;
    align-items: center;
    padding-right: 8px;
  }

  button {
    background: none;
    border: none;
    color: inherit;
    font: inherit;
    cursor: pointer;
  }

  .group {
    flex: 1;
    display: flex;
    align-items: center;
    gap: 8px;
    text-align: left;
    padding: 10px 12px 4px;
  }

  .group:hover {
    background: var(--wash-hover);
  }

  .group.sub {
    padding-left: 32px;
  }

  .title {
    color: var(--ink-soft);
  }

  .caret {
    display: grid;
    place-items: center;
    width: 12px;
    color: var(--ink-mute);
    transition: transform var(--dur-base) var(--ease-ink);
  }

  .caret.open {
    transform: rotate(90deg);
  }

  .count {
    margin-left: auto;
  }

  ul {
    list-style: none;
    margin: 0;
    padding: 0 0 4px;
  }

  .item,
  .row {
    display: flex;
    gap: 6px;
    align-items: center;
    width: 100%;
    padding: 3px 10px 3px 26px;
    text-align: left;
  }

  .item:hover {
    background: var(--wash-hover);
  }

  .name {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .mono {
    font-family: var(--font-mono);
  }

  .badge {
    font-family: var(--font-mono);
    font-size: 11px;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    padding: 0 5px;
    border-radius: 3px;
    background: var(--wash-hover);
    color: var(--ink-mute);
    white-space: nowrap;
  }

  .badge.mine {
    color: var(--success);
  }

  .s-uninitialized,
  .s-modified {
    color: var(--accent-c);
  }

  .s-conflict {
    color: var(--danger);
  }

  .muted {
    color: var(--ink-mute);
  }

  .pad {
    padding: 3px 26px;
  }
</style>
