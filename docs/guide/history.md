# Previews and history

How a diff reads, with the code read token by token, has [a page of its own](diffs.md). This page covers the rest: files shown rendered, a file's history and blame, the reflog, finding the commit that broke something, notes and signatures.

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
| AsciiDoc (`.adoc`, `.asciidoc`, `.asc`) and reStructuredText (`.rst`, `.rest`) | formatted text: titles, sections, lists, links, images, code blocks, quotes, admonitions and tables. Anything else stays as text, and an include names the file it would have brought in |
| Jupyter notebooks (`.ipynb`) | the Markdown cells formatted, and each code cell with the outputs the notebook stored: text, images, errors and tables. Nothing runs |
| HTML (`.html`, `.htm`, `.xhtml`) | the page, in a frame where no script, form or link works. Images from the repository show; anything it would fetch from the web shows as its alt text instead |
| GeoJSON (`.geojson`) | its shapes, drawn over lines of latitude and longitude, with a list of features beside them. Click one for its properties. The wheel zooms and a drag pans. There's no map underneath, since that would mean fetching tiles from someone's server |
| Audio and video (MP3, WAV, Ogg, Opus, FLAC, M4A, AAC, MP4, WebM, MOV and more) | a player, with the length, frame size and file size. It plays only when you press play, and whether a format plays depends on your system |

![A README shown rendered](../img/preview-markdown-light.webp#gh-light-mode-only)
![A README shown rendered](../img/preview-markdown-dark.webp#gh-dark-mode-only)
![A CSV file shown as a table](../img/preview-csv-light.webp#gh-light-mode-only)
![A CSV file shown as a table](../img/preview-csv-dark.webp#gh-dark-mode-only)
![A JSON file shown as a tree you can search](../img/preview-json-light.webp#gh-light-mode-only)
![A JSON file shown as a tree you can search](../img/preview-json-dark.webp#gh-dark-mode-only)
![A Mermaid diagram, drawn](../img/preview-mermaid-light.webp#gh-light-mode-only)
![A Mermaid diagram, drawn](../img/preview-mermaid-dark.webp#gh-dark-mode-only)
![A Jupyter notebook, with its stored outputs](../img/preview-notebook-light.webp#gh-light-mode-only)
![A Jupyter notebook, with its stored outputs](../img/preview-notebook-dark.webp#gh-dark-mode-only)
![A GeoJSON file drawn as its shapes, with a feature's properties](../img/preview-geojson-light.webp#gh-light-mode-only)
![A GeoJSON file drawn as its shapes, with a feature's properties](../img/preview-geojson-dark.webp#gh-dark-mode-only)
![An HTML page in a sandboxed frame](../img/preview-html-light.webp#gh-light-mode-only)
![An HTML page in a sandboxed frame](../img/preview-html-dark.webp#gh-dark-mode-only)

Nothing a preview shows reaches the network. Audio and video of up to 32 MB open straight away, without the "Load anyway" step other large files ask for.

## File history and blame

From a file's diff, **History** lists every commit that changed it, and **Blame** shows who last changed each line and in which commit.

![Blame: each line with the commit and author that last changed it, under the track through the file's versions](../img/blame-light.webp#gh-light-mode-only)
![Blame: each line with the commit and author that last changed it, under the track through the file's versions](../img/blame-dark.webp#gh-dark-mode-only)

### Blame through time

Above the code, a track runs through the file's versions: one mark for each commit that changed it, coloured by its author and taller the more lines it changed, with the years under it. Drag the handle, use the buttons either side or the arrow keys on it, Home and End, or **[** and **]** from anywhere in the view, to read the file as it was at each version. Each step washes what that version changed and brings it into view, and says which version it is ("Version 9 of 24"), with the commit, its author and date, and how many lines it added and took out. Changes not committed yet are the last step.

![The demo's src/alerts/sms.js at version 9 of 24: the line that version changed washed, and the track with the handle partway along](../img/blame-track-light.webp#gh-light-mode-only)
![The demo's src/alerts/sms.js at version 9 of 24: the line that version changed washed, and the track with the handle partway along](../img/blame-track-dark.webp#gh-dark-mode-only)

The text of a version shows as soon as you reach it. Who wrote each line fills in once the handle rests, and a version next to one already read is worked out from it, so stepping through a long history stays quick.

A version reads three ways, from the switch on the track, and the choice is kept for each repository:

- **Who wrote it**: the blame at that version.
- **What changed**: that version against the one before, with what it took out struck through where it was, marked − and +.
- **How old**: each line washed by how many versions back it last changed, from this one to more than ten back, with a key.

**Rewind these lines** gives a block of lines its own history. Select the lines, or use the button that shows at the start of a blame block, and a panel beside the code lists only the versions that changed them, newest first, each with what it did to them. It follows them as they move down the file, as `git log -L` does. **Show the whole file at …** moves the file's track to that version.

**Restore this version**, on the track, writes the file as it was into the working tree, and **Restore these lines**, in a block's history, puts just those lines back where they are now. Either becomes a change to commit or throw away: no commit changes, and the index is left alone. It asks first, saying how many lines it adds and takes out, and if the file has changes you haven't committed, it says so: **Undo**, in the note afterwards, brings back what was there.

The track follows no renames: it stops at the version where the file got its name, which says what it was called before, with **Carry on under the old name**. A version restored from there is written under the file's name today. Blame, like the track, is for text: a version where the file was binary says so.

Besides the blame view, **Blame at this commit** in a commit's files, and **Blame at …** in a file's history, open it at that commit: the track still runs to the newest version, with the handle on that commit's. In a [partial clone](big-repositories.md#when-a-file-isnt-here-yet), the versions the track needs that were never downloaded come in one download, after asking.


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

Git notes attach extra text to a commit without changing it. The details panel shows a selected commit's notes, and lets you add, edit or remove one. A note is shown as Markdown, the way a commit message is: issue numbers, SHAs and links in it are clickable, and a single line break stays a line break.

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
