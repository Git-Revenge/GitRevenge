// @ts-check

// The docs in the order a newcomer reads them: install, a tour of the window, walkthroughs of
// common jobs, then the guide to each part of the app. The project's own papers (plan,
// releasing, decisions) come last, collapsed.
//
// Every page is listed by hand, under a short label, so the order is the one written here
// and never a file name's. src/plugins/sidebar-complete.js fails the build if a page under
// docs/ is missing from this file.

/** @type {import('@docusaurus/plugin-content-docs').SidebarsConfig} */
const sidebars = {
  mainSidebar: [
    { type: 'doc', id: 'guide/README', label: 'Introduction' },
    { type: 'doc', id: 'guide/install', label: 'Installing' },
    { type: 'doc', id: 'guide/tour', label: 'A tour of the window' },
    {
      type: 'category',
      label: 'Walkthroughs',
      link: { type: 'doc', id: 'walkthroughs/README' },
      items: [
        { type: 'doc', id: 'walkthroughs/first-commit', label: 'Stage and commit' },
        { type: 'doc', id: 'walkthroughs/tidy-a-branch', label: 'Tidy a branch' },
        { type: 'doc', id: 'walkthroughs/resolve-a-conflict', label: 'Resolve a conflict' },
        { type: 'doc', id: 'walkthroughs/undo-a-mistake', label: 'Undo a mistake' },
        { type: 'doc', id: 'walkthroughs/find-a-commit', label: 'Find a commit' },
        { type: 'doc', id: 'walkthroughs/several-repositories', label: 'Several repositories' },
      ],
    },
    {
      type: 'category',
      label: 'User guide',
      items: [
        { type: 'doc', id: 'guide/repositories', label: 'Repositories and tabs' },
        { type: 'doc', id: 'guide/graph', label: 'The commit graph' },
        { type: 'doc', id: 'guide/changes', label: 'Changes and commits' },
        { type: 'doc', id: 'guide/branches', label: 'Branches, merges and rebases' },
        { type: 'doc', id: 'guide/branch-care', label: 'Branch care' },
        { type: 'doc', id: 'guide/history', label: 'Diffs and history' },
        { type: 'doc', id: 'guide/remotes', label: 'Remotes, LFS and submodules' },
        { type: 'doc', id: 'guide/git-config', label: 'Git config' },
        { type: 'doc', id: 'guide/insights', label: 'Insights' },
        { type: 'doc', id: 'guide/settings', label: 'Settings and shortcuts' },
      ],
    },
    {
      type: 'category',
      label: 'The project',
      collapsed: true,
      items: [
        { type: 'doc', id: 'PLAN', label: 'Plan' },
        { type: 'doc', id: 'RELEASING', label: 'Releasing' },
        {
          type: 'category',
          label: 'Design decisions',
          collapsed: true,
          items: [
            { type: 'doc', id: 'adr/wails-desktop-shell', label: 'ADR-0001 Wails desktop shell' },
            { type: 'doc', id: 'adr/system-git-cli', label: 'ADR-0002 The system git' },
            { type: 'doc', id: 'adr/svelte-canvas-graph', label: 'ADR-0003 A canvas graph' },
            { type: 'doc', id: 'adr/askpass-bridge', label: 'ADR-0004 The askpass bridge' },
            { type: 'doc', id: 'adr/local-state-and-external-changes', label: 'ADR-0005 Local state' },
            { type: 'doc', id: 'adr/docs-site', label: 'ADR-0006 The docs site' },
            { type: 'doc', id: 'adr/git-config-editing', label: 'ADR-0007 Editing git config' },
            { type: 'doc', id: 'adr/insights-from-repository-state', label: 'ADR-0008 Reproducible insights' },
            { type: 'doc', id: 'adr/branch-kinds', label: 'ADR-0009 Branch kinds' },
            { type: 'doc', id: 'adr/linux-first-class', label: 'ADR-0010 Linux is first-class' },
            { type: 'doc', id: 'adr/repository-settings-in-git-config', label: 'ADR-0011 Repository settings in git config' },
          ],
        },
      ],
    },
  ],
};

module.exports = sidebars;
