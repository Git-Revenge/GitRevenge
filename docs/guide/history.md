# Diffs and history

## Diffs

Click a file in a commit, or in your uncommitted changes, to see its diff. Pick a layout:

- **Unified:** one column, with removed and added lines interleaved. This is the layout for [staging hunks and lines](changes.md#staging).
- **Split:** the old and new versions side by side.
- **File:** the whole file as of that version.

Code is syntax highlighted. Images (PNG, JPEG, GIF, WebP, AVIF, BMP, ICO and SVG) are shown as pictures and compared side by side, fitted to the view or at actual size. <kbd>Alt</kbd>+<kbd>Z</kbd> turns word wrap on and off. Press <kbd>Escape</kbd> to go back to the graph.

## Rendered previews

Some files read better rendered than as text. In the **File** layout, these get a **Rendered** option, and the app remembers whether you left it on:

| Files | Shown as |
|---|---|
| Markdown (`.md`, `.markdown`) | formatted text, including LaTeX math and Mermaid diagrams |
| MDX (`.mdx`) | the same, showing the Markdown it contains. Imports, exports and frontmatter are hidden, a component shows what's inside it, and one with nothing inside shows its name. None of its code runs |
| CSV and TSV (`.csv`, `.tsv`, `.tab`) | a table you can sort by any column |
| JSON, YAML and TOML | a tree you can expand and search |
| LaTeX (`.tex`, `.latex`) | typeset math |
| Mermaid (`.mmd`, `.mermaid`) | the diagram |
| PDF | the document |

![A README shown rendered](../img/preview-markdown-light.webp#gh-light-mode-only)
![A README shown rendered](../img/preview-markdown-dark.webp#gh-dark-mode-only)
![A CSV file shown as a table](../img/preview-csv-light.webp#gh-light-mode-only)
![A CSV file shown as a table](../img/preview-csv-dark.webp#gh-dark-mode-only)
![A JSON file shown as a tree you can search](../img/preview-json-light.webp#gh-light-mode-only)
![A JSON file shown as a tree you can search](../img/preview-json-dark.webp#gh-dark-mode-only)
![A Mermaid diagram, drawn](../img/preview-mermaid-light.webp#gh-light-mode-only)
![A Mermaid diagram, drawn](../img/preview-mermaid-dark.webp#gh-dark-mode-only)

## File history and blame

From a file's diff, **History** lists every commit that changed it, and **Blame** shows who last changed each line and in which commit.

![Blame: each line with the commit and author that last changed it](../img/blame-light.webp#gh-light-mode-only)
![Blame: each line with the commit and author that last changed it](../img/blame-dark.webp#gh-dark-mode-only)


## The reflog

The reflog is git's record of where HEAD and each branch have been. It's how you find a commit you thought you'd lost to a reset or a rebase. Choose **Show the reflog…** in the command palette for HEAD's, or right-click a branch for its own.

![The reflog](../img/reflog-light.webp#gh-light-mode-only)
![The reflog](../img/reflog-dark.webp#gh-dark-mode-only)

[Undo a mistake](../walkthroughs/undo-a-mistake.md) walks through recovering a commit with it.

## Notes

Git notes attach extra text to a commit without changing it. The details panel shows a selected commit's notes, and lets you add, edit or remove one.

git doesn't fetch or push notes on its own. To share them, turn on [syncing notes](branch-care.md#syncing-notes) for the repository.

## Signatures

A signed commit shows a badge in its details:

| Badge | Meaning |
|---|---|
| Verified | The signature is good, from a key you trust. |
| Can't verify | The signature may be fine, but your machine has no trusted key to check it against. |
| Bad signature | The commit doesn't match its signature. |
| Expired | The signature or its key has expired. |
| Revoked | The key has been revoked. |

### Why a signature says "Can't verify"

Git checks signatures with your own keys and settings, so this usually means something is missing locally rather than wrong with the commit.

- **SSH-signed commits** need an allowed-signers file listing whose key is whose. Point git at one with `gpg.ssh.allowedSignersFile`, and add a line for each signer:

  ```sh
  git config --global gpg.ssh.allowedSignersFile ~/.ssh/allowed_signers
  echo "you@example.com $(cat ~/.ssh/id_ed25519.pub)" >> ~/.ssh/allowed_signers
  ```

- **Commits merged on GitHub** are signed by GitHub's own GPG key. Import it, then tell GPG you trust it:

  ```sh
  curl -s https://github.com/web-flow.gpg | gpg --import
  gpg --edit-key noreply@github.com trust
  ```

The commit's details show the result as soon as git can verify it.
