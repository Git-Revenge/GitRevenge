import { joinParts } from './iconPath'

// The icon system (#171): one round-capped pen on a 24 unit grid, 2 units of padding,
// corners of radius 2, drawn with currentColor. Each icon is a single path, so every part
// shares the stroke. File status icons differ by their mark, never by colour alone.

/** A circle as path data. */
const c = (x: number, y: number, r: number) => `M${x - r} ${y}a${r} ${r} 0 1 0 ${2 * r} 0a${r} ${r} 0 1 0 ${-2 * r} 0`

/** A rounded rectangle as path data. */
const rr = (x: number, y: number, w: number, h: number, r: number) =>
  `M${x + r} ${y}h${w - 2 * r}a${r} ${r} 0 0 1 ${r} ${r}v${h - 2 * r}a${r} ${r} 0 0 1 ${-r} ${r}` +
  `h${-(w - 2 * r)}a${r} ${r} 0 0 1 ${-r} ${-r}v${-(h - 2 * r)}a${r} ${r} 0 0 1 ${r} ${-r}z`

/** A dot, for the i of info and the point of warning. */
const dot = (x: number, y: number) => c(x, y, 0.6)

export const ICONS = {
  // Repository and graph
  commit: [c(12, 12, 3.5), 'M3 12h5.5M15.5 12H21'],
  branch: [c(6, 5.5, 2.5), c(6, 18.5, 2.5), c(18, 7, 2.5), 'M6 8v8', 'M18 9.5c0 4.5-5 5-10 7.5'],
  merge: [c(6, 5.5, 2.5), c(6, 18.5, 2.5), c(18, 12, 2.5), 'M6 8v8', 'M6 8c0 3 4 4 9.5 4'],
  // The main line, the branch's commit lifted onto its new base, and the arrow that moved it.
  rebase: ['M6 3v18', 'M6 10.5C6 8 9 6.5 15 6.5', c(17.5, 6.5, 2.5), 'M17.5 20v-8.5', 'm14.5 14.5 3-3 3 3'],
  'cherry-pick': [c(7, 17, 3), c(16.5, 18, 2.5), 'M7 14c.6-5 3.4-8.6 8.5-10.5', 'M16.5 15.5c-.3-4-.6-7.6-1-12'],
  tag: ['M3.5 12.3V4.8a1.3 1.3 0 0 1 1.3-1.3h7.5l8.2 8.2a1.4 1.4 0 0 1 0 2l-6.8 6.8a1.4 1.4 0 0 1-2 0z', c(8.5, 8.5, 1.5)],
  // Branch kinds (ADR 0009). Trunk: the main line, two commits strung on it.
  trunk: ['M12 3v3', c(12, 8.5, 2.5), 'M12 11v2', c(12, 15.5, 2.5), 'M12 18v3'],
  // Release: a package ready to ship, its tape across the lid.
  package: ['M12 3l8.5 4.5v9L12 21l-8.5-4.5v-9z', 'M3.5 7.5 12 12l8.5-4.5', 'M12 12v9', 'M7.75 5.25l8.5 4.5'],
  // Environment: two servers in a rack, a light on each.
  server: [rr(3.5, 4, 17, 7, 2), rr(3.5, 13, 17, 7, 2), dot(7, 7.5), dot(7, 16.5), 'M11 7.5h6', 'M11 16.5h6'],
  // Docs: an open book.
  book: ['M12 6.5C10 5 7 4.5 3.5 5v13c3.5-.5 6.5 0 8.5 1.5', 'M12 6.5c2-1.5 5-2 8.5-1.5v13c-3.5-.5-6.5 0-8.5 1.5', 'M12 6.5v13'],
  // Stacked branches: three layers, one on another.
  stack: ['M12 3 3.5 7.5 12 12l8.5-4.5z', 'm3.5 12 8.5 4.5 8.5-4.5', 'm3.5 16.5 8.5 4.5 8.5-4.5'],
  // Bot: a robot's head, antenna up.
  bot: [rr(4.5, 8, 15, 11, 3), 'M12 8V5.5', c(12, 4, 1.5), dot(9.5, 13), dot(14.5, 13), 'M10 16h4', 'M2.5 12v3', 'M21.5 12v3'],
  // Feature: a young branch, a stem and two leaves.
  sprout: ['M12 21v-9', 'M12 12C12 8 9 5.5 4.5 5.5 4.5 10 7.5 12 12 12z', 'M12 15c0-3 2.5-5.5 7-5.5 0 4-3 5.5-7 5.5z'],
  stash: [
    'M4.5 4h15a1 1 0 0 1 1 1v3a1 1 0 0 1-1 1h-15a1 1 0 0 1-1-1V5a1 1 0 0 1 1-1z',
    'M5 9v9.5A1.5 1.5 0 0 0 6.5 20h11a1.5 1.5 0 0 0 1.5-1.5V9',
    'M10 13h4',
  ],
  reset: ['M11 18 4 12l7-6z', 'M20 18l-7-6 7-6z'],
  revert: [c(12, 16, 3), 'M3 16h6M15 16h6', 'M7.5 9.5A6 6 0 0 1 17 6.5', 'M17.5 3.5v3.5H14'],
  conflict: ['M12 3l9 9-9 9-9-9z', 'M9.5 9.5l5 5M14.5 9.5l-5 5'],
  worktree: ['M12 3 3 7.5l9 4.5 9-4.5z', 'M3 12l9 4.5 9-4.5', 'M3 16.5l9 4.5 9-4.5'],
  submodule: [rr(3, 3, 18, 18, 2.5), rr(8, 8, 8, 8, 1.5)],
  lfs: ['M12 3 20 7.5v9L12 21l-8-4.5v-9z', 'M4 7.5 12 12l8-4.5', 'M12 12v9'],
  remote: ['M7 18.5h10a4 4 0 0 0 .6-7.95A6 6 0 0 0 6.2 9.6 4.5 4.5 0 0 0 7 18.5z'],
  signed: ['M12 3 19.5 6v5.5c0 4.5-3.2 8-7.5 9.5-4.3-1.5-7.5-5-7.5-9.5V6z', 'm9 12 2 2 4-4'],

  // Sync
  fetch: ['M21 12a9 9 0 0 1-15 6.7L3 16', 'M3 12a9 9 0 0 1 15-6.7L21 8', 'M21 3v5h-5', 'M3 21v-5h5'],
  pull: ['M12 3v12', 'm7 10 5 5 5-5', 'M5 21h14'],
  push: ['M12 21V9', 'm7 14 5-5 5 5', 'M5 3h14'],
  clone: ['M12 3v10', 'm8 9 4 4 4-4', 'M4 14v4a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-4'],
  update: [c(12, 12, 9), 'M12 16V8', 'm8.5 11.5 3.5-3.5 3.5 3.5'],

  // Files and diffs
  file: ['M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8z', 'M14 3v5h5'],
  folder: ['M3 6.5A1.5 1.5 0 0 1 4.5 5h4l2 2.5h9A1.5 1.5 0 0 1 21 9v9.5a1.5 1.5 0 0 1-1.5 1.5h-15A1.5 1.5 0 0 1 3 18.5z'],
  'folder-open': ['M3.5 19.5 6 11h15.5L19 19.5z', 'M3 18V6.5A1.5 1.5 0 0 1 4.5 5h4l2 2.5h7A1.5 1.5 0 0 1 19 9v2'],
  diff: ['M12 4v10M7 9h10', 'M7 19h10'],
  split: [rr(3, 4, 18, 16, 2), 'M12 4v16'],
  unified: [rr(3, 4, 18, 16, 2), 'M7 9h10M7 12h10M7 15h6'],
  wrap: ['M4 6h16', 'M4 12h13a3 3 0 0 1 0 6h-4', 'm15 16-2 2 2 2', 'M4 18h5'],
  blame: [c(8, 8, 3), 'M3 19a5 5 0 0 1 10 0', 'M15 8h6M15 12h6M17 16h4'],
  history: ['M3.5 12a8.5 8.5 0 1 0 2.5-6', 'M3 4v4h4', 'M12 7.5V12l3 2'],
  // A frame, a sun and two hills that stay inside the frame.
  image: [rr(3, 4, 18, 16, 2), c(8.5, 9, 1.75), 'M3 17l5.5-5.5 6 6', 'M13 15.5l2.5-2.5 5.5 5.5'],

  // Actions
  plus: ['M12 5v14M5 12h14'],
  minus: ['M5 12h14'],
  check: ['M5 12.5l4.5 4.5L19 7.5'],
  close: ['M18 6 6 18M6 6l12 12'],
  edit: ['M15.5 4.5l4 4L8 20H4v-4z', 'M13 7l4 4'],
  copy: [rr(8, 8, 12, 12, 2), 'M16 8V6a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h2'],
  trash: ['M4 6.5h16', 'M9 6.5v-2h6v2', 'M6 6.5l1 13a1.5 1.5 0 0 0 1.5 1.5h7a1.5 1.5 0 0 0 1.5-1.5l1-13', 'M10 10.5v6M14 10.5v6'],
  undo: ['M9 14 4 9l5-5', 'M4 9h10.5a5.5 5.5 0 0 1 0 11H11'],
  redo: ['m15 14 5-5-5-5', 'M20 9H9.5a5.5 5.5 0 0 0 0 11H13'],
  refresh: ['M21 12a9 9 0 1 1-2.6-6.4', 'M21 3v6h-6'],
  search: [c(11, 11, 7), 'm20 20-3.5-3.5'],
  filter: ['M4 5h16l-6 7.5V19l-4-2v-4.5z'],
  lock: [rr(5, 11, 14, 9, 2), 'M8 11V8a4 4 0 0 1 8 0v3'],
  unlock: [rr(5, 11, 14, 9, 2), 'M8 11V8a4 4 0 0 1 7.5-2'],
  external: ['M14 4h6v6', 'M20 4l-9 9', 'M18 14v4a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h4'],

  // Navigation and chrome
  home: ['M3 10.5 12 3l9 7.5', 'M5 9.5V21h14V9.5'],
  back: ['M19 12H5', 'm12 19-7-7 7-7'],
  chevron: ['m6 9 6 6 6-6'],
  right: ['m9 6 6 6-6 6'],
  more: [c(5, 12, 1), c(12, 12, 1), c(19, 12, 1)],
  grip: [c(9, 6, 1), c(15, 6, 1), c(9, 12, 1), c(15, 12, 1), c(9, 18, 1), c(15, 18, 1)],
  sidebar: [rr(3, 4, 18, 16, 2), 'M9 4v16'],
  command: ['M9 6a3 3 0 1 0-3 3h12a3 3 0 1 0-3-3v12a3 3 0 1 0 3-3H6a3 3 0 1 0 3 3z'],
  keyboard: [rr(2.5, 6, 19, 12, 2), dot(6.5, 10), dot(10, 10), dot(13.5, 10), dot(17, 10), 'M7.5 14h9'],
  gear: [
    c(12, 12, 3),
    'M19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.8-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1.1-1.5 1.7 1.7 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.8 1.7 1.7 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.5-1.1 1.7 1.7 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.8.3H9a1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.8V9a1.7 1.7 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1z',
  ],
  terminal: [rr(3, 4.5, 18, 15, 2), 'm7 9.5 3 2.5-3 2.5', 'M12.5 15H17'],
  eye: ['M2.5 12S6 5.5 12 5.5 21.5 12 21.5 12 18 18.5 12 18.5 2.5 12 2.5 12z', c(12, 12, 3)],
  info: [c(12, 12, 9), 'M12 11v5', dot(12, 8)],
  warning: ['M12 3.5 2.5 20h19z', 'M12 10v4', dot(12, 17)],
  sun: [c(12, 12, 4), 'M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4'],
  moon: ['M20 14.5A8 8 0 1 1 9.5 4a6.5 6.5 0 0 0 10.5 10.5z'],
  rows: ['M4 6h16M4 10h16M4 14h16M4 18h16'],
  list: ['M9 6h11M9 12h11M9 18h11', dot(4.5, 6), dot(4.5, 12), dot(4.5, 18)],
  // The graph's timeline density: commits on a line beside their text.
  timeline: [c(6, 7, 2.5), c(6, 17, 2.5), 'M6 9.5v5M11 6h9M11 8.5h6M11 16h9M11 18.5h6'],

  // File status: an outline plus a mark, so shape tells them apart
  'st-added': [rr(4, 4, 16, 16, 3), 'M12 8.5v7M8.5 12h7'],
  'st-modified': [rr(4, 4, 16, 16, 3), c(12, 12, 2.25)],
  'st-deleted': [rr(4, 4, 16, 16, 3), 'M8.5 12h7'],
  'st-renamed': [rr(4, 4, 16, 16, 3), 'M8 12h7.5', 'm12.5 9 3 3-3 3'],
  'st-untracked': [rr(4, 4, 16, 16, 3), 'M10 10a2 2 0 1 1 3 1.7c-.6.4-1 .8-1 1.5', dot(12, 16)],
  'st-conflict': ['M12 3l9 9-9 9-9-9z', 'M12 8.5v4.5', dot(12, 16)],
} satisfies Record<string, string[]>

export const PATHS = Object.fromEntries(Object.entries(ICONS).map(([k, parts]) => [k, joinParts(parts)])) as Record<IconName, string>

export type IconName = keyof typeof ICONS
