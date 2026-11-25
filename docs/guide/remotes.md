# Remotes, LFS and submodules

## Fetch, pull and push

The toolbar has **Fetch**, **Pull** and **Push**.

| Action | Keys |
|---|---|
| Fetch all remotes | <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>F</kbd> |
| Pull | <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>L</kbd> |
| Push the current branch | <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>U</kbd> |

**Pull** does whatever your git config says (`pull.rebase`, `pull.ff`). The arrow next to it, and the command palette, offer **fast-forward only**, **merge** and **rebase** for a one-off.

**Fetch** fetches every remote except those [left out of Fetch all](#how-a-remote-is-fetched). Two settings under **This repository** add to fetch and pull: [cleaning up merged branches](branch-care.md#cleaning-up-merged-branches) after a pull, and [syncing notes](branch-care.md#syncing-notes) on fetch, pull and push.

### Branches deleted on the remote

Fetch and pull drop your copies of the branches a remote has deleted, as `git fetch --prune` does. Only remote-tracking branches go, never a local branch or a tag, and a message names them: "Fetched origin. 2 deleted branches dropped: …".

Your git config has the last word. `remote.<name>.prune = false` keeps one remote's deleted branches, and `fetch.prune = false` keeps every remote's. **Drop branches … has deleted**, in [a remote's fetch rules](#how-a-remote-is-fetched), sets the first for you. A branch kept this way is marked **gone** in the Remotes panel until you [tidy](#tidying) it.

### Where a push goes

**Push** sends the checked-out branch where `git push` would. Hover over it to see where, and why: "Push fix/gauge-drift to origin/fix/gauge-drift". When `remote.pushDefault` or the branch's own push remote decides, the button names the remote: **Push → fork**.

- **With nothing set**, a branch pushes to the branch it tracks, whatever that's named. A branch that has never been pushed goes to `origin`, or the only remote, under the same name, and tracks the branch it makes there.
- **`remote.pushDefault`** sends every branch to one remote, so you can fetch from `upstream` and always push to your fork. **Push here by default** in [a remote's menu](#a-remotes-menu) sets it.
- **`branch.<name>.pushRemote`** sends one branch somewhere else. Choose **Always push … to ▸** in the branch's menu; the branch's row in the sidebar then shows where it pushes.
- **`push.autoSetupRemote = false`** makes a first push ask which remote. The button reads **Push…** until the branch has somewhere to go.

**Push** in a local branch's menu goes to the same place, and names it: **Push fix/gauge-drift to fork**. [Settings › Pushing](settings.md#pushing) has these settings, and `push.default` and `push.followTags`, each with its git key.

### Signing in

Git Revenge uses your existing credential helpers and SSH agent, so if `git push` works in a terminal, it works here. When git needs something they can't supply, such as a password or a key's passphrase, the app asks you in a dialog. It never stores what you type; saving credentials is left to your credential helper.

The first time you connect to an SSH host your machine doesn't know yet, the app shows the host key's fingerprint and asks before accepting it.

### When a fetch, pull or push fails

A strip under the tabs says what failed and why in one line, then where that leaves you, such as "Nothing was pushed". Where there's an obvious next step it's a button:

- **Pull, then push** or **Force push…** when the remote has commits you don't have.
- **Push to a new branch…** when the server refuses the branch, for example one that only takes changes through a pull request. It starts a branch at your current commit, switches to it and pushes it; the branch you were on stays as it is. **See the rules** opens the page the server pointed to.
- **Try again** after a sign-in or connection problem.
- **Remove them…** when a fetch of all your remotes found no repository behind some of them, such as a fork that has since been deleted. It names the remotes and asks before removing them, along with the remote branches fetched from them. The remotes that did answer were fetched all the same, and the strip says so.

**Details** shows what git itself said, without its progress lines, and **Copy everything** copies all of git's output for a bug report.

A push to [several places](#pushing-to-several-places) says how each one went instead.

## The Remotes panel

The **Remotes** panel of the sidebar lists your remotes and their branches. Each remote says where it lives and how fresh it is:

- **A chip after its name** shows its host. It carries the forge's mark (GitHub, GitLab, Gitea, Forgejo, Codeberg or Bitbucket) when the remote [opens on the web](#forges-and-web-pages). Otherwise it shows a cloud for a host the app doesn't know, a server for one reached only by SSH, and a folder for a path on this machine. **Forge marks in their brands' colours**, in **Settings › Appearance**, colours the marks; they're ink by default.
- **At the right** is how long ago it was fetched, such as `4m` or `9d`. It turns ochre after a week.
- **A small fan** means it [pushes to several places](#pushing-to-several-places).
- **Hovering** shows its fetch and push addresses, when it was fetched, how many branches it has, and which local branches track it.

![The Remotes panel grouped by host: a backup reached by SSH, fetched 9 days ago; two remotes on GitHub; a mirror on GitLab; and origin, a folder on this machine that also pushes to the mirror and the backup](../img/remotes-by-host-light.webp#gh-light-mode-only)
![The Remotes panel grouped by host: a backup reached by SSH, fetched 9 days ago; two remotes on GitHub; a mirror on GitLab; and origin, a folder on this machine that also pushes to the mirror and the backup](../img/remotes-by-host-dark.webp#gh-dark-mode-only)

### Grouping and sorting

The button beside the panel's title says how the remotes are grouped, and changes it:

- **Group by remote** gives each remote a folder.
- **Group by host** puts remotes under the host they're on, so `github.com` holds `origin` and `upstream`. Each host shows its mark and how many remotes it holds. Remotes that are folders on your computer go under **This machine**.
- **Group by name** gives each branch name one row, with chips for the remotes that have it and how many do, so you can see whether a fix has reached `upstream` without opening several folders. A row shows up to four chips, `origin` first. A chip marked `≠` is on a different commit from the first; hover over the row to see every remote's copy and its commit.

![The Remotes panel grouped by name: main is on five remotes, and backup, gitlab and sarah have it at another commit than origin; feature/offline-cache is on origin and, at another commit, sarah](../img/remotes-by-name-light.webp#gh-light-mode-only)
![The Remotes panel grouped by name: main is on five remotes, and backup, gitlab and sarah have it at another commit than origin; feature/offline-cache is on origin and, at another commit, sarah](../img/remotes-by-name-dark.webp#gh-dark-mode-only)

The same button sorts by name or by most recently updated. The choice is kept for each repository, and the filter, folders and keys work the same whichever you pick.

Beside it, **Fetch every remote** is the toolbar's Fetch, and the gear opens [the Remotes dialog](#managing-remotes).

### A remote's menu

Right-click a remote's folder, use its **⋯** button, or press the Menu key on it. The menu is headed with the remote's name, host and URL.

![The menu of a remote on GitHub: open it or its branches on github.com, copy its URL in either form, fetch it, push here by default, hide its branches, tidy, rename, change the URL or remove it](../img/remote-menu-light.webp#gh-light-mode-only)
![The menu of a remote on GitHub: open it or its branches on github.com, copy its URL in either form, fetch it, push here by default, hide its branches, tidy, rename, change the URL or remove it](../img/remote-menu-dark.webp#gh-dark-mode-only)

- **Open tidewater/tidewater on github.com** and **Open its branches on github.com** open the remote's pages. A remote without a web address has **Open on the web…**, which says why and opens [Settings › Links › Forges](settings.md#forges) with the host filled in. A remote that's a folder on your computer has **Show in the file manager** instead.
- **Copy the URL** copies its address. Where the forge's other form is known, **Copy the HTTPS URL** or **Copy the SSH URL** copies that.
- **Fetch origin** fetches that remote alone, and says when it was last fetched.
- **Push here by default** sets `remote.pushDefault` to this remote, and says which remote has it now. Choosing it again unsets it.
- **Hide origin's branches** takes the whole remote out of the graph and the sidebar, and **Show origin's branches** brings them back. This is kept in the app's settings rather than git config, and follows the remote when it's renamed.
- **Tidy origin…** opens [Tidy](#tidying).
- **Rename origin…**, **Change URL…** and **Remove origin…** ask first, as the Remotes dialog does.

### A remote branch's menu

The menu is headed with how far the branch is from its remote's default branch, such as "2 ahead, 5 behind origin/main", and which local branch tracks it.

- **Open origin/fix/gauge-drift**, under the host's name, shows the branch on its forge. **Compare … with origin/main** compares it with the remote's default branch, and a fork's branch can also be compared with the upstream's. **Copy …'s web link** copies its address.
- **Check out as a local branch** makes a local branch of the same name that tracks it. If you already have one, **Checkout fix/gauge-drift** switches to yours.
- **Fetch just fix/gauge-drift** fetches that one branch, and nothing else, from its remote.

A local branch's menu has its upstream's pages too, and a commit's menu has them for the branches on it. Remotes without a web address leave the pages out. Nothing in the app opens or starts a pull request.

### Tidying

The Remotes panel marks branches that could be tidied, in words:

- **gone**: deleted on the remote, and struck through.
- **merged**: every commit on it is on the remote's default branch.
- **quiet**: no commits for longer than a window you choose: 3 months, 6 months or a year. None is assumed, so nothing is quiet until you choose.

A line at the foot of each remote, such as "3 could be tidied", opens **Tidy**, as does **Tidy origin…** in the remote's menu. Tidy lists the branches by kind, each with a line saying what deleting it does:

- **Gone** branches only lose what's left of them here: your copy of a branch the remote no longer has, or a local branch whose upstream was deleted. One starts ticked when nothing on it would be lost. A local branch with commits that are on no remote says how many, and that deleting it leaves them only in the reflog.
- **Merged** and **quiet** branches are deleted on the remote, for everyone, so they're never ticked for you. Each says when its last commit was made, and by whom, and a quiet branch says how many of its commits are on no other branch.
- **Quiet after** sets the window, for this repository.

The button says how many branches it will delete. Afterwards, each one is listed with DONE or FAILED.

![Tidy for origin: two gone branches, a local one and a leftover copy, both ticked; then merged branches, none of them ticked, each saying when its last commit was and who made it](../img/tidy-light.webp#gh-light-mode-only)
![Tidy for origin: two gone branches, a local one and a leftover copy, both ticked; then merged branches, none of them ticked, each saying when its last commit was and who made it](../img/tidy-dark.webp#gh-dark-mode-only)

## Managing remotes

**Manage remotes…** (in the command palette, or the gear at the top of the Remotes panel) adds, edits and removes remotes. Editing one shows how it's fetched and where pushing to it goes.

![Editing origin in the Remotes dialog: its fetch rules, each with its git key, and the three places a push to it goes](../img/remotes-dialog-light.webp#gh-light-mode-only)
![Editing origin in the Remotes dialog: its fetch rules, each with its git key, and the three places a push to it goes](../img/remotes-dialog-dark.webp#gh-dark-mode-only)

### How a remote is fetched

Each row names the git config key it writes:

| Row | Key | What it does |
|---|---|---|
| **Include in Fetch all** | `remote.<name>.skipFetchAll` | Leaves a slow or rarely used remote out of Fetch. It can still be fetched on its own, from its menu. |
| **Drop branches … has deleted** | `remote.<name>.prune` | Whether a fetch drops your copies of [branches deleted on the remote](#branches-deleted-on-the-remote). |
| **Tags** | `remote.<name>.tagOpt` | Tags that come with their commits (git's default), every tag, or none. |
| **Branches** | `remote.<name>.fetch` | All of them, or only those you list, such as `main, release/*`. |

Refspecs Git Revenge didn't write are listed as they are, and kept. Nothing changes until **Save**, and going back to the defaults leaves nothing extra in your config.

### Pushing to several places

Under **Pushing**, the dialog lists where a push to the remote goes, starting with its own URL. **Also push to…** adds another remote's URL, or any URL, and the **×** beside one takes it off. This is git's own `remote.<name>.pushurl`, so a push from a terminal goes to the same places.

A push URL that is another remote's URL says "follows mirror". When Git Revenge changes that remote's URL, the push URL moves with it, and removing that remote asks whether the others should stop pushing there. Taking the last extra off leaves no `pushurl` behind.

After a push to several places, a message names each one: pushed, already up to date, refused (with git's reason), failed, or not tried. Git stops at the first place it can't reach, so the places after it aren't tried. **Push to the rest** carries on from there, or **Retry** when one place is left.

With **After a push, fetch the other places it went** on, in [Settings › Pushing](settings.md#pushing), a remote whose URL another remote pushes to is fetched after the push, so its branches match what was sent. It's off by default.

## Forges and web pages

A remote on GitHub, GitLab.com, Gitea.com, Codeberg, Bitbucket or Azure DevOps opens on the web as it is. For a server of your own, say what it runs under [Settings › Links › Forges](settings.md#forges). The app never guesses from a host's name, so `gitlab.example.com` has no web pages until you say it's GitLab.

The pages are built from the remote's address, the way a [smart link](settings.md#links) is, and open in your browser. Git Revenge calls no forge API: it doesn't read pull requests, statuses or anything else from the site.

## LFS

With [git-lfs](https://git-lfs.com) installed, the **Repository** panel of the sidebar has an **LFS** row with the number of files. Click it for the details:

- the patterns tracked with LFS, and a way to **track** a new one;
- which files are in LFS, and which are only pointers whose content hasn't been downloaded;
- for lockable files, **Check locks** asks the server what's locked, and you can lock and unlock files.

## Submodules

The **Repository** panel of the sidebar lists the repository's submodules, and says in words when one needs attention. Click one to open it as a tab. **Update** initializes and updates all of them.
