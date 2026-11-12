<script lang="ts">
  // Markdown rendered for a commit message, or a whole document (a file's rendered
  // preview). marked only tokenizes; every token becomes a Svelte element, so nothing is ever
  // inserted as HTML: raw HTML shows as its text. Web links open in the system browser.
  import { marked, type Token, type Tokens } from 'marked'
  import { BrowserOpenURL } from '../../wailsjs/runtime/runtime.js'
  import MarkdownImage from './MarkdownImage.svelte'
  import MathView from './markdown/MathView.svelte'
  import MermaidDiagram from './markdown/MermaidDiagram.svelte'
  import { mathMarked, type MathToken } from './markdown/math'
  import { linkTitle, webOnly, type LinkPart, type Linker } from './links/links'

  let {
    text,
    linker = webOnly,
    onLinkMenu,
    document = false,
    onLink,
    resolveImage,
    math = false,
    breaks = false,
  }: {
    text: string
    /** Turns text into links: web addresses, and in a commit message the repository's smart
     * links. */
    linker?: Linker
    /** Opens a menu for a smart link, such as Copy link. */
    onLinkMenu?: (e: MouseEvent, part: LinkPart) => void
    /** A whole document: keeps heading levels and sets a reading size. */
    document?: boolean
    /** Handles a relative link, such as another file in the repository. */
    onLink?: (href: string) => void
    /** Loads a relative image as a data URL; without it, images show as links. */
    resolveImage?: (src: string) => Promise<string>
    /** Reads $…$ and $$…$$ as math; off for commit messages, where "$5" is money. */
    math?: boolean
    /** Keeps a single line break as a line break, for short text written a line at a time,
     * such as a note. Markdown proper joins such lines into one paragraph. */
    breaks?: boolean
  } = $props()

  let tokens = $derived(math ? mathMarked.lexer(text) : marked.lexer(text, { gfm: true, breaks }))

  const WEB_LINK = /^(https?:|mailto:)/i
  /** A relative path: no scheme, not protocol-relative, not just a fragment. */
  const RELATIVE = /^(?![a-z][a-z0-9+.-]*:|\/\/|#)/i

  function open(e: MouseEvent, href: string) {
    e.preventDefault()
    if (WEB_LINK.test(href)) BrowserOpenURL(href)
    else if (RELATIVE.test(href)) onLink?.(href)
  }

  /** Heading level in a document (h2 down, under the view's own heading), or h3 in a message. */
  const headingTag = (depth: number) => (document ? `h${Math.min(6, depth + 1)}` : 'h3')
</script>

{#snippet links(value: string)}{#each linker.parts(value, 'message') as part, k (k)}{#if part.href}<a
        href={part.href}
        title={linkTitle(part)}
        class:smart={!!part.rule}
        onclick={(e) => open(e, part.href!)}
        oncontextmenu={(e) => onLinkMenu?.(e, part)}>{part.text}</a
      >{:else}{part.text}{/if}{/each}{/snippet}

<!-- inLink: already inside an <a>, so nothing here may become another one. -->
{#snippet inline(list: Token[], inLink = false)}{#each list as t, i (i)}{#if t.type === 'text' && 'tokens' in t && t.tokens?.length}{@render inline(
        t.tokens,
        inLink,
      )}{:else if t.type === 'text' || t.type === 'escape'}{#if inLink}{t.text}{:else}{@render links(t.text)}{/if}{:else if t.type === 'strong'}<strong
        >{@render inline((t as Tokens.Strong).tokens, inLink)}</strong
      >{:else if t.type === 'em'}<em>{@render inline((t as Tokens.Em).tokens, inLink)}</em>{:else if t.type === 'del'}<del
        >{@render inline((t as Tokens.Del).tokens, inLink)}</del
      >{:else if t.type === 'codespan'}<code>{t.text}</code>{:else if t.type === 'br'}<br />{:else if t.type === 'link'}<a
        href={(t as Tokens.Link).href}
        title={(t as Tokens.Link).href}
        onclick={(e) => open(e, (t as Tokens.Link).href)}>{@render inline((t as Tokens.Link).tokens, true)}</a
      >{:else if t.type === 'image'}{#if resolveImage && RELATIVE.test((t as Tokens.Image).href)}<MarkdownImage
          src={(t as Tokens.Image).href}
          alt={(t as Tokens.Image).text}
          resolve={resolveImage}
        />{:else if inLink}{(t as Tokens.Image).text || (t as Tokens.Image).href}{:else}<a href={(t as Tokens.Image).href} title="Image (not loaded)" onclick={(e) => open(e, (t as Tokens.Image).href)}
          >{(t as Tokens.Image).text || (t as Tokens.Image).href}</a
        >{/if}{:else if t.type === 'checkbox'}<input type="checkbox" checked={(t as Tokens.Checkbox).checked} disabled />{:else if t.type === 'math'}<MathView
          tex={(t as unknown as MathToken).text}
        />{:else}{t.raw}{/if}{/each}{/snippet}

{#snippet blocks(list: Token[])}
  {#each list as t, i (i)}
    {#if t.type === 'paragraph'}
      <p>{@render inline((t as Tokens.Paragraph).tokens)}</p>
    {:else if t.type === 'heading'}
      <svelte:element this={headingTag((t as Tokens.Heading).depth)}>{@render inline((t as Tokens.Heading).tokens)}</svelte:element>
    {:else if t.type === 'code' && document && (t as Tokens.Code).lang?.trim().toLowerCase() === 'mermaid'}
      <MermaidDiagram code={(t as Tokens.Code).text} />
    {:else if t.type === 'code'}
      <pre><code>{(t as Tokens.Code).text}</code></pre>
    {:else if t.type === 'blockquote'}
      <blockquote>{@render blocks((t as Tokens.Blockquote).tokens)}</blockquote>
    {:else if t.type === 'list'}
      {@const l = t as Tokens.List}
      {#if l.ordered}
        <ol start={Number(l.start) || 1}>
          {#each l.items as item, k (k)}<li>{@render blocks(item.tokens)}</li>{/each}
        </ol>
      {:else}
        <ul class:tasks={l.items.some((it) => it.task)}>
          {#each l.items as item, k (k)}<li>{@render blocks(item.tokens)}</li>{/each}
        </ul>
      {/if}
    {:else if t.type === 'table'}
      {@const tb = t as Tokens.Table}
      <div class="table">
        <table>
          <thead>
            <tr>{#each tb.header as cell, k (k)}<th>{@render inline(cell.tokens)}</th>{/each}</tr>
          </thead>
          <tbody>
            {#each tb.rows as row, r (r)}
              <tr>{#each row as cell, k (k)}<td>{@render inline(cell.tokens)}</td>{/each}</tr>
            {/each}
          </tbody>
        </table>
      </div>
    {:else if t.type === 'hr'}
      <hr />
    {:else if t.type === 'math'}
      <MathView tex={(t as unknown as MathToken).text} display />
    {:else if t.type === 'text' || t.type === 'checkbox'}
      {@render inline([t])}
    {:else if t.type === 'html'}
      <p>{t.raw}</p>
    {:else if t.type !== 'space' && t.type !== 'def'}
      <p>{t.raw}</p>
    {/if}
  {/each}
{/snippet}

<div class="markdown" class:document>{@render blocks(tokens)}</div>

<style>
  .markdown {
    font-size: 15px;
    line-height: 1.6;
    color: var(--ink-soft);
    overflow-wrap: anywhere;
  }

  .markdown > :global(:first-child) {
    margin-top: 0;
  }

  p,
  ul,
  ol,
  pre,
  blockquote,
  .table {
    margin: 0 0 10px;
  }

  .markdown :global(:is(h2, h3, h4, h5, h6)) {
    margin: 14px 0 6px;
    font-size: 15px;
    font-weight: 600;
    color: var(--ink);
  }

  /* A document reads at body size, with a heading scale and a measure. */
  .document {
    max-width: 72ch;
    font-size: 16px;
    line-height: 1.65;
    color: var(--ink);
  }

  .document :global(h2) {
    margin: 22px 0 10px;
    font-size: 28px;
    font-weight: 500;
    letter-spacing: -0.01em;
  }

  .document :global(h3) {
    margin: 20px 0 8px;
    font-size: 22px;
    font-weight: 500;
  }

  .document :global(h4) {
    font-size: 18px;
  }

  ul,
  ol {
    padding-left: 22px;
  }

  ul.tasks {
    list-style: none;
    padding-left: 4px;
  }

  li > :global(p) {
    margin: 0;
  }

  code {
    font-family: var(--font-mono);
    font-size: 13px;
    color: var(--accent-b);
    background: var(--paper-warm);
    padding: 1px 4px;
    border-radius: var(--r-sm);
  }

  pre {
    overflow-x: auto;
    padding: 10px 12px;
    background: var(--paper-warm);
    border-radius: var(--r-sm);
  }

  pre code {
    padding: 0;
    background: none;
    color: var(--ink);
    white-space: pre;
  }

  blockquote {
    padding-left: 12px;
    border-left: 3px solid var(--paper-edge);
    color: var(--ink-mute);
  }

  .table {
    overflow-x: auto;
  }

  table {
    border-collapse: collapse;
    font-size: 13px;
  }

  th,
  td {
    padding: 3px 8px;
    border: 1px solid var(--paper-edge);
    text-align: left;
  }

  hr {
    border: none;
    border-top: 1px dotted var(--paper-edge);
    margin: 12px 0;
  }

  a {
    color: var(--link);
  }

  a:hover {
    color: var(--link-hover);
  }

  input[type='checkbox'] {
    margin: 0 4px 0 0;
    vertical-align: -1px;
  }
</style>
