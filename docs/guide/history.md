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

## Finding the commit that broke something

When something worked once and doesn't now, a bisect finds the commit that changed it. You name one commit where it's broken and one where it worked, and Git Revenge checks out commits in between for you to try, halving the range with each answer.

Right-click the broken commit and choose **Find what broke it, up to here…**, or choose **Find the commit that broke it…** in the command palette to start from the commit you're on. The dialog takes the two ends. It suggests the last tag as the good one, and says how many commits lie between them and about how many steps that is. Your working tree has to be clean enough for git to check out other commits.

While a bisect runs, a strip under the tabs asks about the commit that's checked out. Build it, try the thing that's broken, then answer:

- **Good** (<kbd>G</kbd>) if it works there.
- **Bad** (<kbd>B</kbd>) if it's broken there.
- **Can't tell** (<kbd>S</kbd>) if that commit won't build or can't be tested. Git picks another nearby.

**Undo last** takes back your last answer and checks that commit out again. **Stop…** ends the bisect and returns you to where you started.

The graph shows where things stand. Commits you've answered for carry a tick or a cross, the commit under test has a ring and the note "testing this one", the commits still in question have a teal band, and everything that's been ruled out fades. The panel on the right has the same three answers in words, what the commit changed, and your answers so far.

When the answer is known, the strip says how many steps it took and the graph marks the commit "it broke here". The panel shows what that commit changed and offers to open its changes, start a branch for the fix, or revert it. **Copy the log** copies your answers as text for an issue. **Done** returns you to your branch. If the commits you skipped hide the answer, you're told which commits it could be instead.

![A bisect in progress: the strip asks about the commit under test, and the graph marks the answers so far](../img/bisect-step-light.webp#gh-light-mode-only)
![A bisect in progress: the strip asks about the commit under test, and the graph marks the answers so far](../img/bisect-step-dark.webp#gh-dark-mode-only)

The walkthrough [Find what broke it](../walkthroughs/find-what-broke.md) goes through one from start to finish.

A bisect lives in the repository, not the app: one started with `git bisect` in a terminal shows up here, and one started here carries on in a terminal.

## Notes

Git notes attach extra text to a commit without changing it. The details panel shows a selected commit's notes, and lets you add, edit or remove one.

In the graph, a commit with a note wears a folded-page mark. Hover over the mark to read the note, or click it to open the commit with its notes in view. Notes in every notes ref count. Type `has:note` in the graph's [search](graph.md#searching) to step through the commits that have one.

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
