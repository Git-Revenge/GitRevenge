<script lang="ts">
  import { onMount } from 'svelte'
  import { BuiltWith } from '../../../wailsjs/go/app/App.js'
  import { BrowserOpenURL } from '../../../wailsjs/runtime/runtime.js'
  import Icon from '../ui/Icon.svelte'
  import InkButton from '../ui/InkButton.svelte'
  import Mark from '../ui/Mark.svelte'
  import Modal from '../ui/Modal.svelte'
  import { LOGOS, type LogoName } from './logos'

  let { version, onClose }: { version: string; onClose: () => void } = $props()

  const HOME = 'https://gitrevenge.com'
  const SPONSOR = 'https://github.com/sponsors/risadams'

  /** The projects the app is built on directly; nothing they bring along with them. */
  interface Credit {
    logo: LogoName
    name: string
    href: string
    license: string
    role: string
    version: string
  }

  let built = $state({ go: '', wails: '' })
  let actions: HTMLDivElement

  let credits = $derived<Credit[]>([
    { logo: 'go', name: 'Go', href: 'https://go.dev', license: 'BSD-3-Clause', role: 'The backend, and everything that talks to git.', version: built.go },
    { logo: 'svelte', name: 'Svelte', href: 'https://svelte.dev', license: 'MIT', role: "The interface you're looking at.", version: __SVELTE_VERSION__ },
    { logo: 'wails', name: 'Wails', href: 'https://wails.io', license: 'MIT', role: 'Joins the two into one desktop app.', version: built.wails },
  ])

  onMount(async () => {
    // Nothing here is a field, so the dialog would start on its first link. Done is the better
    // place: Enter closes it.
    actions.querySelector('button')?.focus()
    // The credits read fine without versions, so a failure here isn't worth a word.
    try {
      built = await BuiltWith()
    } catch {
      /* keep the names and licenses */
    }
  })

  /** Links open in the browser, never in the app's own window. */
  function open(e: MouseEvent, href: string) {
    e.preventDefault()
    BrowserOpenURL(href)
  }
</script>

<Modal title="Git Revenge" eyebrow="about" {onClose}>
  {#snippet mark()}<Mark size={52} />{/snippet}
  {#snippet subtitle()}
    <div class="sub">
      <span class="meta">Version {version}</span>
      <a class="site" href={HOME} onclick={(e) => open(e, HOME)}>gitrevenge.com</a>
    </div>
  {/snippet}

  <section class="sponsor">
    <p>Git Revenge is free and open source. If it's earned a place in your day, you can sponsor the work.</p>
    <div class="ask">
      <InkButton variant="secondary" onclick={() => BrowserOpenURL(SPONSOR)}><Icon name="external" />Sponsor on GitHub</InkButton>
      <span class="hand">thank you</span>
    </div>
  </section>

  <section>
    <h3 class="meta">Built with</h3>
    <ul>
      <li>
        <span class="logo love"><Icon name="heart" size={20} /></span>
        <span class="name">Love</span>
        <a class="aside" href="https://risadams.com" onclick={(e) => open(e, 'https://risadams.com')}>risadams.com</a>
        <span class="role">Built for you with love and coffee</span>
      </li>
      {#each credits as c (c.name)}
        <li>
          <svg class="logo" width="20" height="20" viewBox="0 0 24 24" aria-hidden="true" focusable="false"><path d={LOGOS[c.logo]} /></svg>
          <a class="name" href={c.href} onclick={(e) => open(e, c.href)}>{c.name}</a>
          <span class="meta">{c.version ? `${c.version} · ` : ''}{c.license}</span>
          <span class="role">{c.role}</span>
        </li>
      {/each}
    </ul>
  </section>

  <div class="form-actions" bind:this={actions}>
    <InkButton onclick={onClose}>Done</InkButton>
  </div>
</Modal>

<style>
  .sub {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 2px 12px;
  }

  /* An italic link with a painterly bar under it, as the text button has. */
  .site {
    position: relative;
    z-index: 0;
    padding: 0 2px;
    font-style: italic;
    text-decoration: none;
  }

  .site::after {
    content: '';
    position: absolute;
    z-index: -1;
    left: 0;
    right: 0;
    bottom: 1px;
    height: 0.4em;
    border-radius: 3px 6px 4px 5px;
    background: var(--accent);
    opacity: 0.18;
    pointer-events: none;
    transition: opacity var(--dur-base);
  }

  .site:hover::after,
  .site:focus-visible::after {
    opacity: 0.4;
  }

  /* The ask sits on a rose wash with a torn edge, not in a box. */
  .sponsor {
    position: relative;
    z-index: 0;
    display: flex;
    flex-direction: column;
    gap: 12px;
    margin-top: 2px;
    padding: 14px 16px 16px;
  }

  .sponsor::before {
    content: '';
    position: absolute;
    z-index: -1;
    inset: 0;
    background: var(--accent);
    opacity: 0.12;
    mix-blend-mode: multiply;
    filter: url(#wc-edge);
    border-radius: 22px 30px 26px 34px / 30px 24px 32px 22px;
    pointer-events: none;
  }

  :global(:root[data-theme='dark']) .sponsor::before {
    opacity: 0.14;
    mix-blend-mode: screen;
  }

  .sponsor p {
    margin: 0;
    font-size: 14px;
    line-height: 1.55;
    color: var(--ink-soft);
  }

  .ask {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px 16px;
  }

  .ask .hand {
    font-size: 20px;
  }

  h3 {
    margin: 0;
    padding-bottom: 4px;
    border-bottom: 1px solid var(--paper-edge);
    font-weight: normal;
    color: var(--ink-soft);
  }

  ul {
    margin: 0;
    padding: 0;
    list-style: none;
  }

  li {
    display: grid;
    grid-template-columns: 24px minmax(0, 1fr) auto;
    gap: 2px 12px;
    align-items: baseline;
    padding: 10px 0;
  }

  li + li {
    border-top: 1px dashed var(--hairline);
  }

  .logo {
    grid-row: 1 / span 2;
    align-self: center;
    fill: currentColor;
    color: var(--ink-soft);
  }

  .love {
    display: inline-flex;
    color: var(--link);
  }

  .love :global(path) {
    fill: currentColor;
  }

  .name {
    justify-self: start;
    font-size: 15px;
    font-weight: 500;
  }

  .aside {
    font-size: 13px;
    font-style: italic;
    white-space: nowrap;
  }

  li .meta {
    white-space: nowrap;
  }

  .role {
    grid-column: 2 / -1;
    font-size: 13px;
    line-height: 1.5;
    color: var(--ink-mute);
  }

  @media (forced-colors: active) {
    .sponsor::before,
    .site::after {
      display: none;
    }
  }
</style>
