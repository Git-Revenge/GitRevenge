<script lang="ts">
  import Icon from '../ui/Icon.svelte'
  import InkButton from '../ui/InkButton.svelte'
  import { syntaxHighlighting } from '@codemirror/language'
  import { EditorState } from '@codemirror/state'
  import { EditorView, lineNumbers } from '@codemirror/view'
  import { classHighlighter } from '@lezer/highlight'
  import { ConflictFile, RepoState, ResolveConflict, TakeConflictSide } from '../../../wailsjs/go/app/App.js'
  import type { git } from '../../../wailsjs/go/models'
  import { session } from '../session.svelte'
  import { languageFor } from './highlight'
  import { hasMarkers, parseConflicts, resolution, type Choice, type ConflictRegion } from './markers'
  import Loading from '../ui/Loading.svelte'

  let { path, file, onClose }: { path: string; file: string; onClose: () => void } = $props()

  let cf = $state<git.ConflictFile | null>(null)
  let op = $state('')
  let regions = $state<ConflictRegion[]>([])
  let result = $state('')
  let busy = $state(false)

  let oursHost = $state<HTMLDivElement>()
  let theirsHost = $state<HTMLDivElement>()
  let resultHost = $state<HTMLDivElement>()
  let resultView: EditorView | undefined

  // In a rebase git swaps the meaning: "ours" is the branch being rebased onto.
  let oursLabel = $derived(op === 'rebase' ? 'Ours (upstream being rebased onto)' : 'Ours (current branch)')
  let theirsLabel = $derived(op === 'rebase' ? 'Theirs (your commit being replayed)' : 'Theirs (incoming)')

  async function load() {
    const [c, st] = await Promise.all([session.run(() => ConflictFile(path, file)), session.run(() => RepoState(path))])
    if (!c) return
    cf = c
    op = st?.operation ?? ''
    result = c.merged.text
    regions = parseConflicts(result)
  }

  $effect(() => {
    void file
    void load()
  })

  function readOnly(text: string, lang: ReturnType<typeof languageFor> extends Promise<infer L> ? L : never) {
    return EditorState.create({
      doc: text,
      extensions: [
        lineNumbers(),
        EditorView.editable.of(false),
        EditorState.readOnly.of(true),
        syntaxHighlighting(classHighlighter),
        ...(lang ? [lang] : []),
      ],
    })
  }

  // Build the three editors once the file is loaded.
  $effect(() => {
    if (!cf || cf.binary || !oursHost || !theirsHost || !resultHost) return
    const c = cf
    let views: EditorView[] = []
    let cancelled = false
    void languageFor(file).then((lang) => {
      if (cancelled) return
      const ours = new EditorView({ state: readOnly(c.ours.text, lang), parent: oursHost! })
      const theirs = new EditorView({ state: readOnly(c.theirs.text, lang), parent: theirsHost! })
      resultView = new EditorView({
        state: EditorState.create({
          doc: result,
          extensions: [
            lineNumbers(),
            syntaxHighlighting(classHighlighter),
            ...(lang ? [lang] : []),
            // Keep the conflict list in step with manual edits.
            EditorView.updateListener.of((u) => {
              if (u.docChanged) {
                result = u.state.doc.toString()
                regions = parseConflicts(result)
              }
            }),
          ],
        }),
        parent: resultHost!,
      })
      views = [ours, theirs, resultView]
    })
    return () => {
      cancelled = true
      views.forEach((v) => v.destroy())
      resultView = undefined
    }
  })

  function choose(region: ConflictRegion, choice: Choice) {
    if (!resultView) return
    resultView.dispatch({ changes: { from: region.from, to: region.to, insert: resolution(region, choice) } })
  }

  function jump(region: ConflictRegion) {
    resultView?.dispatch({ selection: { anchor: region.from }, scrollIntoView: true })
    resultView?.focus()
  }

  async function markResolved() {
    busy = true
    const ok = await session.run(async () => {
      await ResolveConflict(path, file, result)
      return true
    })
    busy = false
    if (ok) onClose()
  }

  async function take(side: 'ours' | 'theirs') {
    busy = true
    const ok = await session.run(async () => {
      await TakeConflictSide(path, file, side)
      return true
    })
    busy = false
    if (ok) onClose()
  }
</script>

<div class="conflict">
  <header class="view-head">
    <button class="btn quiet" onclick={onClose} title="Back"><Icon name="back" />Back</button>
    <span class="view-badge danger">Conflict</span>
    <span class="view-file" title={file}>{file}</span>
    <span class="spacer"></span>
    <button class="btn quiet" disabled={busy} onclick={() => take('ours')} title="Resolve with the whole ours version">Use ours</button>
    <button class="btn quiet" disabled={busy} onclick={() => take('theirs')} title="Resolve with the whole theirs version">Use theirs</button>
    {#if cf && !cf.binary}
      <InkButton
        disabled={busy || hasMarkers(result)}
        onclick={markResolved}
        title={hasMarkers(result) ? 'Resolve every conflict first' : 'Save the result and mark the file resolved'}>Mark resolved</InkButton
      >
    {/if}
  </header>

  {#if !cf}
    <Loading />
  {:else if cf.binary}
    <p class="muted">This file is binary, so it can't be merged line by line. Choose a whole version above.</p>
  {:else}
    <div class="sides">
      <section>
        <h4 class="meta">{oursLabel}{cf.ours.exists ? '' : ' (deleted)'}</h4>
        <div class="editor code-ground" bind:this={oursHost}></div>
      </section>
      <section>
        <h4 class="meta">{theirsLabel}{cf.theirs.exists ? '' : ' (deleted)'}</h4>
        <div class="editor code-ground" bind:this={theirsHost}></div>
      </section>
    </div>
    <div class="strip">
      {#if regions.length === 0}
        <span class="done">No conflicts left. Review the result, then Mark resolved.</span>
      {:else}
        <span>{regions.length} {regions.length === 1 ? 'conflict' : 'conflicts'}:</span>
        {#each regions as r, i (r.from)}
          <span class="region">
            <button class="btn sm text link" onclick={() => jump(r)} title="Go to conflict {i + 1}">#{i + 1}</button>
            <button class="btn sm quiet" onclick={() => choose(r, 'ours')}>Ours</button>
            <button class="btn sm quiet" onclick={() => choose(r, 'theirs')}>Theirs</button>
            <button class="btn sm quiet" onclick={() => choose(r, 'both')}>Both</button>
            {#if r.base !== null}<button class="btn sm quiet" onclick={() => choose(r, 'base')}>Base</button>{/if}
          </span>
        {/each}
      {/if}
    </div>
    <section class="result">
      <h4 class="meta">Result · editable</h4>
      <div class="editor code-ground" bind:this={resultHost}></div>
    </section>
  {/if}
</div>

<style>
  .conflict {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
  }

  .spacer {
    flex: 1;
  }

  .sides {
    display: flex;
    flex: 1;
    min-height: 0;
    border-bottom: 1px solid var(--paper-edge);
  }

  .sides section {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
  }

  .sides section + section {
    border-left: 1px solid var(--paper-edge);
  }

  h4 {
    margin: 0;
    padding: 8px 14px 6px;
    font-weight: normal;
  }

  .editor {
    flex: 1;
    min-height: 0;
    overflow: auto;
    font-size: 13px;
    tab-size: 4;
  }

  .editor :global(.cm-editor) {
    height: 100%;
    background: var(--paper);
    color: var(--ink);
  }

  .editor :global(.cm-gutters) {
    background: var(--paper-warm);
    color: var(--ink-faint);
    border-right: 1px solid var(--paper-edge);
  }

  .editor :global(.cm-content) {
    font-family: var(--font-mono);
    caret-color: var(--link);
  }

  .editor :global(.cm-selectionBackground) {
    background: var(--selection) !important;
  }

  .strip {
    display: flex;
    flex-wrap: wrap;
    gap: 8px 14px;
    align-items: center;
    padding: 8px 14px;
    border-bottom: 1px solid var(--paper-edge);
    font-size: 13px;
  }

  .region {
    display: inline-flex;
    gap: 2px;
    align-items: center;
    padding: 2px;
    border-radius: var(--r-md);
    background: var(--wash-hover);
  }

  .region .link {
    font-family: var(--font-mono);
  }

  .done {
    color: var(--success);
    font-style: italic;
  }

  .result {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
  }

  .muted {
    color: var(--ink-mute);
    padding: 10px 14px;
    font-style: italic;
  }
</style>
