<script lang="ts">
  // The whole file as of the selected commit (or the working tree), read-only, with syntax
  // highlighting, line numbers and the shared word wrap setting. Find in files (#980) opens a
  // file here with its matches marked: the current one stronger, and in view.
  import { EditorState, RangeSetBuilder, StateEffect, StateField } from '@codemirror/state'
  import { Decoration, EditorView, lineNumbers, type DecorationSet } from '@codemirror/view'
  import { syntaxHighlighting } from '@codemirror/language'
  import { classHighlighter } from '@lezer/highlight'
  import { languageFor } from './highlight'
  import { wrap } from './wrap.svelte'

  let {
    path,
    text,
    marks = [],
    active = -1,
    line = 0,
  }: {
    path: string
    text: string
    /** Stretches to mark, [from, to) in UTF-16 units, in order. */
    marks?: { from: number; to: number }[]
    /** Which mark is the current one, kept in view; -1 for none. */
    active?: number
    /** A line (1-based) to bring into view and wash, when there's no mark to show. */
    line?: number
  } = $props()

  let host: HTMLDivElement
  let view = $state<EditorView | undefined>()

  const setMarks = StateEffect.define<{ marks: { from: number; to: number }[]; active: number; line: number }>()
  const mark = Decoration.mark({ class: 'found' })
  const current = Decoration.mark({ class: 'found current' })
  const lineWash = Decoration.line({ class: 'found-line' })
  const marksField = StateField.define<DecorationSet>({
    create: () => Decoration.none,
    update(deco, tr) {
      for (const e of tr.effects) {
        if (!e.is(setMarks)) continue
        const doc = tr.state.doc
        const b = new RangeSetBuilder<Decoration>()
        const { marks, active, line } = e.value
        if (!marks.length && line > 0 && line <= doc.lines) {
          const at = doc.line(line).from
          b.add(at, at, lineWash)
        }
        marks.forEach((m, i) => {
          if (m.from < m.to && m.to <= doc.length) b.add(m.from, m.to, i === active ? current : mark)
        })
        return b.finish()
      }
      return deco.map(tr.changes)
    },
    provide: (f) => EditorView.decorations.from(f),
  })

  // Rebuild the view whenever the file, its text or word wrap changes.
  $effect(() => {
    const [p, doc, wrapped] = [path, text, wrap.on]
    let made: EditorView | undefined
    let cancelled = false
    void languageFor(p).then((lang) => {
      if (cancelled) return
      made = new EditorView({
        parent: host,
        state: EditorState.create({
          doc,
          extensions: [
            EditorView.editable.of(false),
            EditorState.readOnly.of(true),
            lineNumbers(),
            syntaxHighlighting(classHighlighter),
            EditorView.theme({}, { dark: true }),
            marksField,
            ...(wrapped ? [EditorView.lineWrapping] : []),
            ...(lang ? [lang] : []),
          ],
        }),
      })
      view = made
    })
    return () => {
      cancelled = true
      made?.destroy()
      view = undefined
    }
  })

  // The marks, and the current one in view, without building the editor again.
  $effect(() => {
    const v = view
    if (!v) return
    const doc = v.state.doc
    const m = marks[active]
    const at = m && m.to <= doc.length ? m.from : line > 0 && line <= doc.lines ? doc.line(line).from : -1
    v.dispatch({
      effects: [setMarks.of({ marks, active, line }), ...(at >= 0 ? [EditorView.scrollIntoView(at, { y: 'center' })] : [])],
    })
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

  /* Find's matches: the highlighter wash, and the current one outlined as well. */
  .file :global(.found) {
    background: var(--highlight);
    border-radius: 2px;
  }

  .file :global(.found.current) {
    outline: 2px solid var(--link);
    outline-offset: 1px;
  }

  .file :global(.found-line) {
    background: var(--wash-hover);
  }
</style>
