<script lang="ts">
  // The blame view's track through a file's versions (#1078): a mark for each version,
  // coloured by its author and taller the more it changed, with the years under it. Dragging
  // the handle, the arrow keys on it, Home and End, and the buttons either side move between
  // versions; [ and ] do it from anywhere in the view.
  import Icon from '../ui/Icon.svelte'
  import { formatTime, laneVar } from '../graph/draw'
  import { along, authorInks, churnText, markHeight, READ_WAYS, stepLabel, yearMarks, type ReadWay, type Step } from './track'

  let {
    steps,
    at,
    onAt,
    renamedFrom = '',
    onCarryOn,
    onReveal,
    way = 'who',
    onWay,
    onRestore,
  }: {
    steps: Step[]
    /** The step shown. */
    at: number
    onAt: (i: number) => void
    /** What the oldest version renamed the file from: the track starts there. */
    renamedFrom?: string
    /** Opens the file's blame under its old name, from before the rename. */
    onCarryOn?: () => void
    /** Shows a commit in the graph. */
    onReveal?: (commit: string) => void
    /** How the version is read (#1080), and how to read it another way. */
    way?: ReadWay
    onWay?: (way: ReadWay) => void
    /** Restores the version shown into the working tree (#1082): offered for a commit. */
    onRestore?: () => void
  } = $props()

  let inks = $derived(authorInks(steps))
  let years = $derived(yearMarks(steps))
  let step = $derived(steps[at])
  let last = $derived(steps.length - 1)
  let label = $derived(stepLabel(at, steps))
  /** What a screen reader says the handle is at. */
  let valueText = $derived(step ? (step.id ? `${label}: ${step.subject}, ${step.author}, ${formatTime(step.time)}` : label) : '')
  /** [ and ] on the handle too: the view's own keys skip a focused input, as typing. */
  function stepKey(e: KeyboardEvent) {
    if (e.ctrlKey || e.metaKey || e.altKey || (e.key !== '[' && e.key !== ']')) return
    e.preventDefault()
    onAt(at + (e.key === '[' ? -1 : 1))
  }
  const markColor = (s: Step) => (s.id ? laneVar(inks.get(s.author) ?? 0) : 'var(--ink-mute)')
  const left = (i: number) => `calc(9px + (100% - 18px) * ${along(i, steps.length)})`
</script>

<section class="track" aria-label="The file through time">
  {#if step}
    <div class="stop">
      <span class="meta">{label}</span>
      <span class="subject">{step.subject}</span>
      {#if step.id}
        <span class="by">
          <button class="sha" onclick={() => onReveal?.(step.id)} title="Show in graph">{step.id.slice(0, 7)}</button>
          · {step.author} · {formatTime(step.time)} · <span class="mono">{churnText(step, at === 0)}</span>
        </span>
        {#if onRestore && !step.binary}<button class="btn sm quiet restore" onclick={onRestore} title="Write this version into the working tree, as a change to commit or throw away"><Icon name="history" size={13} />Restore this version</button>{/if}
      {/if}
      {#if onWay}
        <div class="ways" role="group" aria-label="How to read this version">
          {#each READ_WAYS as w (w.id)}<button aria-pressed={way === w.id} onclick={() => onWay(w.id)}>{w.label}</button>{/each}
        </div>
      {/if}
    </div>
  {/if}
  <div class="rail">
    <button class="btn sm quiet icon" disabled={at <= 0} onclick={() => onAt(at - 1)} aria-label="The version before" title="The version before ([)"><Icon name="back" size={14} /></button>
    <div class="marks">
      {#if renamedFrom}
        <span class="renamed" title="Before this, the file was {renamedFrom}" aria-hidden="true"></span>
      {/if}
      <span class="line" aria-hidden="true"></span>
      {#each steps as s, i (s.id || 'worktree')}
        <span class="mark" class:here={i === at} class:tree={!s.id} style:left={left(i)} style:height="{markHeight(s)}px" style:background={markColor(s)} aria-hidden="true"></span>
      {/each}
      <input type="range" min="0" max={last} step="1" value={at} oninput={(e) => onAt(Number(e.currentTarget.value))} onkeydown={stepKey} aria-label="Version of the file" aria-valuetext={valueText} />
    </div>
    <button class="btn sm quiet icon next" disabled={at >= last} onclick={() => onAt(at + 1)} aria-label="The version after" title="The version after (])"><Icon name="back" size={14} /></button>
  </div>
  <div class="years" aria-hidden="true">
    {#each years as y (y.label)}<span style:left="{y.at * 100}%">{y.label}</span>{/each}
  </div>
  {#if renamedFrom && at === 0}
    <p class="note" role="note">
      Before this it was <code>{renamedFrom}</code>.
      {#if onCarryOn}<button class="btn sm text" onclick={onCarryOn}>Carry on under the old name</button>{/if}
    </p>
  {/if}
</section>

<style>
  .track {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 10px 16px 6px;
    border-bottom: 1px dashed var(--paper-edge);
    background: var(--paper-warm);
  }

  .stop {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 2px 10px;
    min-width: 0;
  }

  .ways {
    display: inline-flex;
    flex: none;
    margin-left: auto;
    border: 1px solid var(--paper-edge);
    border-radius: var(--r-md);
    overflow: hidden;
  }

  .ways button {
    padding: 3px 10px;
    border: 0;
    background: none;
    color: var(--ink-mute);
    font-size: 13px;
    cursor: pointer;
  }

  .ways button + button {
    border-left: 1px solid var(--paper-edge);
  }

  .ways button[aria-pressed='true'] {
    background: var(--selected);
    color: var(--ink);
  }

  .subject {
    font-size: 15px;
    color: var(--ink);
  }

  .by {
    font-size: 13px;
    color: var(--ink-soft);
  }

  .sha {
    padding: 0;
    border: 0;
    background: none;
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--accent-b);
    cursor: pointer;
  }

  .sha:hover {
    color: var(--link);
  }

  .mono {
    font-family: var(--font-mono);
    font-size: 12px;
  }

  .rail {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .next :global(svg) {
    transform: rotate(180deg);
  }

  .marks {
    position: relative;
    flex: 1;
    min-width: 0;
    height: 36px;
  }

  .line {
    position: absolute;
    left: 9px;
    right: 9px;
    bottom: 6px;
    height: 1px;
    background: var(--paper-edge);
  }

  .mark {
    position: absolute;
    bottom: 6px;
    width: 3px;
    margin-left: -1.5px;
    border-radius: 2px;
    opacity: 0.7;
    pointer-events: none;
  }

  .mark.here {
    width: 4px;
    margin-left: -2px;
    opacity: 1;
  }

  /* The work tree isn't a commit: a hollow mark. */
  .mark.tree {
    background: transparent !important;
    border: 1px dashed var(--ink-mute);
    box-sizing: border-box;
  }

  /* Where the file got its name: the track's start is a wall, not the beginning. */
  .renamed {
    position: absolute;
    left: 0;
    top: 4px;
    bottom: 2px;
    width: 2px;
    background: repeating-linear-gradient(to bottom, var(--ink-mute) 0 3px, transparent 3px 6px);
  }

  input[type='range'] {
    -webkit-appearance: none;
    appearance: none;
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    margin: 0;
    background: transparent;
    cursor: grab;
  }

  input[type='range']:active {
    cursor: grabbing;
  }

  input[type='range']::-webkit-slider-runnable-track {
    height: 100%;
    background: transparent;
  }

  input[type='range']::-webkit-slider-thumb {
    -webkit-appearance: none;
    appearance: none;
    width: 18px;
    height: 32px;
    margin-top: 2px;
    border-radius: 9px;
    background: var(--accent);
    border: 3px solid var(--paper-warm);
  }

  input[type='range']::-moz-range-thumb {
    width: 12px;
    height: 26px;
    border-radius: 9px;
    background: var(--accent);
    border: 3px solid var(--paper-warm);
  }

  input[type='range']:focus-visible {
    outline: 2px solid var(--link);
    outline-offset: 3px;
    border-radius: var(--r-sm);
  }

  .years {
    position: relative;
    height: 16px;
    margin: -6px 41px 0;
  }

  .years span {
    position: absolute;
    font-family: var(--font-mono);
    font-size: 12px;
    letter-spacing: 0.06em;
    color: var(--ink-mute);
  }

  .note {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 4px 8px;
    margin: 0;
    font-size: 13px;
    color: var(--ink-soft);
  }

  .note code {
    font-family: var(--font-mono);
    font-size: 12px;
  }
</style>
