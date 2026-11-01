<script lang="ts" module>
  /** Open panels, newest last: the one to answer a key when focus is on none of them. */
  const open: HTMLElement[] = []
</script>

<script lang="ts">
  import { onMount, type Snippet } from 'svelte'
  import { inkIn, inkOut, scrim } from './motion'

  /** A modal dialog: scrim, paper panel, title. Keeps focus inside while open, starts it on
   * the first field (or button), and gives it back to whatever had it on close. */
  let {
    title,
    eyebrow,
    width = 460,
    layer = 200,
    dismissable = true,
    scrimCloses = true,
    onClose,
    children,
  }: {
    title: string
    /** A handwritten note above the title. */
    eyebrow?: string
    width?: number
    /** z-index: credential prompts sit above other dialogs. */
    layer?: number
    /** Whether Escape (and a click on the scrim) close it. */
    dismissable?: boolean
    /** Set false to keep a click on the scrim from closing it, e.g. while work is running. */
    scrimCloses?: boolean
    onClose: () => void
    children: Snippet
  } = $props()

  const id = $props.id()
  let panel: HTMLDivElement

  const FOCUSABLE = 'input:not([disabled]), textarea:not([disabled]), select:not([disabled]), button:not([disabled]), [href], [tabindex]:not([tabindex="-1"])'

  onMount(() => {
    const before = document.activeElement as HTMLElement | null
    const first = panel.querySelector<HTMLElement>('input:not([disabled]), textarea:not([disabled]), select:not([disabled])') ?? panel.querySelector<HTMLElement>(FOCUSABLE)
    ;(first ?? panel).focus()
    open.push(panel)
    return () => {
      open.splice(open.indexOf(panel), 1)
      before?.focus?.()
    }
  })

  /** Disabling the focused button moves focus to the body, where the panel can't hear keys. */
  function onLostKey(e: KeyboardEvent) {
    if (e.target !== document.body || open.at(-1) !== panel || (e.key !== 'Escape' && e.key !== 'Tab')) return
    panel.focus()
    onKey(e)
  }

  function onKey(e: KeyboardEvent) {
    if (e.key === 'Escape' && dismissable) {
      e.stopPropagation()
      onClose()
    } else if (e.key === 'Tab') {
      // Keep Tab inside the dialog.
      const items = [...panel.querySelectorAll<HTMLElement>(FOCUSABLE)].filter((el) => el.offsetParent !== null)
      if (items.length === 0) return
      const first = items[0]
      const last = items[items.length - 1]
      if (e.shiftKey && (document.activeElement === first || document.activeElement === panel)) {
        e.preventDefault()
        last.focus()
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault()
        first.focus()
      }
    }
  }
</script>

<svelte:window onkeydowncapture={onLostKey} />

<div
  class="scrim"
  role="presentation"
  in:scrim|global
  out:scrim|global={{ out: true }}
  style:z-index={layer} onmousedown={(e) => dismissable && scrimCloses && e.target === e.currentTarget && onClose()}>
  <div
    class="panel"
    in:inkIn|global
    out:inkOut|global
    role="dialog"
    aria-modal="true"
    aria-labelledby="{id}-title"
    tabindex="-1"
    style:width="min({width}px, calc(100vw - 32px))"
    bind:this={panel}
    onkeydown={onKey}
  >
    <span class="splash" aria-hidden="true"></span>
    <header>
      {#if eyebrow}<span class="hand">{eyebrow}</span>{/if}
      <h2 id="{id}-title">{title}</h2>
    </header>
    {@render children()}
  </div>
</div>

<style>
  .scrim {
    position: fixed;
    inset: 0;
    display: grid;
    place-items: center;
    padding: 16px;
    background: var(--scrim);
  }

  .panel {
    position: relative;
    max-height: calc(100vh - 32px);
    overflow: auto;
    display: flex;
    flex-direction: column;
    gap: 14px;
    padding: 22px 24px 20px;
    background: var(--paper);
    border: 1px solid var(--paper-edge);
    border-radius: 10px;
    box-shadow: var(--shadow);
    outline: none;
  }

  /* A torn rose stroke along the top edge, like the active tab's. */
  .splash {
    position: absolute;
    top: 0;
    left: 24px;
    width: 88px;
    height: 4px;
    background: var(--link);
    opacity: 0.8;
    filter: url(#wc-edge);
    border-radius: 45% 55% 50% 50% / 55% 45%;
    pointer-events: none;
  }

  header {
    display: flex;
    flex-direction: column;
  }

  .hand {
    align-self: flex-start;
    margin-bottom: 4px;
    font-size: 20px;
    line-height: 1.2;
  }

  h2 {
    margin: 0;
    font-size: 22px;
    font-weight: 500;
    letter-spacing: -0.015em;
    line-height: 1.2;
    font-variation-settings:
      'SOFT' 50,
      'WONK' 1;
  }
</style>
