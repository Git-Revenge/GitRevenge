# 0006 — A documentation website built with Docusaurus

- Status: Accepted
- Date: 2026-10-30

## Context

Git Revenge's documentation was the README and the project's own papers in `docs/`: the plan, the release process and these decisions. There was no user guide, and Markdown on GitHub has no navigation between pages, no search and no front page.

The owner's other projects (risadams/Janus, risadams/docker-registry.helm) publish their docs as a Docusaurus site on GitHub Pages, in the Ris Ink look of risadams.com. Doing the same here keeps one way of working and one theme across them.

## Decision

- **`docs-site/` holds a Docusaurus 3 site**, copied in structure and theme from Janus's: the same `custom.css` and code palette, a homepage, a navbar logo and a 404 page. The classic preset reads `../docs` directly, so the Markdown in `docs/` stays the only copy and still reads correctly on GitHub.
- **`.md` files are read as CommonMark, not MDX**, so the docs need nothing Docusaurus-specific.
- **Links that leave `docs/`** (the README, CONTRIBUTING, source files) are rewritten to GitHub by `src/remark/repo-links.js`, rather than edited in the Markdown.
- **The sidebar is written by hand** in `sidebars.js`, every page under a short label in the order a reader meets it. `src/plugins/sidebar-complete.js` fails the build if a page is in no sidebar.
- **Broken links and anchors fail the build.** The `docs` workflow builds the site on pull requests that touch it, and publishes it to the `gh-pages` branch on pushes to `main` and on release tags.
- **It's served at gitrevenge.com.** `docs-site/static/CNAME` names the domain, so every publish carries it; the deploy replaces the whole `gh-pages` branch, and without the file each publish would drop the custom domain. *Added 2026-10-30.*
- **Screenshots are generated, never taken by hand.** A script builds Tidewater, a made-up repository with fixed authors and dates (since #736 in its own repository, [Git-Revenge/demo-repos](https://github.com/Git-Revenge/demo-repos), pinned by tag), and `docs-site/scripts/screenshots.mjs` drives the app (`wails dev`) in Chrome to capture each still in a light and a dark version, and each walkthrough as an animated WebP. Retaking them all after a UI change is one command, and no real repository or person appears in them. Pages show the version matching the reader's theme by GitHub's `#gh-light-mode-only` / `#gh-dark-mode-only` convention, which `custom.css` applies on the site too. *Added 2026-10-30 (#219).*
- **Its npm dependencies are separate from the app's.** Nothing under `docs-site/` is imported by the app, and the Go and frontend builds don't need it.

## Consequences

- Easier: reading the docs, linking to a page, and finding what a page is for. A broken link in `docs/` is caught by CI.
- Harder: a second Node toolchain to keep current, and a page that renders on GitHub can fail the site's stricter link check. The theme is shared with other repositories, so a change to it is best made in all of them.
- The site shows `main`'s docs, which can describe features the newest release doesn't have yet. Janus solved this with a stable version built from a `docs/vX.Y` branch (its ADR-002); that can be added here when it matters.
