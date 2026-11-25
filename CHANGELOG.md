# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/2.0.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Settings › Links › Forges** names what a server of your own runs (GitLab, Gitea, Forgejo, GitHub Enterprise, Bitbucket Data Center or Azure DevOps Server), so its remotes can open on the web. It lists the hosts of the repository's remotes that have none, so picking one is a click. A forge is never guessed from a host's name. Remotes on GitHub, GitLab.com, Gitea.com, Codeberg, Bitbucket and Azure DevOps need nothing here. A rule is a line in git config, the repository's or the global one: `[gitrevenge-forge "git.example.com"]` with `type = gitlab`, and `web =` for a forge whose pages aren't at `https://` and the host, such as one reached through an SSH alias. (#918)
- Each remote in the Remotes panel says where it lives and how fresh it is. A chip after its name shows the host with the forge's mark (GitHub, GitLab, Gitea, Forgejo, Codeberg or Bitbucket) when the remote opens on the web, or the app's own cloud for a host it doesn't know, a server for SSH only, and a folder for a path on this machine. At the right is how long ago it was fetched, such as `4m` or `9d`, which turns ochre after a week. A remote that pushes to several places shows a small fan of where. Hover for its fetch and push addresses, when it was fetched, how many branches it has and which local branches track it. **Settings › Appearance** can show the marks in their brands' colours; they're ink by default. (#919)
- The Remotes panel can be grouped three ways, from the button beside its title. **Remote** is a folder for each remote, as before. **Host** puts the remotes under the host they're on, so github.com holds origin and upstream, with the host's mark and how many remotes it holds. **Name** gives each branch name one row, with a chip for each remote that has it, so you can see whether a fix has reached upstream without opening several folders. A chip marked `≠` is on another commit than the first, and hovering lists each copy's commit. The same button sorts by name or by most recently updated. The choice is remembered for each repository, and the filter, folders and keyboard work the same in all three. A **Fetch every remote** button sits beside the gear. (#920)
- A remote has a menu of its own: right-click its folder in the Remotes panel, use its **⋯** button, or press the Menu key on it. It's headed with the remote's name, host and URL. **Open … on github.com** and **Open its branches** take you to its forge; without a web address the item says so and opens **Settings › Links › Forges** with the host filled in, and a remote that's a folder on this computer offers **Show in the file manager** instead. **Copy the URL** copies its address, and the HTTPS or SSH form too where the forge's is known. **Fetch** says when it last fetched. **Push here by default** sets `remote.pushDefault` and shows which remote has it. **Hide …'s branches** takes a whole remote out of the graph and the sidebar, kept in the app's settings and following the remote when it's renamed. **Rename…**, **Change URL…** and **Remove…** ask first, as the Remotes dialog does. Menus can now carry a line of explanation under an item. (#921)
- A remote branch's menu is headed with how far it is from its remote's default branch and which local branch tracks it. Under the host's name, **Open** shows the branch on its forge, **Compare with origin/main** compares it with the remote's default branch (and with the upstream's, for a fork), and **Copy … web link** copies its address. **Check out as a local branch** says it makes a branch tracking it, or switches to yours of the same name. **Fetch just …** fetches that one branch and nothing else. A local branch's menu gets its upstream's pages too, and so does a commit's menu for the branches on it. Remotes without a web address simply leave the pages out. Nothing opens or starts a pull request. (#922)
- The Remotes panel marks branches that could be tidied, in words: **gone** (deleted on the remote, struck through), **merged** (every commit is on the remote's default branch) and **quiet** (no commits for longer than a window you choose: 3 months, 6 months or a year; none is assumed). A line at the foot of each remote says how many, and opens **Tidy**, as does **Tidy …** in the remote's menu. Tidy lists them by kind, with a line on each saying what deleting it does: a gone branch only removes what's left here (and starts ticked when nothing on it would be lost), while merged and quiet branches are deleted on the remote for everyone, so they're never ticked for you. A quiet branch says how many of its commits no other branch holds. The button names the count, and the result lists every branch with DONE or FAILED. (#924)
- Editing a remote in the Remotes dialog shows how it's fetched, each row naming the git config key it writes: **Include in Fetch all** (`skipFetchAll`), **Drop branches … has deleted** (`prune`), **Tags** with their commits, every tag or none (`tagOpt`), and **Branches**, all of them or only those you list, such as `main, release/*` (`fetch`). Refspecs Git Revenge didn't write are listed as they are and kept. Nothing changes until Save, and going back to the defaults leaves nothing extra in your config. (#925)
- A remote can push to several places. Editing it in the Remotes dialog lists where pushing to it sends, starting with its own URL; **Also push to…** adds another remote's URL or any URL, and each extra can be taken off again. It's git's own `remote.<name>.pushurl`, so a push from a terminal goes to the same places. A push URL that's another remote's URL shows "follows mirror": when Git Revenge changes that remote's URL, the push URL moves with it, and removing that remote asks whether the others should stop pushing there. Taking the last extra off leaves no `pushurl` behind. After a push to several places, a message names every one: pushed, already up to date, refused (with git's reason), failed, or not tried, since git stops at the first place it can't reach. **Push to the rest** or **Retry** carries on from there. (#926)
- **Settings › Pushing** says where pushes go, for this repository or all of yours, each row with its git key and written only when you change it: **Push goes to** (`remote.pushDefault`), so you can fetch from upstream and always push to your fork; **On the remote, the branch is named** (`push.default`); **A branch's first push** (`push.autoSetupRemote`); **Send tags with the commits they mark** (`push.followTags`); and **After a push, fetch the other places it went**, an app setting, off by default. An untouched row says what's used now, from another config or Git Revenge's own default. **Always push … to ▸** in a branch's menu sends one branch elsewhere (`branch.<name>.pushRemote`), and its row shows where. Push now follows these the way `git push` does, and its tooltip, and a branch's **Push … to** item, say where it'll go. Where git config says nothing, Push works as before: a first push publishes to origin (or the only remote) and tracks it, and a branch pushes to the branch it tracks whatever it's named. With `push.autoSetupRemote = false`, a first push asks which remote. (#927)
- Diffs mark the words that changed inside a line, in both the unified and split views, rather than only colouring the whole line. A block of lines that moved shows once, in teal, with "moved to line 9" where it left and "moved from line 3" where it arrived, instead of a deletion and an addition. A row above the diff counts the lines added and removed and the moves, and **F7** and **Shift+F7** step through the changes. (#944)
- The split view has a **ribbon**: its sides aren't padded to line up, each reads as a plain file, the two scroll together, and a curved band in the gap joins each change's old lines to its new ones: rose to green for a change, green or rose for lines only added or removed, and teal for a move, drawn over the others. The change being read is outlined. **Settings › Diffs** turns it off, which pads the sides as before. (#944)
- **Settings › Diffs** picks the line diff: Myers, Minimal, Patience or Histogram, or by default whatever the repository's `diff.algorithm` says, as git does. It can also turn off finding moved code. (#944)
- Diffs read the code in Go, JavaScript, TypeScript, Python, C#, Java, CSS, SCSS, JSON, Rust, Vue, HTML, Lua and XML. Changes are marked token by token. Formatting is told apart and marked with a dotted underline and a note, such as "same value" for `0x30` and `48`: whitespace, line breaks, how a number or string is written, and optional commas, semicolons and parentheses. A name changed the same way in several places is a rename, in ochre, and the row above the diff says "rename: out → readings, 4 places". A chip says **Read as Go**. The language comes from a file's `linguist-language` or `diff` attribute, or its name. (#945)
- **Hide formatting** in a diff takes formatting-only changes out of it, and says how many it hid. Where you stage, nothing is hidden, and formatting stands in hunks of its own, tagged **formatting only**, to stage or leave on its own. File lists mark files whose changes are only formatting. (#945)
- Diffs read the code in more languages: YAML, shell (sh, bash, zsh, ksh), PowerShell, Markdown, C, C++, Objective-C, PHP, Ruby, Kotlin, Swift, Dart, Scala, Perl, R, Elixir, Haskell, Zig, Groovy, SQL, TOML, INI, Dockerfile, Makefile, CMake, HCL, Nix, Protocol Buffers, GraphQL, LESS, Svelte, `go.mod` and git config files. A script with no extension is known by its `#!` line. In Markdown, rewrapped paragraphs and realigned tables are formatting, and code blocks are read in their language. Regular expressions in JavaScript and TypeScript are read token by token. (#965)
- **Code** and **Lines** in a diff switch between reading the code and comparing lines as git does. **Settings › Diffs** sets which a diff starts with, and whether formatting starts hidden. (#945)
- A file that takes longer than 150 ms to read as code shows its line diff at once, and switches to the code when a longer read finishes, within 3 seconds; **Read it as code anyway** gives it up to a minute. A file over 1 MB, or mostly unparsable, keeps the line diff, with a line saying why. (#945)
- The conflict tool reads the code. Where both sides changed different parts of a conflicted place, such as one adding an argument at the start of a line and the other at its end, the place starts filled in with both, marked **Suggested** with a dashed edge, and says what each side did. Each part is washed teal or plum by the side it came from, and the panes above mark what each side changed against the original. A suggestion is never kept for you: it counts as undecided until you keep it (**Keep the suggestion**, <kbd>3</kbd>, or **Keep all N suggestions**) or change it, and **Mark resolved** counts the suggestions to keep and the places undecided separately. A place both sides changed the same way offers **Write it myself**, which starts from both sides. A chip beside the file name says whether it was **Read as Go** or by lines, and why. (#946)
- Rebase and merge stops count what the conflict tool will find, such as "2 suggestions · 1 undecided", and the files a merge driver set in `.gitattributes` settled on its own ("1 settled by mergiraf"). The conflict tool names the driver a file went to first, with the command from your git config, and links to how to set one up when there's none. The merge and rebase previews say how many of a file's conflicted places look combinable. (#946)
- Moves in full (#947, #971). A block of code moved and edited on the way is still a move, "moved from line 3, changed", and **Compare with the original** opens it against the lines it came from so the edits show as edits. Code wrapped in new structure, such as statements put inside an `if`, shows as "moved into an if statement" rather than rewritten, and arguments that swap places are marked as moved. Moves cycle teal, plum, sepia and ochre in both layouts and the ribbon, and a move's note goes to the move.
- Each rename in a diff has its own colour, and clicking one lights up every place it changed, on both sides. (#947)
- In the conflict tool, imports both sides added in the same spot merge as a set, in nineteen languages, with removals kept and a sorted list kept sorted. (#947)

### Changed

- Diffs are worked out by the app's Go side rather than in the window, and kept for the files you go back to, so a large file's diff no longer holds up the view while it's compared. (#944)
- A line indented again where it stood is no longer called a move to where it already was. (#945)
- The app is about 11 MB larger, and its Windows installer about 5 MB, for the grammars it reads code with. (#945)

- Fetching and pulling still drop the remote-tracking branches whose branch the remote deleted, but now respect your config: `remote.<name>.prune = false` keeps that remote's, and `fetch.prune = false` keeps every remote's. Before, the app always pruned whatever the config said. Only remote-tracking branches go, never a local branch or a tag. When a fetch or pull drops any, a message says which ("Fetched origin. 2 deleted branches dropped: …"), so a branch leaving the Remotes panel is never silent. (#923)
- When you write a smart link, the suggested address for issues now covers Gitea, Forgejo and Codeberg as well as GitHub and GitLab. A self-hosted GitLab is suggested once its host is named under Forges, no longer because "gitlab" is in its name. (#918)

### Fixed

- The conflict tool lost every place's original in a file where git had joined two conflicts a line or two apart into one, as git's default conflict style does: **Three-way** was off, and the file had no suggestions while the operation strip counted some. The tool now works from git's merge with the originals kept whenever the file on disk is still the same merge, so the places come apart again, each with its original. (#974)

## [1.1.0] - 2026-11-22

### Added

- **People**, a tab opened from the Repository panel or the command palette: everyone a repository's history names, as its mailmap reads them, with the names joined to each person and the line that joins them. **Take apart** removes that line, **Text** shows the `.mailmap` with what each line does (what it joins, that it matches nobody, or why git skips it), and **Who is this?** answers for any name or address. An edit shows what it does before it's saved, and saving puts the change in `.mailmap` for you to commit. A repository needs no mailmap: the tab works without one, and the first `.mailmap` is only created after you say yes. (#837, #846, #847, #848)
- People offers likely matches: the same name under another address, or the same address under another name. **Same person…** shows the exact `.mailmap` line before adding it, and writes the shortest one that works, so an old name isn't spelled out when the address is enough. **Different people** is remembered for the repository. Bots, no-reply addresses and names many machines use aren't offered; they fold away under the list, and any two people can still be joined by hand with **Same person as…**. Nothing is joined on its own, and an added line is an unsaved edit until you save. **Take apart** now takes only that one name: when its line joined others too, they stay joined by lines of their own, and the view says so. (#838, #852)
- A card about a commit's author. Click the author's name in a commit's details or on a graph row, press Enter on a commit, or choose **About …** in the row's menu. It shows who they are as the mailmap reads them, what this commit says when that differs and the line that decides it, how many commits they wrote and co-authored, and whether they're a bot. From there you can search the timeline for them, open them in People, or start **Same person as…** with them picked. The name in a commit's details used to search straight away; the search is now the card's first button. (#839, #853)
- The trailers on every commit are read once, in the background, and kept, so that filtering and searching by trailer won't have to read the history each time. The first time you open a big repository the status bar says **Reading trailers** with how far it has got; after that only new commits are read. The index is a file in your cache folder, never in the repository, and deleting it is always safe. (#826, #858)
- **Smart filters**, a new panel in the sidebar, lists what the trailers on your commits offer to filter by. Each key is a folder (Reviewed-by, Closes, Changelog and whatever else your project writes) with its values under it and how many commits have each. Spellings of a key are joined, a person is one person under every name your mailmap joins, and a bot is marked. Click a value to filter the graph to its commits, and again to take it out. Keys that wouldn't make a useful filter, such as one on nearly every commit, are held back at the foot of the panel, with why. Nothing is ever written to a commit. (#828, #862)
- Searching the graph for `Key: value` now works for any trailer your repository's commits write, where it knew only a fixed list of keys. A person is found under the name they're shown as, whichever name the commit used. (#828, #862)
- A smart filter changes what the graph shows. A strip above the graph says how many commits it keeps, with a chip for each value picked and **Clear**. Values of one key are either-or, and values of different keys must all hold. **Only these**, the default, takes the other commits out and strings the rest on one dashed lane; **Highlight** leaves everything and lets the matches keep their ink, with a tick for each on the ruler. The mode is remembered for each repository, and the Horizontal view always highlights. A filter stacks with Focus and hidden branches, and lasts until you clear it or close the tab. (#829)
- The search box suggests as you type: the trailer keys and values that contain the text, each with its count, people by name and address. The text search is always the first row, so **Enter** still moves between matches; arrow down to a value and **Enter** adds it to the filter, **Tab** completes it in the box, and **Backspace** in an empty box takes the last filter value off. Counts are within the filter that's on. (#830)
- Search also understands `has:reviewed-by` for any value of a trailer, a leading `-` for the commits that don't match (`-has:reviewed-by`, `-changelog: none`), and a key by any of its endings, so `closed: #14` finds `Closes` too. (#830)
- Under a smart filter, a day's line says how the filter sits in it: "1 of 3 commits" under **Only these**, and "3 commits · 1 match" under **Highlight**. A commit's **Trailers** tab marks the line the filter picked. The commit you had selected stays in the panel when the filter hides its row, and is selected again when it comes back. Where the way back to the checked-out commit would be, the graph says when that commit isn't in the filter. While trailers are still being read, the ruler's foot is hatched for the history whose matches aren't known yet. (#863)
- Under **Highlight**, the filter strip has **Previous match** and **Next match**: they go to the commit before or after the selected one that the filter keeps, round from either end as search does, through every match however long the history. Enter on the graph still opens the author's card. (#866, #891)
- Pin a trailer key to the graph's rows, and its values show as chips after each commit's subject in the timeline, so you can read `Changelog: fixed` or `Closes #14` down the history without opening each commit. Click a chip to filter the graph to it. The pin is beside each key in the Smart filters panel and in a commit's **Trailers** tab; nothing is pinned until you pin it, and the panel offers a repository's most used label once. A pin is a line in git config, the repository's or the global one: `[gitrevenge-trailer "Changelog"]` with `show = rows`. (#831)
- In the **Compact** density a pinned key has a column of its own, left of Author, with the key said once in the heading and each commit's value under it. Click a value to filter the graph to it, or use the heading's filter button for the selected commit's value. Up to two keys get a column while the message has room to be read; a narrower window gives the columns up and moves nothing else. (#868)
- A repository can say how its trailers are read, with rules in git config beside its smart links: under `[gitrevenge-trailer "Reviewed-by"]`, `also = Rviewed-by` reads a misspelled key as the right one, `kind = people` says what its values are, and `show = panel` or `show = held` says whether the Smart filters panel offers it. A rule changes how the history is read and never a commit, and a repository's rule replaces a global one for the same key. (#870)
- **Trailer care**, from **Settings › Smart filters**, the **Needs care** pill in the Smart filters panel, or the command palette. It asks what the app won't decide for you: a key one letter off a much more used one (**Read as Reviewed-by** or **Keep apart**), and two keys holding the same things, such as Fixes and Closes. Several slips of one key can be answered together, **Keep apart** is remembered, and every join can be undone. It lists what was joined without asking, and every key in a table that sorts and filters, with what its values are and where it shows, each a guess until you change it. An answer is a rule in the repository's git config and never changes a commit. (#832)
- Installing Git Revenge gives you a `gitrevenge` command, which opens the app from a terminal. The Windows installer adds a folder that holds only the command to your own `PATH`, for cmd, PowerShell and Git Bash, and uninstalling takes it off again; the machine's `PATH`, which every account shares, isn't touched. On a Mac, Homebrew links the command; if you installed from the disk image, choose **Install the gitrevenge command** in the command palette. The Linux packages already had it. [Docs](https://gitrevenge.com/docs/guide/install#from-a-terminal). (#824, #835, #895)
- The experimental Map view has an **Authors** mode, beside its branches: a node for each person who committed in the span the rings cover, bigger the more commits and closer the more recently. Zoom to change the span, from a week to ten years. A commit counts for its author and for each co-author, merges for nobody, and names that share an email or a name are one person. **Everyone / People / Bots** chooses who shows, your own node says "that's you", and a card gives what someone wrote in the span and in all, with a way to search the timeline for them. (#810, #819)
- In the Map's Authors mode, double-click a person, or choose **Show their branches** on their card, to open them: a ring for what of theirs has landed in the trunk, and a node for each unmerged branch that holds their commits. Any number of people can be open, each with their own, and pointing at a branch lights up the same branch under everyone else who's open. (#811, #820)
- The Map's Authors mode can open a person **By area** as well as **By branch**: the folders their commits touched, each sized by their commits there. A folder opens into its own folders, as far down as you like, and its card lists their latest commits in it, each leading to its place in the timeline. This is the one that shows who works where in a repository that squash-merges, where everybody's branch is the trunk. (#812, #821)
- A repository can say which authors are bots, by name or email, in Settings under **Bot authors** (`gitrevenge.botAuthor` in git config). Insights lists what they shipped with the dependency updates, where it used to go only by names ending in `[bot]`, and prints the patterns with a report's inputs. (#808, #817)

### Changed

- Insights' **Shipped** reads a commit's ticket references the way Smart filters reads its trailers, so a ticket counted in the panel is the same ticket in Shipped. A trailer names a ticket when its whole value is one (`#12`, `TIDE-12`, an issue's address, or several with commas), whichever key it's under: `Bug: #12` now counts, and a number inside a quoted subject, as in `Fixes: 00d60fd ("… [ver #2]")`, no longer does. A repository's Trailer care rules apply to Shipped too. A tracker's key by itself, such as `TIDE-12`, is read as a ticket in the Smart filters panel as well. (#833)
- Names follow the repository's mailmap everywhere, where only Insights and the Map did. The graph, a commit's details, file history and the commits listed in merge and rebase previews show the name the `.mailmap` gives, which is the name `git log` shows in a terminal. A saved `.mailmap` counts straight away, before it's committed. When a commit was written under another name or address, its details say what the commit has. Searching for an author finds them under any name they committed with, and a search with an `@` in it matches addresses too. A repository with no mailmap looks as it did. (#836, #841)
- The Map's Authors mode reads the same mailmap as the graph, where it read only the `.mailmap` committed on the trunk. A `.mailmap` you've saved and not committed, or a `mailmap.file` of your own, now shows the same people in both, and the Map counts again when the mailmap changes. Insights still reads only the committed one, so its reports can be reproduced. (#842, #844)
- Only the repository on screen is watched for changes. A tab behind it used to refresh every time something changed in its repository, with nobody looking, and on Linux each open tab held a watch for every folder in its tree. Now a tab is brought up to date when you switch to it: its files, branches and operation are read as it opens, and its graph is read again only if a branch, tag or HEAD has moved. The Insights and Ignore rules tabs watch the repositories they're showing. A tab's label can show the branch it was on until you switch to it. (#780)
- Starting with several tabs open is quicker: the tab on screen loads its graph first and the others load behind it one at a time, where they all used to start at once. (#780)
- A very wide history scrolls more smoothly. The lanes that only pass through a row are sent to the graph packed together, which makes a page of an 800-lane history about a sixteenth the size it was. (#776)
- A commit or a working tree with tens of thousands of files stays quick: the file lists draw only the rows in view. (#777)
- Closing a tab stops what was still being read for it, and asking for another file's blame or history stops the one before. (#778)
- Scrolling a long way down a big history no longer keeps every row it passed in memory. (#784)

### Fixed

- In People's Text view, a mailmap line that matches people and leaves them as their commits have them now says "changes nothing", with how many names already read that way. It used to say "matches nobody now", the same as a line about someone who isn't in the history. (#854, #857)
- Fetching all remotes no longer reports the whole fetch as failed when some of them are gone, such as a fork deleted after its work was merged. It says which remotes it couldn't find or reach and that the others were fetched, offers to remove the ones with no repository behind them, and still syncs notes from the remotes that answered. A group's fetch says the same in the repository's row. (#806, #807)
- A commit whose message carries one trailer over thousands of lines, or holds thousands of trailers, is read in a moment. One such commit used to take minutes, in the background, in any repository that had it, and closing its tab or quitting waited for it. (#893)
- Staging or unstaging single lines no longer joins two lines when the file's last line has no line ending. Staging only a line added after it used to write the two as one line that was in neither version. (#773)
- In the Map's Authors mode, a person's card and their **By area** view count a commit that credits them in a trailer under a name the mailmap maps to them, as the map itself did. A trailer at an address they share with somebody else, which the mailmap reads as that other person, no longer counts for them. (#843)
- A `.mailmap` that's a symbolic link is read as no mailmap, as git reads it, by People, an author's card, Smart filters and the Map, and People won't save through it, since that would change a file outside the repository. A link to a device such as `/dev/zero` used to make the app read until it ran out of memory. A `mailmap.file` that names something other than a file is read as missing too. (#892)
- A tab's label no longer lands on the tab next to it when another tab closes at the same moment. (#774)
- Leaving a repository's tab for one that isn't a repository, while the repository was still being read, no longer throws errors in the console. (#849)
- A bare repository says it isn't supported, where with `safe.bareRepository=explicit` it showed git's own error. (#775)
- Insights says when its fetch failed, where the error used to be cleared before it showed, and no longer shows one scope's releases or thresholds, or loses its results, when the scope is changed while it's still reading the last one. (#782)
- The Ignore rules tab no longer shows a file from the repository it was on before, when that one was slow to read. (#782)
- A commit's trailers are read when another line sits among them, as git reads them: a bracketed note between the sign-offs, as the Linux kernel writes, a `(cherry picked from commit …)` line, comment lines, or a `Conflicts:` list under them. Such a commit used to show no trailers at all, and wasn't found by a trailer search. The other lines stay in the message. (#860)

### Security

- Built with Go 1.27.4, which fixes GO-2026-6859 in Go's HTTP server. Git Revenge runs no HTTP server of its own, so it wasn't exposed, but the release no longer carries the flaw. (#894)

## [1.0.1] - 2026-11-18

### Added

- Every file in a release now comes with a build provenance attestation: a signed statement that the project's release workflow built that exact file from the release's tag. `gh attestation verify <file> --repo Git-Revenge/GitRevenge` checks it. The sha256 on the release page says a file hasn't changed since it was published; this says where it was built. [How to verify a download](https://github.com/Git-Revenge/GitRevenge/blob/main/SECURITY.md#verifying-what-you-download). (#761)

### Security

- Opening a repository no longer runs programs it names before you've agreed to them. Git takes orders from a repository's own `.git/config` and hooks, and a repository that arrives with its `.git` folder already made (an archive, a shared drive) has whatever its maker put there: with `core.fsmonitor` set to a command, opening it ran that command at once. Now, the first time you open a repository the app didn't clone or create, it reads the config and lists the hooks without running anything there. If any would run a program, pull in other settings or send pushes elsewhere, it shows each one with what it does and asks; **Don't open** leaves it closed with nothing run. Repositories with none open as before, Git LFS's own filter and hooks aren't asked about, and repositories you'd already opened are trusted as they stand. [Docs](https://gitrevenge.com/docs/guide/repositories#a-repository-that-runs-programs-of-its-own). (GHSA-r6gg-5grh-qv76)
- A branch, tag or remote whose name starts with a dash is no longer handed to git where git would read it as an option. A repository's owner can give its default branch such a name, and a clone checks that branch out: rebasing onto a branch named `--exec=<command>` ran the command, and other names made git write its output to a file. Every action that takes a name now refuses one that starts with a dash. Such a branch still shows, and can be renamed from the command line. (GHSA-qvcv-jp5p-2vw5)
- Two buttons no longer open an address the app didn't check. **See the rules**, on a refused push, is only offered when the link the server printed leads to one of the repository's own remotes; any other link stays as text in the details. **Download**, on the update banner, always opens the release's page on the project's GitHub releases, made from the version number. (GHSA-m5wc-fwpc-79qr)
- A credential prompt now says where it came from. The question in the dialog is written by git, or by the server, which can word it as it likes, so the dialog adds a line of its own: which fetch, pull, push or clone is asking, and in which repository. It warns when the question names a server that isn't one of the repository's remotes. The **Trust and connect** button only shows for SSH's own host-key question, not for any prompt that borrows its words. Behind it, each operation's prompts are answered only while that operation runs. (GHSA-w5mm-xpwv-w4hw)
- A second line of defence around what a repository can show. The app's own page now has a Content-Security-Policy: it runs only the app's own script files, with no inline script and no `eval`, and it can fetch nothing from the network. Nothing from a repository was getting in as script before; this is what stops it if a renderer or a library ever gets that wrong. Behind the page, the Go side now checks what it's sent: the Git Config tab only reads, opens and restores config files the app itself found, importing links only reads a file picked in the file dialog, and an interactive rebase only takes whole commit ids. (GHSA-xv2q-84hh-gg5r)
- A link in a repository's document can no longer take the app's window anywhere. A middle click on one used to open its address in a window of the app's own, and a link could be dragged and dropped onto the window. Now a middle click opens a web address in your browser, as a click does, and does nothing for any other; links and pictures can't be dragged, and nothing dropped on the window is followed. A link that isn't a web address (a `file:` or `javascript:` one, say) no longer carries its address at all, only its words. (GHSA-8crh-7f69-q64m)
- Files are no longer read, written or deleted outside the repository through a symbolic link in the working tree. A file that is a link shows as where it points, which is what git stores for it, and discarding lines in one is refused; staging it stages the link. Nothing under a linked folder that leads out of the repository is touched. A `.gitignore` or `.gitattributes` that is a link is left alone, as git itself doesn't read one. (GHSA-3r48-xh64-538c)

## [1.0.0] - 2026-11-16

### Added

- **Map**, an experimental view with no commits in it: a node for each branch and tag, held around the trunk. A node's size is its commits (for the trunk all of them, for a branch the ones the trunk doesn't have, for a tag the ones since the tag before it) and its distance is how long since it last moved, with rings for this week and last. Zoom changes what the rings mean: all the way in, the map is one week with a ring for each day; out, the rings become weeks, then months, then years (the wheel, **+** and **-**, or the buttons in the corner). A branch with nothing of its own is a hollow ring, one that's only on the remote has a dashed outline, a tag hangs off the branch that holds it, and tags older than the newest six fold into one node. Click a node for its card, which opens the timeline on it or focused on its branch; right-click a branch or tag for its menu. Switch it on under **Settings › Experimental views**. The working tree is a dashed mark against the checked-out branch that says how many files have changed, a branch checked out in another worktree wears the worktree mark, and a stacked branch hangs off the branch under it. A busy repository stays readable: remote-only branches that share a path (`feature/`, `bugfix/`) are one node that says how many it stands for and opens to show them, a switch shows all branches, only local ones or only remote ones, what's older than the last ring is counted on the edge ("43 older") and a click zooms out to it, zoom-out stops at the level that holds the oldest branch, the map opens at the level that fits most of it and remembers the level for each repository, and a name shows only where it has room, local branches first. Drag a node to put it where you want it: it stays there for that repository until you double-click it, or its card lets it go. [Docs](https://gitrevenge.com/docs/guide/experimental-views#map). (#652, #661, #662, #672, #683, #684, #696)
- **Horizontal**, an experimental view of the graph with time running across: a column for each commit and a row for each lane, newest on the left. A pinned column on the left names each lane by the branch that started it, even after its tip has scrolled away, and a lane a merge brought in by what the merge says it merged; tags have a row of their own, and lanes scroll up and down when there are more than fit. Subjects lean above their columns, a band across the top names each day, and the selected commit's subject, author and time show in full under the lanes. Left and Right move the selection and the wheel scrolls sideways. It's off until you switch it on under **Settings › Experimental views**; then it's in the graph's view menu. The graph still opens in Timeline. The working tree is the first column, before the newest commit, and the scrollbar is marked as the timeline's ruler is. Bot and Docs lanes fold as they do in the timeline: a lane's commits of a day are one column that says how many, and a click opens it. Drag one commit onto another for the same choices the timeline gives. [Docs](https://gitrevenge.com/docs/guide/experimental-views#horizontal). (#651, #659, #660, #663, #671, #673, #674)
- Audio and video files play in the rendered file view, with the length and, for a video, the frame size. MP3, WAV, Ogg, Opus, FLAC, M4A and AAC sound; MP4, WebM, Ogg video and MOV pictures, as far as the system's own player can decode them; a file it can't is said so. A file over 2 MB shows straight away instead of asking first, up to the 32 MB preview limit. [Docs](https://gitrevenge.com/docs/guide/history#rendered-previews). (#162)
- HTML files show as their page in the rendered file view, in a frame that runs no scripts, submits no forms, opens nothing and loads nothing from the network. Pictures in the repository show; ones on the web are left out, and links keep their words with their address in the tooltip. A line above the page says what was held back. [Docs](https://gitrevenge.com/docs/guide/history#rendered-previews). (#160)
- Jupyter notebooks (`.ipynb`) render as their cells: Markdown with math and attached pictures, code with its `In [n]` count, and the outputs the notebook stored. Plots show as pictures, a pandas DataFrame as a table, and an error as its message and traceback. Nothing is run: a script or widget output is named instead of shown, and HTML that isn't a table shows as its stored text. [Docs](https://gitrevenge.com/docs/guide/history#rendered-previews). (#158)
- AsciiDoc (`.adoc`) and reStructuredText (`.rst`) files render as documents, the way Markdown does: headings, emphasis, lists, links, images from the repository, code blocks with their language, admonitions such as notes and warnings, tables, math, AsciiDoc attributes and rST named links and substitutions. A directive or macro it doesn't know is named, not shown, and nothing in either becomes HTML. [Docs](https://gitrevenge.com/docs/guide/history#rendered-previews). (#159)
- GeoJSON files (`.geojson`) draw as their shapes: areas, lines and points over lines of latitude and longitude, in the file's own colours where it gives plain ones. Zoom with the wheel or **+** and **-**, drag to pan, and **Fit** to see it all again. A list beside the map picks a feature and shows its properties. There's no map underneath: nothing is fetched from a tile server to draw it. [Docs](https://gitrevenge.com/docs/guide/history#rendered-previews). (#161)
- A warning when names differ only by letter case, which Windows and macOS can't keep apart. The new-branch and rename dialogs say so as you type when a name, or a folder in it, differs from an existing local or remote branch's only by case (`Foo` beside `foo`, `Feature/y` beside `feature/x`); you can still go ahead. Where the filesystem tells case apart, as on Linux, a new file whose path differs from a tracked one's only by case gets a **case twin** mark, and a note above the changes says a checkout on Windows or macOS will break. (#726, #727)

### Changed

- The Windows installer, and the app and uninstaller in it, are signed, so Windows names the publisher instead of an unknown one. SmartScreen may still warn about the first releases while the certificate builds up a reputation. (#710)
- On Windows, Git Revenge now installs to `Program Files\Git Revenge`, not `Program Files\Ris Adams\Git Revenge`. Installing 1.0 over an earlier version removes the earlier install, so Apps & features lists one Git Revenge; your settings stay as they are. On macOS the app's bundle identifier is now `com.gitrevenge.gitrevenge`, and the first start carries the window's layout (panel sizes and the like) over from the old one. (#711)
- Three touches to the graph's rows. The selected commit's wash thins away across the row, where it was a flat block. A lane line that leaves a commit at full strength now fades into the quiet line below it, where it stopped at a flat edge. And each row has a hairline under it in its lane's colour, from under the commit's node out across the message, thinning to nothing. [Docs](https://gitrevenge.com/docs/guide/graph#marks-on-a-commit). (#670)
- The graph's two-icon switch between Timeline and Compact is now a menu: one button in the bar shows the current view and opens the list. It takes less room, and it's where experimental views will go. Swapping the two takes one more click than before; the command palette's command still does it in one. [Docs](https://gitrevenge.com/docs/guide/graph#views). (#650)
- Rendered previews use newer libraries: Mermaid 12.3.0 for diagrams, PDF.js 6.5.438 for PDFs and marked 18.1.3 for Markdown. (#643)
- The documentation site no longer loads its fonts from Google, so reading the docs sends nothing to a third party. (#645, #646)

### Fixed

- The clone dialog starts in your home folder when there's no clone folder in Settings and no recent repository. It started empty. (#724)
- Repositories made the way Git 3.0 will make them by default, with reftable refs and SHA-256 commit IDs (`git init --ref-format=reftable --object-format=sha256` today, with git 2.45 or later), work like any other. Commits, checkouts and new branches made outside the app now show up straight away; a cherry-pick, revert or rebase in progress shows in the strip under the tabs, and the conflict tool says where each side came from; and 64-character commit IDs are shortened and read like 40-character ones. (#732, #733, #734, #740, #741, #742)
- A commit's notes are rendered as markdown, as its message is, and link the same way: web addresses and your smart links (a ticket key such as `ABC-1234`) are links in a note, with the same right-click menu. A single line break in a note stays a line break. The message's **Raw** switch shows notes as written too. They were plain text. [Docs](https://gitrevenge.com/docs/guide/history#notes). (#681)

## [0.11.0] - 2026-11-11

Git tools: everyday git chores without leaving the app.

### Added

- A ruler takes the scrollbar's place at the graph's right edge. Its box shows what's in view and can be dragged, with the month beside it as it moves; a press anywhere on it goes there. Ticks mark the checked-out commit, branches, tags, commits with notes and search matches, a faint line marks each month, and a hatched foot is the part of a long history not read yet. Point at a tick to see what's there. (#627, #638)
- A button at the foot of the graph goes back to the checked-out commit whenever that commit is out of view. It names the branch (or the short SHA when detached), points the way, and selects nothing; **Alt+Home** does the same. After a fetch it counts what's new on the upstream. (#619, #633)
- A commit with a git note wears a folded-page mark in the graph, beside its signature seal. Hover the mark to read the note, or click it to open the commit with its notes in view. Notes in every notes ref count. Type `has:note` in the graph's search to step through the commits that have one. (#620, #631)
- An **About** dialog with the version, a link to gitrevenge.com, how to sponsor the project, and credits for Go, Svelte and Wails. Click the version in the status bar, or the heart beside it, or choose **About Git Revenge** in the command palette. (#613)
- Right-click a changed file or folder to ignore it: the file, every file with its extension, or only on this machine. **Ignore…** picks the pattern and the file it goes in, previews what it hides, and offers to stop tracking matching tracked files. Every rule added can be undone. (#521, #524)
- An **Ignored** toggle lists ignored files with the rule behind each, and right-clicking one makes an exception or removes the rule. (#521, #524)
- An **Ignore rules** tab to read and edit every ignore file that applies: each `.gitignore`, `.git/info/exclude` and your global excludes file. Each rule says what it ignores now, and a checker says which rule decides any path. Open it from the sidebar's Repository panel or the command palette. (#522, #527, #540)
- Smart links: patterns you set in **Settings › Links** turn ticket keys such as `ABC-1234` in commit messages, trailers and branch names into links that open in your browser. Presets cover Jira, numbered issues, GitHub, GitLab, Linear and Azure Boards. Rules live in git config, in this repository's `.git/config` or your global config. A branch's right-click menu opens its ticket first. Git Revenge only makes the link and never contacts the site. (#525, #528)
- Bisect: find the commit that broke something. Name a commit where it's broken and one where it worked, then answer **Good**, **Bad** or **Can't tell** as Git Revenge checks out the commits in between. The graph marks your answers, the commit under test and the range still in question, and when the first bad commit is found you can open its changes, start a fix branch or revert it. Start it from a commit's menu or the command palette; a bisect started in a terminal shows up too. (#591, #592)
- Merge and rebase show what they would do before they run. The preview says which files will conflict and in how many places, worked out without touching your files, and lists the commits involved. A merge asks how: fast-forward only, a merge commit, or squash into one, with the ones that can't apply turned off. A rebase says which commit it would stop on, offers to fold `fixup!` commits into the commits they name or to open the rebase editor first, and warns when the branch is already pushed. (#594, #595)
- A merge, rebase, cherry-pick or revert in progress has a strip that says what it is and where it stands: both sides by name, the commit a rebase stopped on with a bead per step, and how many of a merge's conflicted files are resolved. **Resolve** opens the first conflicted file, **Continue** stays off until nothing is conflicted and then says what it will do, and **Abort…** says what it undoes before it does it. The graph ticks a rebase's commits as they're written again. When it's over, a notice offers **Undo**, and **Force push…** after rebasing a pushed branch. (#593)

### Changed

- Bot and Docs lanes fold in the graph: each lane's commits of a day take one line, such as "11 Docs commits", where they used to take a row each. Click the line, or press Right on it, to see the commits; Left closes it. The lane's tip, the selected commit and search matches always show. In the **Kinds** menu, **Fold** beside Bot or Docs chooses between one line a day and every commit; the choice is kept in the repository's git config as `gitrevenge.showKind`. Day counts leave folded commits out. (#624, #637)
- Each day in the timeline has a line of its own, with the day's name and a count of its commits (and of the people, once there's more than one). The line stays at the top of the graph while its day scrolls past, says so when more than a week went by before it ("3 weeks earlier"), and says what just arrived after a pull. Compact rows have no day lines. (#623, #636)
- The timeline gives a commit a second line only when it starts a run: the checked-out commit, a merge, the first commit of a day, a change of author, or a commit made two hours or more after the one before. Commits that carry a run on take one line, with the time at the right, so about twice as many fit on screen. Compact rows are unchanged. (#622, #635)
- The graph follows a branch you move. After a pull, checkout, merge, rebase, reset, cherry-pick or revert, the view goes to the branch's new tip, a third of the way down, and each commit that arrived wears a teal bar at the right edge until you select something else. A commit you had selected stays selected; with nothing chosen, the tip is selected. A fetch, or a change made outside the app, leaves the view where it was. (#619, #633)
- The bar above the graph stays on one line at every width. As it narrows, the commit count moves into the search field ("Search 411 commits"), the search becomes an icon that opens the field over the bar, then Undo and Redo lose their words, then Fetch, Pull and Push. In a very narrow panel the density switch goes too; Settings and the command palette still have it. The timeline no longer has column headings; compact rows keep theirs. (#626, #630)
- The graph's branch and tag column fits the labels in view, between 120 and 240 pixels wide, and you can drag its edge to set a width that's kept for the repository; double-click the edge to fit again. A long name is cut in the middle, so `617-watch…cleanup` and `617-watch…ends` stay apart. A commit shows one label and a count of the rest: hover for the list, or press the row's label button, where each label opens its own menu. The line tying a label to its commit and the band of lane colour behind a commit are gone. (#625, #629)
- Errors are short and say what to do. The banner gives one headline, such as "Push refused: main only takes changes through a pull request", one line on where that leaves you, and up to two next steps: **Push to a new branch…**, **See the rules**, **Pull, then push**, **Force push…** or **Try again**. **Details** shows what git said without its progress lines, and **Copy everything** copies all of it. A push refused by a branch rule, being behind the remote, sign-in, SSH key and connection failures are told this way; any other long message shows its first line with the rest behind **Details**. (#614)
- The interactive rebase editor is rebuilt. Commits are listed oldest first, as git applies them. Each has a menu for what happens to it, which says what every choice does and takes a letter key; a commit that folds into another sits indented under it; a reword edits the subject in place; and you can drag commits or move them with Alt and the arrow keys. **Fold it in for me** places `fixup!` and `squash!` commits under the commits they name. A panel shows the branch as it will be afterwards, what a drop throws away, and whether this order would stop on a conflict. The button says the outcome, such as **Rewrite as 3 commits**. (#596)
- The conflict tool is rebuilt around the result. Each conflicted place is a block in it with its own choices (**Ours**, **Theirs**, **Both**, **Start again**), and conflict markers never show and are never saved. The result is an ordinary editor: typing inside a place makes it **Edited**. Above it you can show both sides of the current place, or all three with the **Original**, each labelled with its branch and the commit that last changed it. F7 and Shift+F7 move between places, and **Mark resolved** stays off until every place is decided. A panel lists the files of the operation with their progress and those that merged cleanly, with whole-file choices and **Open in** your own merge tool when `merge.tool` is set. (#597)
- A repository's own settings (branch kinds, hidden branches, clean up on pull, notes syncing, stack parents set by hand and Insights thresholds) are kept in its git config, under `gitrevenge` keys, instead of the app's settings file. Set them in your global git config to make them your defaults for every repository; a repository's own setting wins, and its list replaces the global one. Existing settings move over the first time each repository opens. The app's own settings, such as theme and zoom, stay where they were. (#526)
- Insights › Shipped groups by the references your smart links find, when any link counts for Shipped, and shows them as links. The rules in use are listed in every export. Without such a link, `PROJ-123` and `#123` still group as before. (#525, #528)
- A bare `#123` in a trailer no longer links to the remote's issues on its own. Add a **Numbered (#123)** link in Settings to say where numbers go. (#525, #528)
- The graph's commit count, Kinds, density and search share one bar with Undo, Redo, Fetch, Pull and Push, so the graph starts a bar higher. They step aside while a file or the rebase editor is open. (#559)
- The working tree is the graph's first row, above the newest commit, with its mark on the first lane and its words where the messages start, instead of a strip above the graph. It still says clean, how many files changed, and offers **Review**. The button that hides the right panel moved to the end of the toolbar. (#556)
- Text actions that throw something away (**Discard all…**, **Delete**, **Remove**, **Discard mine and reload**) are drawn in the danger colour, so they no longer look like the safe action beside them. (#543)
- The graph's density switch is a pair of icons, timeline and compact, that shows which is on, instead of a button whose label changed with the state. (#544)
- A group dashboard's rows open the repository on click, as rows do everywhere else; the **Open** link stays for the keyboard. (#552)
- The sidebar rail marks branches that need care, and submodules that need updating, with a rose dot instead of a number, so it no longer looks like the stash count beside it. (#550)
- Every empty list says so the same way: a handwritten note and one plain sentence, from the sidebar and the working tree to the command palette, LFS, remotes and the reflog. Lists still being read show the loading mark instead. (#549)
- Settings › Appearance picks the theme (System, Light, Dark) and the commit graph (Timeline, Compact) with segmented controls, the same shape as the diff's Unified / Split, instead of drop-downs. (#561)
- Switching between the light and dark themes cross-fades the window instead of flipping it. Reduced motion keeps the flip. (#560)
- Compact rows: the Author column shows a ditto mark for an author carrying on from the row above, and the date column is headed **When**, since it holds relative times. (#557)
- In the timeline, the checked-out commit's row carries a small handwritten "you’re here" after its label when the branch column has room. (#562)
- Jumping to a commit (from a branch, a parent link or a search match) glides there when it's within three screens and cuts when it's further, and the commit lands under a wash that fades. Commits that arrive where you're already looking fade up, a few at a time. Reduced motion keeps everything instant. (#558)
- Resting the pointer on a commit in the graph (or selecting one from the keyboard) shows two small actions at the row's right edge: copy its SHA, and a menu with everything right-click offers. (#555)
- The selected commit is marked by a bar in its lane's colour at the row's left edge, over a lighter wash, and the selection, hover and search dimming ease into place instead of snapping. Reduced motion keeps the instant draw. (#553)
- The timeline's second line stops repeating itself: the first commit of each day names the day (**Today**, **Yesterday**, then the date) under a dashed line, commits below it in the same day show the author and time, and a run of commits by the same author shows only the time, faded. (#554)

### Fixed

- Pressing End in a long history shows the last commit in the commit panel once it has loaded; the panel used to stay empty. (#621, #634)
- Tab from the graph reaches the selected row's actions; the card used to go as the focus left the graph. (#625, #629)
- A link in a commit's subject is rose like the links in its message and trailers, not the browser's blue. (#582)
- An ignored file's menu opens on right-click (or the Menu key) only; a plain click no longer opens it. (#583)
- Opening a repository no longer adds Git LFS's hooks to it. They made every commit, checkout and merge run git-lfs, so a rebase in the app took seconds per commit even in repositories that don't use LFS. Repositories that don't track anything in LFS no longer run git-lfs at all on refresh. Hooks added before stay: in a repository that doesn't use LFS, `git lfs uninstall --local` removes them. (#604, #612)
- On Linux, the first worktree added from a terminal could go unnoticed until something else changed, so its branch didn't say it was checked out there. (#608, #611)

## [0.10.0] - 2026-11-06

Linux becomes a first-class platform, with `.deb` and `.rpm` packages.

### Added

- Linux support for Ubuntu 24.04 and later, Debian 13, RHEL 9 and RHEL 10 (with EPEL), with `.deb` and `.rpm` packages on each release.
- A note when live refresh is off because the system's inotify watch limit is reached.
- A **Kinds** filter in the graph, replacing the Docs and Bots toggles, so any branch kind can be hidden.
- Focus the graph on one branch, or on a whole stack.
- Push or pull a single branch from its right-click menu.
- `.mdx` files preview as Markdown.

### Fixed

- A link in a commit message opened twice per click.
- Branches checked out in other worktrees can be shown, opened and deleted.
- Removing a worktree that's in use finishes the job, and the main tab keeps watching for changes.
- A worktree's tab notices ref changes made elsewhere.
- The graph flickered near 640px wide. The Working tree row now stays pinned left.

## [0.9.1] - 2026-11-04

A round of fixes and commit-panel work from daily use.

### Added

- Pull a branch that isn't checked out. (#408)
- The graph stays readable with many branches: a commit's labels are ranked so the important ones show, a local branch folds with the remote it tracks, the same branch on several remotes becomes one label with a count, and hovering expands every ref. (#412, #420–#423)
- A commit's kind is chosen by priority when several branches share it. (#424)
- The right panel can be resized, and hidden with a shortcut. (#443, #460)
- A commit opens in tabs, with its files always a click away, and its trailers grouped by purpose. (#449, #450)
- Uncommitted changes show as one list of files to tick. The commit button says what happens next, and undo is offered right after committing. (#451, #452, #458)
- Warnings for changes that come from file names differing only by case, and for committing on a detached HEAD. (#453, #454)
- A quiet row above the graph replaces the changes banner. (#455)
- Filter a panel's files, browse them as a tree, and move through them from the keyboard. (#456, #457)
- A trailer helper in the commit form. (#459)
- A stack of three or more branches shows as one group. Manage a stack from its right-click menu: rebase, push and update it as a whole. (#470, #476, #480)
- Delete every merged branch from Needs care in one click. (#472)
- A warning when undo or a force push would leave history behind that others already have. (#479)

### Fixed

- The confirm buttons' labels were hard to read. (#407)
- The row wash overpowered the commit graph. (#431)
- The signature mark touched the date in compact rows. (#442)
- Deleting a squash-merged branch asked to force it. (#445)
- A branch showed twice when its labels expanded. (#447)
- The commit form's options ran onto each other's lines. (#467)
- Stacks merged by squash didn't read as merged, said "parent merged" instead of "merged", and stopped reading as merged once Trunk moved on. (#468, #469, #478)
- Deciding what's safe to delete now reads git, not the graph.

## [0.9.0] - 2026-11-02

A new sidebar, and fixes from using the app every day.

### Added

- A sidebar with an icon rail and panels, a branch tree, the checked-out branch first, and sorting by recent updates. It works from the keyboard and with screen readers. (#334, #336–#338, #342)
- A Repository panel in the sidebar. (#345)
- A Feature branch kind, with a sprout icon. (#347)
- Re-order groups, and their repositories, on the start page. (#349)
- Each ref on a commit gets its own submenu. (#369)
- One loading state across the app. (#364)

### Changed

- Heavy views load on demand, and typing no longer rebuilds ref commands on every key press. (#370, #371)
- Git is asked about upstreams only for branches that have one. (#372)

### Fixed

- The commit menu didn't open on commits with several branches. (#333)
- The graph couldn't scroll back to the very top. (#341)
- Set upstream didn't take a typed branch name. (#348)
- The drop marker drifted when the start page scrolled mid-drag. (#363)
- Some text was smaller than 12px. (#366)
- Failures could look like empty data. (#367)
- Escape and Tab stopped working after a dialog button lost focus. (#375)
- Worktrees showed as clean, or as none, before they had loaded. (#376)
- Repositories under the home folder missed config change events, and a config file was reported before it had finished changing. (#378, #380)
- Trunk lost its kind when another kinded tip sat on its line. (#393)
- The end of a load could undo the move to HEAD. (#394)
- A closed diff reloaded. (#395)
- The rebase editor warned about dropped commits while its plan was still loading. (#396)
- The Repository panel waited for every section before showing any. (#397)
- File counts were claimed before status had loaded. (#398)
- The search box crowded the graph header, labels were clipped past reading, long menu labels were cut off, and blame dates and the Insights scope were clipped. (#399–#402)

## [0.8.0] - 2026-10-31

UI touch, and branch care: kinds, cleanup and stacks.

### Added

- One button family in three sizes, motion tokens, and subtle motion across the app. (#272, #273, #290–#292)
- Branch kinds (Trunk, Release, Environment, Docs, Bot), with per-repository patterns, icons, chips and sidebar sections. (#294, #301)
- Hide branches. (#295)
- Clean up merged branches on pull. (#299)
- Sync notes on fetch, pull and push. (#300)
- Trunk lanes are pinned in the graph. Docs lanes are muted, with toggles and kind icons in the timeline. (#302, #303)
- Confirmation before force pushes and deletes of special branches. (#309)
- Branch kinds in Insights. (#312)
- Stacked branches are detected and shown as a stack. A merged stack parent is flagged, with a one-click restack. (#315, #316)

### Fixed

- Parallel histories didn't interleave by date in the graph. (#288)
- The commit message toggle slid under long subjects. (#287)

## [0.7.0] - 2026-10-30

Insights, rich previews, and a Git Config editor.

### Added

- Git trailers on the selected commit, and search or filter commits by trailer. (#189, #193)
- Markdown in commit messages, with LaTeX math and Mermaid diagrams. (#191, #206, #210)
- A file view mode in the diff viewer. (#194)
- Rendered previews for Markdown, images, CSV and TSV tables, JSON, YAML and TOML trees, and PDFs. (#196–#198, #201, #211)
- Show and edit git notes on a commit. (#203)
- Signature verification badges. (#202)
- A reflog viewer. (#204)
- A settings panel and custom keybindings. (#207, #209)
- Worktree support. (#205)
- A documentation website with walkthroughs, a tour and generated screenshots. (#217, #222)
- A **Git Config** tab for global, local and included config files: a key/value table, a raw text editor, an alias editor, and backups with restore. Saves are validated, and repositories refresh when global config changes. (#231–#233, #235–#239)
- An **Insights** tab: Overview, Shipped, Needs attention and Hotspots for one repository or a group, computed only from repository state, with Copy summary and CSV export. (#255–#261)

### Fixed

- The graph's nodes slid out of view when the history was wider than its column. (#200)
- WebView2's own context menu showed where the app has none. (#214)
- Closing a repository didn't drop its undo and redo history, and the Undo label went stale. (#242, #243)

## [0.6.0] - 2026-10-29

Polish: the Ink and Agency look, a command palette and keyboard shortcuts.

### Added

- Ink and Agency design throughout: tokens, bundled fonts, light and dark themes, the app chrome, menus, dialogs, diff, blame and code views, and an icon system. (#127, #128, #132, #134, #178)
- The merge flag app icon. (#131)
- A timeline commit graph, with a compact mode. (#133)
- A command palette and keyboard shortcuts. (#136, #137)
- A word wrap toggle for diff and blame views. (#179)
- A check for newer versions on GitHub Releases. (#180)
- The commit graph loads progressively, ahead of the viewport. (#182)

### Changed

- The first screen and memory use meet the Linux kernel repository targets. (#187)

### Fixed

- The context menu highlight was hard to see. (#170)
- git-lfs wasn't found when the app was launched from Finder, and its absence wasn't explained. (#175)
- Closing the window on macOS didn't quit the app. (#177)

## [0.5.0] - 2026-10-29

Groups of repositories, LFS and submodules, and file history.

### Added

- Project groups, with a dashboard and batch fetch and pull. (#118)
- Git LFS (patterns, files and locks) and submodules. (#119)
- File history and blame. (#120)
- Clone and init repositories. (#121)

### Fixed

- Closing a repository didn't wait for its background graph load. (#117)

## [0.4.0] - 2026-10-29

Rewriting history, safely.

### Added

- Reset and revert. (#112)
- Undo and redo. (#113)
- A three-pane merge conflict tool. (#114)
- An interactive rebase editor. (#115)

## [0.3.0] - 2026-10-29

Branches and remotes.

### Added

- Branches, upstreams and tags. (#108)
- Fetch, pull and push, remotes, and in-app credential prompts. (#109)
- Merge, rebase and cherry-pick, including by drag and drop. (#110)

## [0.2.0] - 2026-10-28

The working tree.

### Added

- Working tree status, file staging and a commit composer. (#104)
- A diff viewer with hunk and line staging. (#105)
- Stash save, apply, pop and drop, with stashes in the graph. (#106)

## [0.1.0] - 2026-10-28

The foundation: open a repository and read its history fast.

### Added

- Open repositories in tabs, restored on launch, watched for changes made elsewhere.
- Detection of an in-progress merge, rebase, cherry-pick, revert or bisect.
- A canvas commit graph with incremental lane layout and virtualized rows, ref labels, and a read-only sidebar.
- A commit detail panel with changed files and file diffs.
- Search commits by message, author or SHA.
- Works with the `git` you have installed, and checks its version.

[Unreleased]: https://github.com/Git-Revenge/GitRevenge/compare/v1.1.0...HEAD
[1.1.0]: https://github.com/Git-Revenge/GitRevenge/compare/v1.0.1...v1.1.0
[1.0.1]: https://github.com/Git-Revenge/GitRevenge/compare/v1.0.0...v1.0.1
[1.0.0]: https://github.com/Git-Revenge/GitRevenge/compare/v0.11.0...v1.0.0
[0.11.0]: https://github.com/Git-Revenge/GitRevenge/compare/v0.10.0...v0.11.0
[0.10.0]: https://github.com/Git-Revenge/GitRevenge/compare/v0.9.1...v0.10.0
[0.9.1]: https://github.com/Git-Revenge/GitRevenge/compare/v0.9.0...v0.9.1
[0.9.0]: https://github.com/Git-Revenge/GitRevenge/compare/v0.8.0...v0.9.0
[0.8.0]: https://github.com/Git-Revenge/GitRevenge/compare/v0.7.0...v0.8.0
[0.7.0]: https://github.com/Git-Revenge/GitRevenge/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/Git-Revenge/GitRevenge/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/Git-Revenge/GitRevenge/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/Git-Revenge/GitRevenge/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/Git-Revenge/GitRevenge/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/Git-Revenge/GitRevenge/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/Git-Revenge/GitRevenge/releases/tag/v0.1.0
