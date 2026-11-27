<script lang="ts">
  import { onMount, untrack } from 'svelte'
  import { CompareVersions, CountVersion, Reflog, Refs, VersionBase } from '../../wailsjs/go/app/App.js'
  import type { app } from '../../wailsjs/go/models'
  import { errorText } from './errors'
  import Busy from './ui/Busy.svelte'
  import InkButton from './ui/InkButton.svelte'
  import Modal from './ui/Modal.svelte'
  import { ago, tipLabel } from './versions'

  /** Pick two versions of a branch by hand (#1038): each a range, from where it starts to its
   * tip, then git range-diff. */
  let {
    path,
    branch,
    oldTip: startTip = '',
    onClose,
    onCompared,
  }: {
    path: string
    /** The branch to start from: its earlier tips are offered as the old version. */
    branch: string
    /** The old version's tip to start on, when it's known (#1040): its base couldn't be. */
    oldTip?: string
    onClose: () => void
    onCompared: (compared: app.VersionsCompared, oldLabel: string) => void
  } = $props()
  const uid = $props.id()

  type Choice = { value: string; label: string; said: string }
  /** The old version's offered tips: earlier tips from the branch's reflog, and its upstream. */
  let choices = $state<Choice[]>([])
  const OTHER = '\u0000other'
  let oldPick = $state(OTHER)
  let oldTip = $state('')
  let oldSince = $state('')
  let newTip = $state('')
  let newSince = $state('')
  let stackParent = ''
  let baseName = $state('')

  let oldCount = $state<app.VersionCount | null>(null)
  let newCount = $state<app.VersionCount | null>(null)
  let error = $state('')
  let busy = $state(false)

  const short = (id: string) => (/^[0-9a-f]{40}$/.test(id) ? id.slice(0, 7) : id)

  onMount(async () => {
    newTip = branch
    const [refs, log] = await Promise.all([Refs(path).catch(() => []), branch ? Reflog(path, branch).catch(() => []) : Promise.resolve([])])
    const ref = (refs ?? []).find((r) => r.kind === 'local' && r.short === branch)
    stackParent = ref?.stackParent ?? ''
    const out: Choice[] = []
    const entries = log ?? []
    // An entry's tip is what the entry before it (newer) moved the branch away from.
    for (let i = 1; i < entries.length && out.length < 8; i++) {
      const e = entries[i]
      if (e.commit === entries[0].commit || out.some((c) => c.value === e.commit)) continue
      // Named and dated by what moved the branch away from it: "before the rebase · 4 min ago".
      const left = entries[i - 1]
      const said = tipLabel(left.action)
      out.push({ value: e.commit, said: `${said[0].toUpperCase()}${said.slice(1)}`, label: `${short(e.commit)}, ${said} · ${ago(left.time)}` })
    }
    if (ref?.upstream && ref.behind + ref.ahead > 0) out.push({ value: ref.upstream, said: ref.upstream, label: ref.upstream })
    choices = out
    const start = untrack(() => startTip)
    if (start) {
      oldPick = out.some((c) => c.value === start) ? start : OTHER
      oldTip = start
    } else if (out.length) {
      oldPick = out[0].value
      oldTip = out[0].value
    }
    await fillBases()
  })

  /** Where each version starts: where its tip left the branch's base. */
  async function fillBases() {
    if (!branch) return
    const [o, n] = await Promise.all([oldTip ? VersionBase(path, branch, stackParent, oldTip).catch(() => '') : '', VersionBase(path, branch, stackParent, newTip).catch(() => '')])
    if (o) oldSince = short(o)
    if (n) newSince = short(n)
    baseName = stackParent || ''
  }

  function onPick() {
    if (oldPick === OTHER) oldTip = ''
    else {
      oldTip = oldPick
      void fillBases()
    }
  }

  // Each side counts its commits as it's filled in.
  let timer: ReturnType<typeof setTimeout> | undefined
  $effect(() => {
    const sides = [oldSince, oldTip, newSince, newTip]
    clearTimeout(timer)
    timer = setTimeout(async () => {
      const [os, ot, ns, nt] = sides
      oldCount = os && ot ? await CountVersion(path, os, ot).catch(() => null) : null
      newCount = ns && nt ? await CountVersion(path, ns, nt).catch(() => null) : null
    }, 250)
    return () => clearTimeout(timer)
  })

  const said = (n: app.VersionCount | null) =>
    !n ? '' : `${n.commits.toLocaleString()} ${n.commits === 1 ? 'commit' : 'commits'}${n.merges ? `, ${n.merges.toLocaleString()} ${n.merges === 1 ? 'merge' : 'merges'} left out` : ', no merges'}`

  function swap() {
    ;[oldTip, newTip] = [newTip, oldTip]
    ;[oldSince, newSince] = [newSince, oldSince]
    oldPick = choices.some((c) => c.value === oldTip) ? oldTip : OTHER
  }

  let ready = $derived(!!(oldTip.trim() && oldSince.trim() && newTip.trim() && newSince.trim()))

  async function compare() {
    if (!ready || busy) return
    busy = true
    error = ''
    try {
      const got = await CompareVersions(path, { base: oldSince.trim(), tip: oldTip.trim() }, { base: newSince.trim(), tip: newTip.trim() }, false)
      const picked = choices.find((c) => c.value === oldTip)
      onCompared(got, picked?.said ?? short(oldTip))
    } catch (e) {
      error = errorText(e)
    } finally {
      busy = false
    }
  }
</script>

<Modal title="Compare two versions" eyebrow="before and after" width={600} onClose={() => !busy && onClose()} scrimCloses={!busy}>
  <form
    onsubmit={(e) => {
      e.preventDefault()
      void compare()
    }}
  >
    <fieldset disabled={busy}>
      <legend>The old version</legend>
      <div class="grid">
        <label for="{uid}-old-pick">Up to</label>
        <div class="pick">
          {#if choices.length}
            <select id="{uid}-old-pick" bind:value={oldPick} onchange={onPick}>
              {#each choices as c (c.value)}<option value={c.value}>{c.label}</option>{/each}
              <option value={OTHER}>a branch, tag or commit…</option>
            </select>
          {/if}
          {#if oldPick === OTHER}
            <input id={choices.length ? undefined : `${uid}-old-pick`} aria-label="The old version's tip" placeholder="a branch, tag or commit" bind:value={oldTip} />
          {/if}
        </div>
        <label for="{uid}-old-since">Since</label>
        <input id="{uid}-old-since" bind:value={oldSince} placeholder="where it starts" />
        <span></span><span class="count">{said(oldCount)}</span>
      </div>
    </fieldset>

    <div class="swap"><button type="button" class="btn sm text" onclick={swap} title="Swap the two versions">⇅ Swap</button></div>

    <fieldset disabled={busy}>
      <legend>The new version</legend>
      <div class="grid">
        <label for="{uid}-new-tip">Up to</label>
        <input id="{uid}-new-tip" bind:value={newTip} placeholder="a branch, tag or commit" />
        <label for="{uid}-new-since">Since</label>
        <input id="{uid}-new-since" bind:value={newSince} placeholder="where it starts" />
        <span></span><span class="count">{said(newCount)}</span>
      </div>
    </fieldset>

    {#if ready}
      <code class="command">git range-diff {short(oldSince.trim())}..{short(oldTip.trim())} {short(newSince.trim())}..{short(newTip.trim())}</code>
    {/if}
    {#if baseName}<p class="hint">Each starts where it left {baseName}.</p>{/if}

    {#if error}<p class="form-error" role="alert">{error}</p>{/if}
    <div class="form-actions">
      {#if busy}<span class="foot"><Busy /> Comparing…</span>{/if}
      <InkButton variant="secondary" onclick={onClose} disabled={busy}>Cancel</InkButton>
      <InkButton type="submit" disabled={busy || !ready}>Compare</InkButton>
    </div>
  </form>
</Modal>

<style>
  fieldset {
    margin: 0 0 6px;
    padding: 0;
    border: 0;
    min-width: 0;
  }

  legend {
    padding: 0 0 8px;
    font-family: var(--font-mono);
    font-size: 12px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--ink-mute);
  }

  .grid {
    display: grid;
    grid-template-columns: 70px minmax(0, 1fr);
    gap: 8px 12px;
    align-items: center;
    font-size: 14px;
  }

  .grid label {
    color: var(--ink-soft);
  }

  .pick {
    display: flex;
    flex-direction: column;
    gap: 6px;
    min-width: 0;
  }

  .count,
  .hint,
  .foot {
    font-size: 12.5px;
    color: var(--ink-mute);
  }

  .hint {
    margin: 4px 0 0;
  }

  .swap {
    display: flex;
    justify-content: flex-end;
    margin: -4px 0 2px;
  }

  .command {
    display: block;
    margin: 10px 0 0;
    padding: 8px 12px;
    border-radius: var(--r-sm);
    background: var(--paper-warm);
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--accent-b);
    overflow-wrap: anywhere;
  }

  .foot {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    margin-right: auto;
  }
</style>
