<script lang="ts">
  import { MergeView } from '@codemirror/merge'
  import { EditorState } from '@codemirror/state'
  import { EditorView, lineNumbers } from '@codemirror/view'
  import { syntaxHighlighting } from '@codemirror/language'
  import { classHighlighter } from '@lezer/highlight'
  import { languageFor } from './highlight'
  import { wrap } from './wrap.svelte'

  let { path, oldText, newText }: { path: string; oldText: string; newText: string } = $props()

  let host: HTMLDivElement

  // Rebuild the read-only merge view whenever the inputs or word wrap change. MergeView pads
  // the shorter side of each chunk, so wrapped rows stay level across the panes.
  $effect(() => {
    const [p, a, b, wrapped] = [path, oldText, newText, wrap.on]
    let view: MergeView | undefined
    let cancelled = false
    void languageFor(p).then((lang) => {
      if (cancelled) return
      const ext = [
        EditorView.editable.of(false),
        EditorState.readOnly.of(true),
        lineNumbers(),
        syntaxHighlighting(classHighlighter),
        EditorView.theme({}, { dark: true }),
        ...(wrapped ? [EditorView.lineWrapping] : []),
        ...(lang ? [lang] : []),
      ]
      view = new MergeView({
        a: { doc: a, extensions: ext },
        b: { doc: b, extensions: ext },
        parent: host,
        collapseUnchanged: { margin: 3, minSize: 6 },
        gutter: true,
      })
    })
    return () => {
      cancelled = true
      view?.destroy()
    }
  })
</script>

<div class="split" bind:this={host}></div>

<style>
  .split {
    font-size: 13px;
    tab-size: 4;
  }

  .split :global(.cm-mergeView) {
    height: 100%;
  }

  .split :global(.cm-editor) {
    background: var(--paper);
    color: var(--ink);
  }

  .split :global(.cm-gutters) {
    background: var(--paper-warm);
    color: var(--ink-mute);
    border-right: 1px solid var(--paper-edge);
  }

  .split :global(.cm-content) {
    font-family: var(--font-mono);
  }

  .split :global(.cm-gutters) {
    color: var(--ink-faint);
  }

  .split :global(.cm-merge-a .cm-changedLine),
  .split :global(.cm-deletedChunk) {
    background: var(--danger-wash);
  }

  .split :global(.cm-merge-b .cm-changedLine) {
    background: var(--success-wash);
  }

  .split :global(.cm-merge-a .cm-changedText) {
    background: color-mix(in srgb, var(--danger) 35%, transparent);
  }

  .split :global(.cm-merge-b .cm-changedText) {
    background: color-mix(in srgb, var(--success) 35%, transparent);
  }

  .split :global(.cm-collapsedLines) {
    background: var(--paper-warm);
    color: var(--ink-mute);
    font-family: var(--font-serif);
    font-style: italic;
  }

  .split :global(.cm-selectionBackground) {
    background: var(--selection) !important;
  }
</style>
