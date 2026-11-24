<script lang="ts">
  import Modal from '../ui/Modal.svelte'
  import { moveResult, type Move } from './blocks'
  import { splitLines } from './lineDiff'
  import UnifiedDiff from './UnifiedDiff.svelte'

  /** A move that changed on the way, against where it came from (#947): its edits show as
   * edits, which the diff of the whole file shows only as moved lines. */
  let {
    path,
    move,
    oldText,
    newText,
    language,
    onClose,
  }: {
    path: string
    move: Move
    oldText: string
    newText: string
    language: string
    onClose: () => void
  } = $props()

  const slice = (text: string, from: number, to: number) => splitLines(text).slice(from, to).join('')
  let before = $derived(slice(oldText, move.oldStart, move.oldEnd))
  let after = $derived(slice(newText, move.newStart, move.newEnd))
  let edits = $derived(move.edits?.length ?? 0)
</script>

<Modal title="Compare with the original" eyebrow="moved, and changed on the way" width={880} {onClose}>
  <p class="where">
    Lines {move.oldStart + 1}–{move.oldEnd} before are lines {move.newStart + 1}–{move.newEnd} now, changed in {edits === 1 ? 'one place' : `${edits} places`}.
    Line numbers here count from the start of the move.
  </p>
  <div class="diff">
    <UnifiedDiff {path} oldText={before} newText={after} diff={moveResult(move, language)} />
  </div>
</Modal>

<style>
  .where {
    margin: 0 0 12px;
    color: var(--ink-soft);
    font-size: 14px;
    line-height: 1.5;
  }

  .diff {
    max-height: 60vh;
    overflow: auto;
    border: 1px solid var(--paper-edge);
    border-radius: var(--r-md);
  }
</style>
