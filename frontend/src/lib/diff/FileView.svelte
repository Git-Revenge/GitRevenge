<script lang="ts">
  // The whole file as of the selected commit (or the working tree), read-only, with syntax
  // highlighting, line numbers and the shared word wrap setting.
  import { EditorState } from '@codemirror/state'
  import { EditorView, lineNumbers } from '@codemirror/view'
  import { syntaxHighlighting } from '@codemirror/language'
  import { classHighlighter } from '@lezer/highlight'
  import { languageFor } from './highlight'
  import { wrap } from './wrap.svelte'

  let { path, text }: { path: string; text: string } = $props()

  let host: HTMLDivElement

  // Rebuild the view whenever the file, its text or word wrap changes.
  $effect(() => {
    const [p, doc, wrapped] = [path, text, wrap.on]
    let view: EditorView | undefined
    let cancelled = false
    void languageFor(p).then((lang) => {
      if (cancelled) return
      view = new EditorView({
        parent: host,
        state: EditorState.create({
          doc,
          extensions: [
            EditorView.editable.of(false),
            EditorState.readOnly.of(true),
            lineNumbers(),
            syntaxHighlighting(classHighlighter),
            EditorView.theme({}, { dark: true }),
            ...(wrapped ? [EditorView.lineWrapping] : []),
            ...(lang ? [lang] : []),
          ],
        }),
      })
    })
    return () => {
      cancelled = true
      view?.destroy()
    }
  })
</script>

<div class="file" bind:this={host}></div>

<style>
  .file {
    font-size: 13px;
    tab-size: 4;
  }

  .file :global(.cm-editor) {
    background: var(--paper);
    color: var(--ink);
  }

  .file :global(.cm-gutters) {
    background: var(--paper-warm);
    color: var(--ink-faint);
    border-right: 1px solid var(--paper-edge);
  }

  .file :global(.cm-content) {
    font-family: var(--font-mono);
  }

  .file :global(.cm-selectionBackground) {
    background: var(--selection) !important;
  }
</style>
