# Cloning a big repository

A clone brings the whole history: every commit, every version of every file, every branch, and then writes every file. For most repositories that's what you want. For a big one it can be most of a day. The Linux kernel, for one, is 1.5 million commits and 6.8 GB, and writing its 97,310 files took 16 to 19 minutes on a Windows machine.

Git can clone less: part of the history, file contents only when something needs them, one branch, some folders. Git Revenge offers each of these when you clone, and every part of the app copes with a clone that has them.

## The clone's options

**Clone a repository**, on the start page or from the command palette, has **Options** under the URL and folder. Folded, a line beside it says what's chosen. Every default is what plain `git clone URL` does, so a clone you don't change is the clone git would make.

![The clone dialog with Options open: the last 1,000 commits of each branch, file contents only when something needs them, and only some folders](../img/clone-options-light.webp#gh-light-mode-only)
![The clone dialog with Options open: the last 1,000 commits of each branch, file contents only when something needs them, and only some folders](../img/clone-options-dark.webp#gh-dark-mode-only)

| Option | Choices | What git does |
|---|---|---|
| **History** | Every commit, the last *N* commits (of each branch), or since a day. | A *shallow* clone: `--depth N` or `--shallow-since`. The history stops at the oldest commit fetched, and more can be fetched later. |
| **File contents** | Download every version now, or only when something needs them. | A *partial* clone: `--filter=blob:none`. Every commit and folder listing now, and a file's contents the first time something reads it. |
| **Branches** | Every branch and tag, or only one, chosen from the server's branches. | `--single-branch --branch`. |
| **Folders** | Every folder, or only the ones you choose next. | A *sparse checkout*: `--no-checkout --sparse`, then `git sparse-checkout set` with the folders. |
| **Submodules** | Clone them too. | `--recurse-submodules`, shallow or partial as the clone is. |

What each costs, measured on the kernel over `file://`, so the machine was also the server, with no network in the way:

| Clone | Size | Time |
|---|---|---|
| Every commit, every version | 6.8 GB | |
| The last commit only | 290 MB | 75 s |
| File contents only when needed | 2.1 GB, then 2.5 GB once checked out | 8.8 minutes, then 16 to 19 minutes writing the files |
| The same, with only `mm/` written | | 22 s to write its 226 files and the top level's |

The partial clone's own options stop at file contents (`blob:none`). Git can also leave out the folder listings (`tree:0`), but then anything that asks which files a commit changed downloads them a commit at a time: one file's history in the practice repository, 1,406 commits, took 175 seconds and 1,371 downloads. On the kernel that would be well over a million.

### Only some folders

Choose **Only the ones I choose, next** and the clone fetches the history and folder listings and writes no file. A second step then lists the repository's folders from its own tree, each with how many files it holds. Tick the ones you want: the line under the list says how many files a checkout of them writes, counting what a sparse checkout always keeps (the files at the top, and those directly in the folders above the ones you chose).

![Step two of the clone: the repository's folders with their file counts, docs/ and public/ ticked, and how many files that writes](../img/clone-folders-light.webp#gh-light-mode-only)
![Step two of the clone: the repository's folders with their file counts, docs/ and public/ ticked, and how many files that writes](../img/clone-folders-dark.webp#gh-dark-mode-only)

**Check out** writes them. **Every folder after all** writes everything. Closing the dialog takes the clone away, since a clone with no files isn't one you'd keep.

## This clone

A clone that holds less than the whole repository says so in the [sidebar](tour.md#the-sidebar)'s Repository panel, whoever made it, Git Revenge or git in a terminal. Each row says what the clone has, and has the way to the rest:

![The Repository panel's This clone section: history since a day, file contents when they're needed, two folders checked out, and only main from origin](../img/this-clone-light.webp#gh-light-mode-only)
![The Repository panel's This clone section: history since a day, file contents when they're needed, two folders checked out, and only main from origin](../img/this-clone-dark.webp#gh-dark-mode-only)

| Row | Says | The way to the rest |
|---|---|---|
| **Shallow** | The history since a day, and how many commits that is. | **Fetch more history**: a year more, 1,000 commits more, or all of it. |
| **Partial** | File contents when they're needed, from which remote. | **Download them all…**: every version of every file in the checked-out branch's history (in a sparse checkout, its folders'), as `git backfill` does. It says first what it would fetch: how big the clone is now, and how many versions it would download, counted while you decide. On a long history the count can take minutes (the kernel's hadn't finished in ten), so after a few seconds it says so, and the download needn't wait for it. |
| **Sparse** | How many folders are checked out, how many of the files that writes, and the folders' names. | **Change folders…** opens the same folder list as the clone's second step, with the current folders ticked. **Every folder** asks first, saying how many files it writes. Patterns that aren't folders show as patterns, and are changed with `git sparse-checkout`. |
| **One branch** | Which branch, from which remote. | **Fetch every branch**: `git remote set-branches origin '*'`, then a fetch. |

Each runs as a fetch does: its progress in the toolbar, with **Cancel**, and the graph and lists catching up when it lands.

## Where a shallow history stops

Git lists a shallow clone's oldest commits as if they had no parents, so a graph would draw them as where the project began. Git Revenge reads the commits they really have from `.git/shallow` and lets their lines run on, dashed, to the end of the graph, into a band that says where the history stops:

![The end of a shallow clone's graph: the oldest commits' lines run on, dashed, into a band that says history stops here, since a day, with Fetch a year more, Fetch 1,000 commits more and Fetch the whole history](../img/shallow-edge-light.webp#gh-light-mode-only)
![The end of a shallow clone's graph: the oldest commits' lines run on, dashed, into a band that says history stops here, since a day, with Fetch a year more, Fetch 1,000 commits more and Fetch the whole history](../img/shallow-edge-dark.webp#gh-dark-mode-only)

The rest of the app says so too:

- **Counts:** the count above the graph says "since" that day, and so do the Map, [Insights](insights.md) and [People](people.md). An author who's been quiet since then may only be out of reach, not gone.
- **Blame:** a line from before the edge says "before" the edge's day, rather than crediting the oldest commit here with it.
- **A file's history** that reaches the edge ends with the same band.
- **A commit at the edge** opens with all its files as its own, as a first commit's do, and says its parent wasn't fetched.
- **A merge or rebase** whose branches part before the edge would be "unrelated histories" to git. Its preview says so and offers more history, and once that's in, it forecasts as usual.

## When a file isn't here yet

In a partial clone, git downloads a file's contents the first time anything reads it: one request for each file, behind whatever asked. That's fine for one file and slow for thousands. Blame on the kernel's `Makefile`, left to git, made 1,888 downloads and hadn't finished after 15 minutes; its versions in one request take seconds. So Git Revenge reads with git's downloads switched off (`GIT_NO_LAZY_FETCH`), finds what's missing, and decides:

1. **One thing you opened downloads by itself.** A diff, a file, a file from Find in files, a commit's line counts, the files a merge or rebase preview compares, and the pictures a rendered document shows. It says what it's downloading, has **Stop**, and nothing else waits for it.

   ![A diff in a partial clone: this version of the file isn't here yet, downloading, with Stop](../img/partial-download-light.webp#gh-light-mode-only)
   ![A diff in a partial clone: this version of the file isn't here yet, downloading, with Stop](../img/partial-download-dark.webp#gh-dark-mode-only)

2. **Many files ask first**, saying how many, and come in one download. Blame reads every version of a file. A file's history that stops at a rename it can't follow offers **Download and follow**. Insights counts commits and files at once, and lines changed when you choose **Download two weeks of changes**. Two weeks of the kernel's line counts left to git made 1,442 downloads and hadn't finished after 15 minutes.
3. **Reads across the history never download.** The Map, People, smart filters, branch care, the [history search](searching-code.md#the-history-search) and Find in files at a commit use the commits and folder listings, which a partial clone has. Where one would need a file, it says what isn't here.

### What leaves your machine

A download is the same request git makes for a partial clone: it asks the clone's own remote, the one it was cloned from, for the file versions it names, and nothing else. It happens only as the rules above say. Nothing is sent anywhere else.

## Clone by clone

What each part of the app does in each kind of clone. Times are on the kernel, with a whole clone's in brackets where it helps.

| Part | Shallow | Partial | Sparse |
|---|---|---|---|
| The graph | Ends at the edge, with the band. | All of it, as quick: the first 2,000 rows in 0.21 s (0.21 s). | All of it. |
| Status, staging, commit | As usual. | As usual: status 0.64 s (0.56 s). | The chosen folders, as git does. |
| A commit's files | Commits past the edge aren't there. | Names at once, with nothing downloaded; its line counts download that commit's files. | Every file, in the folders or not. |
| A diff, a file, a preview | As usual. | Downloads what it shows, with Stop. | Files outside the folders still open, from the commit. |
| Blame | Lines from before the edge say so. | Asks first, then one download. | As partial, for any file. |
| A file's history | Ends at the edge, with the band. | Nothing downloaded; a rename it can't follow offers to. | As usual, in the folders or not. |
| Insights | Says "since" the edge. | Commits and files at once; lines changed when asked. | As usual. |
| The Map, People, smart filters | Say "since" the edge. | As usual: they read commits and folder listings. | As usual. |
| Find in files | The commits that are here. | The working tree; at a commit, says what isn't downloaded. | The working tree's folders; at a commit, every folder. |
| Merges, rebases, previews | Branches that part past the edge wait for more history. | Downloads the files both sides changed, in one go. | As git does: conflicts outside the folders are written to the working tree. |
| Fetch, pull, push | A fetch stays shallow. | A fetch brings commits and listings, as the clone did. | As usual. |
