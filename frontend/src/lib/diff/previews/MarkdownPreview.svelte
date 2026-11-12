<script lang="ts">
  // A markdown file rendered as a document (and AsciiDoc and reStructuredText, read into it). Relative images load from the repository at the
  // same version; relative links open that file. Remote images aren't fetched. MDX shows as the
  // Markdown it contains; none of its code runs.
  import Markdown from '../../Markdown.svelte'
  import type { PreviewProps } from './registry'
  import { mdxToMarkdown } from '../../markdown/mdx'
  import { asciidocToMarkdown } from '../../markdown/asciidoc'
  import { rstToMarkdown } from '../../markdown/rst'
  import { resolveInRepo } from './paths'

  let { path, text, loadImage, openFile }: PreviewProps = $props()

  // AsciiDoc and reStructuredText are read into Markdown, and shown the same way: nothing in
  // them becomes HTML.
  let markdown = $derived(/\.mdx$/i.test(path) ? mdxToMarkdown(text) : /\.(adoc|asciidoc|asc)$/i.test(path) ? asciidocToMarkdown(text) : /\.(rst|rest)$/i.test(path) ? rstToMarkdown(text) : text)

  async function resolveImage(src: string): Promise<string> {
    const file = resolveInRepo(path, src)
    return file ? loadImage(file) : ''
  }

  function onLink(href: string) {
    const file = resolveInRepo(path, href)
    if (file) openFile(file)
  }
</script>

<div class="preview"><Markdown text={markdown} document math {resolveImage} {onLink} /></div>

<style>
  .preview {
    padding: 24px 32px 40px;
    background: var(--paper);
  }
</style>
