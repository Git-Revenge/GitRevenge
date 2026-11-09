<script lang="ts">
  // The Raw text view of the Git Config tab (#226): the working copy as text, on the dark code
  // ground. Edits reach the backend shortly after typing stops; git's verdict on the text
  // comes back as the file's problem, marked on its line.
  import { defaultKeymap, history, historyKeymap, indentWithTab } from '@codemirror/commands'
  import { syntaxHighlighting } from '@codemirror/language'
  import { EditorState, StateEffect, StateField } from '@codemirror/state'
  import { Decoration, EditorView, keymap, lineNumbers, type DecorationSet } from '@codemirror/view'
  import { classHighlighter } from '@lezer/highlight'
  import { onDestroy } from 'svelte'
  import { SetGitConfigText } from '../../../wailsjs/go/app/App.js'
  import { gitConfig } from './config.svelte'
  import { gitConfigSyntax } from './syntax'

  /** How long typing must pause before the text is sent. */
  const SEND_DELAY = 300

  let host = $state<HTMLDivElement>()
  /** The file this editor was built for; the selection may move on before a last send. */
  let path = ''
  let view = $state<EditorView>()
  /** The text last sent to or received from the backend, so an echo isn't applied twice. */
  let synced = ''
  let timer: ReturnType<typeof setTimeout> | undefined
  /** Where the note about the bad line sits, in px from the top of the editor. */
  let noteTop = $state(0)

  let doc = $derived(gitConfig.doc)
  let problem = $derived(doc?.problem)

  /** Marks the line git can't read; 0 clears it. */
  const markLine = StateEffect.define<number>()
  const badLine = StateField.define<DecorationSet>({
    create: () => Decoration.none,
    update(marks, tr) {
      marks = marks.map(tr.changes)
      for (const e of tr.effects) {
        if (!e.is(markLine)) continue
        const n = e.value
        marks = n > 0 && n <= tr.state.doc.lines ? Decoration.set([Decoration.line({ class: 'cm-bad-line' }).range(tr.state.doc.line(n).from)]) : Decoration.none
      }
      return marks
    },
    provide: (f) => EditorView.decorations.from(f),
  })

  async function send() {
    timer = undefined
    if (!view || !path) return
    const text = view.state.doc.toString()
    if (text === synced) return
    synced = text
    const file = path
    await gitConfig.call(() => SetGitConfigText(file, text)).then((d) => d && (gitConfig.docs[file] = d))
  }

  /** Sends what's typed now rather than after the pause, before a save. */
  function flush(): Promise<void> {
    if (!timer) return Promise.resolve()
    clearTimeout(timer)
    return send()
  }

  // Build the editor once, for the file it opens with; the view remounts for another file.
  $effect(() => {
    if (!host || view || !doc) return
    const text = doc.text
    path = doc.path
    synced = text
    view = new EditorView({
        parent: host,
        state: EditorState.create({
          doc: text,
          extensions: [
            lineNumbers(),
            history(),
            keymap.of([...defaultKeymap, ...historyKeymap, indentWithTab]),
            syntaxHighlighting(classHighlighter),
            gitConfigSyntax(),
            badLine,
            EditorView.contentAttributes.of({ 'aria-label': 'Config file text', spellcheck: 'false' }),
            EditorView.updateListener.of((u) => {
              if (!u.docChanged) return
              clearTimeout(timer)
              timer = setTimeout(() => void send(), SEND_DELAY)
            }),
          ],
        }),
      })
  })

  // Take in text that changed elsewhere: a reload, a restore, an edit in Settings.
  $effect(() => {
    const text = doc?.path === path ? doc.text : undefined
    if (!view || text === undefined || text === synced || timer) return
    synced = text
    if (text !== view.state.doc.toString()) view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: text } })
  })

  $effect(() => {
    const line = problem?.line ?? 0
    if (!view) return
    view.dispatch({ effects: markLine.of(line) })
    if (line > 0 && line <= view.state.doc.lines) noteTop = view.lineBlockAt(view.state.doc.line(line).from).top
  })

  function cancel() {
    clearTimeout(timer)
    timer = undefined
  }

  $effect(() => {
    gitConfig.flush = flush
    gitConfig.cancel = cancel
    return () => {
      if (gitConfig.flush === flush) gitConfig.flush = undefined
      if (gitConfig.cancel === cancel) gitConfig.cancel = undefined
    }
  })

  onDestroy(() => {
    void flush()
    view?.destroy()
  })
</script>

<div class="raw">
  <p class="notice">
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" aria-hidden="true"
      ><path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12z" /><circle cx="12" cy="12" r="3" /></svg
    >
    Nothing is hidden here, including tokens and passwords. Settings masks them.
  </p>
  <div class="body">
    <div class="editor code-ground" bind:this={host}></div>
    {#if problem}
      <aside class="why" style:padding-top="{problem.line ? noteTop : 0}px" aria-live="polite">
        <span class="hand">{problem.line ? `git can’t read line ${problem.line}` : 'git can’t read this'}</span>
        <span class="detail">{problem.line ? 'Fix the line so git can parse the file again.' : problem.message} Until then, Settings can’t show it and it can’t be saved.</span>
      </aside>
    {/if}
  </div>
</div>

<style>
  .raw {
    display: flex;
    flex-direction: column;
    gap: 14px;
  }

  .notice {
    display: flex;
    align-items: center;
    gap: 10px;
    margin: 0;
    font-size: 14px;
    color: var(--ink-mute);
  }

  .body {
    display: flex;
    flex-wrap: wrap;
    gap: 24px;
    align-items: flex-start;
  }

  .editor {
    position: relative;
    flex: 999 1 520px;
    min-width: 0;
    padding: 14px 0;
    border-radius: var(--r-sm);
    background: var(--code-bg);
  }

  /* The brand's small rose smear on a code block's top edge. */
  .editor::before {
    content: '';
    position: absolute;
    left: 28px;
    top: -5px;
    z-index: 1;
    width: 64px;
    height: 10px;
    background: var(--accent);
    opacity: 0.8;
    filter: url(#wc-edge);
    border-radius: 45% 55% 50% 50% / 55% 45%;
  }

  .editor :global(.cm-editor) {
    background: none;
    color: var(--code-text);
    font-size: 14px;
  }

  .editor :global(.cm-editor.cm-focused) {
    outline: none;
  }

  .editor :global(.cm-scroller) {
    font-family: var(--font-mono);
    line-height: 1.75;
  }

  .editor :global(.cm-gutters) {
    background: none;
    border: none;
    color: var(--code-comment);
  }

  .editor :global(.cm-lineNumbers .cm-gutterElement) {
    padding: 0 16px 0 14px;
  }

  .editor :global(.cm-cursor) {
    border-left-color: var(--code-text);
  }

  .editor :global(.cm-selectionBackground),
  .editor :global(.cm-focused .cm-selectionBackground) {
    background: var(--selection);
  }

  .editor :global(.cm-bad-line) {
    background: color-mix(in srgb, var(--syn-invalid) 18%, transparent);
  }

  .editor:focus-within {
    outline: 2px solid var(--link);
    outline-offset: 3px;
  }

  .why {
    flex: 1 1 220px;
    max-width: 260px;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .why .hand {
    font-size: 26px;
    line-height: 1.05;
    color: var(--danger);
    transform: rotate(-2.5deg);
    transform-origin: left;
  }

  .detail {
    font-size: 14px;
    color: var(--ink-soft);
  }
</style>
