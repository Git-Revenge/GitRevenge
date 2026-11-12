import type { Component } from 'svelte'
import CsvPreview from './CsvPreview.svelte'
import HtmlPreview from './HtmlPreview.svelte'
import MediaPreview from './MediaPreview.svelte'
import { mediaOf } from './media'
import MarkdownPreview from './MarkdownPreview.svelte'
import MermaidPreview from './MermaidPreview.svelte'
import PdfPreview from './PdfPreview.svelte'
import TexPreview from './TexPreview.svelte'
import TreePreview from './TreePreview.svelte'
import { formatOf } from './structured'

/** What every previewer is given: the file, its text and ways to reach the rest of the repo. */
export interface PreviewProps {
  path: string
  text: string
  /** Loads an image in the repository, at the same version, as a data URL ("" if none). */
  loadImage: (file: string) => Promise<string>
  /** Opens another file in the repository, at the same version. */
  openFile: (file: string) => void
  /** This file's bytes as base64, for binary formats. */
  loadData?: () => Promise<string>
}

export interface Previewer {
  id: string
  matches: (path: string) => boolean
  component: Component<PreviewProps>
  /** Reads the file's bytes (loadData) instead of its text, and shows for binary files. */
  binary?: boolean
  /** Shows without asking for a file over the text size limit: it reads the bytes itself, up
   * to the preview limit, and a big file is the usual case (audio, video). */
  large?: boolean
}

/**
 * Rendered previews for File mode, by file type. Adding a type is one entry here and one
 * component. SVG isn't listed: it's classified as an image and already shows through an
 * <img> element, where its scripts never run.
 */
export const PREVIEWERS: Previewer[] = [
  { id: 'markdown', matches: (p) => /\.(md|mdx|markdown|mdown|mkd|mkdn)$/i.test(p), component: MarkdownPreview },
  { id: 'csv', matches: (p) => /\.(csv|tsv|tab)$/i.test(p), component: CsvPreview },
  { id: 'structured', matches: (p) => formatOf(p) !== null, component: TreePreview },
  { id: 'tex', matches: (p) => /\.(tex|latex)$/i.test(p), component: TexPreview },
  { id: 'mermaid', matches: (p) => /\.(mmd|mermaid)$/i.test(p), component: MermaidPreview },
  { id: 'html', matches: (p) => /\.(html?|xhtml)$/i.test(p), component: HtmlPreview },
  { id: 'pdf', matches: (p) => /\.pdf$/i.test(p), component: PdfPreview, binary: true },
  { id: 'media', matches: (p) => mediaOf(p) !== null, component: MediaPreview, binary: true, large: true },
]

export function previewerFor(path: string): Previewer | undefined {
  return PREVIEWERS.find((p) => p.matches(path))
}
