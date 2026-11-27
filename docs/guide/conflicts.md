# Resolving conflicts

When a merge, rebase, cherry-pick or revert finds both sides changed the same lines, it stops and leaves the file for you. Git Revenge shows where things stand and gives you a tool to settle each file, place by place. For code, it reads the three versions and suggests where they can be combined.

When a merge, rebase or cherry-pick stops on conflicts, the conflicted files are listed with your uncommitted changes. For each one you can:

- resolve it with **ours** or **theirs** in one click, or
- open the **conflict tool** to decide place by place.

In the conflict tool the **result** is the main view: the file as it will be saved, with a block wherever the two sides disagree. Conflict markers never appear in it and are never saved.

- Each **place** has its own choices: **Ours**, **Theirs** or **Both** (ours first). Until you choose, the place is empty and shows what each side would put there. **Start again** puts a place back to undecided.
- The result is an ordinary editor. Typing inside a place makes it **Edited**, which counts as decided; typing anywhere else just changes the file.
- Above the result, **Ours and theirs** shows both sides of the current place with a few lines around them, each labelled with its branch and the commit that last changed those lines. **Three-way** adds the **Original**, where both sides started. **Result only** hides them. **Take** on a side, or <kbd>1</kbd> and <kbd>2</kbd> (with <kbd>Alt</kbd> while typing), takes that side for the current place.
- **Next** and **Previous**, or <kbd>F7</kbd> and <kbd>Shift</kbd>+<kbd>F7</kbd>, move between places, going to the undecided ones first.
- **Mark resolved** stays off until every place is decided, and its tooltip says how many are left. It saves the result, stages the file, and opens the next conflicted file if there is one.

The panel on the right lists the files of the operation with their progress, and the files that merged without help. For the whole file it offers **Take ours** and **Take theirs**, **Start this file again**, and, when `merge.tool` is set in your git config, **Open in** that tool.

A binary file, or one a side deleted, has no lines to choose between: the tool offers the whole-file choices only.

![The conflict tool: both sides of a place above the result](../img/conflict-tool-light.webp#gh-light-mode-only)
![The conflict tool: both sides of a place above the result](../img/conflict-tool-dark.webp#gh-dark-mode-only)

See [Resolve a merge conflict](../walkthroughs/resolve-a-conflict.md) for the whole thing, step by step.

## Suggestions

git compares lines, so two edits to the same line conflict even when they don't touch: one side adds an argument at the start, the other one at the end. For a file in a language the app [reads as code](diffs.md#reading-the-code), the conflict tool merges each place again token by token, using the three versions: ours, theirs and the original.

- **Where the sides changed different tokens,** both changes go in, and the place starts filled in and marked **Suggested**, with a dashed edge. Its heading says why, such as *ours added `ctx context.Context,`; theirs added `, step time.Duration`*.
- **In the result,** each part of a suggestion is washed by where it came from: teal for ours and plum for theirs, the colours the tool already uses for each side.
- **In the panes above,** each side's own changes against the original are marked, so you can see what the suggestion combined.

A suggestion is never kept for you. It counts as undecided until you keep it or change it, and **Mark resolved** stays off until then. Its tooltip counts the suggestions to keep and the places still undecided separately.

- **Keep the suggestion,** or <kbd>3</kbd> (with <kbd>Alt</kbd> while typing), keeps it for the current place. **Keep all N suggestions** keeps every one still waiting, after you've looked them over.
- **Ours** and **Theirs** take one side instead. **Start again** puts the suggestion back.
- Typing in a suggested place makes it **Edited**, as with any place.

Where both sides changed the same token, such as one number to two different values, the place stays undecided and says why: *can't be combined: both sides changed `48`*. **Write it myself** fills it with both sides, ours first, ready to edit.

![The conflict tool on config.yaml: a place filled in with a suggestion that takes both sides' changes, and one where both sides changed the same value](../img/conflict-suggestion-light.webp#gh-light-mode-only)
![The conflict tool on config.yaml: a place filled in with a suggestion that takes both sides' changes, and one where both sides changed the same value](../img/conflict-suggestion-dark.webp#gh-dark-mode-only)

A string or a comment is one token, so two edits inside the same string are never combined.

**Imports merge as a set.** Their order means nothing, so when both sides add a different import in the same spot, both go in, and one that either side removed stays removed. A list that both sides keep sorted stays sorted. The heading says so: *imports merge as a set: ours added `"log"`; theirs added `"io"`*. This covers Go, JavaScript, TypeScript, Python, Java, Kotlin, Scala, Groovy, C#, Rust, C, C++ and Objective-C includes, PHP, Swift, Dart, Ruby, Haskell and Elixir. A module both sides import in different ways, under two names, say, is still a conflict.

The chip beside the file name says how it was read: **Read as Go**, say, or **Read by lines** when it couldn't be, with the reason in its tooltip. A file that doesn't parse, or isn't in a language the app reads as code, keeps the tool as described above.

## Conflicts you've resolved before

Git can remember how you resolved a conflict and resolve it the same way when it comes back, as it does when you rebase a long branch again and again. Git calls this rerere ("reuse recorded resolution"), and `rerere.enabled` in your git config turns it on.

**When rerere is on**, a file it resolved says **resolved as before** in the list of files, and git reuses the resolution in one of two ways:

- **Still to mark resolved:** git leaves the file conflicted, with your old resolution in it. Opening it shows the resolution, to check and then mark resolved.
- **Staged straight away:** with `rerere.autoUpdate`, git stages the file as well. It's listed with the files to settle as resolved as before, never among the files that merged cleanly, so you can tell it apart.

Opening a file rerere resolved says so. **Forget that resolution**, or **Forget** beside a staged one, throws the recorded resolution away (`git rerere forget`) and puts the conflict back in the file, to resolve again. The new resolution is what git reuses next time.

**When rerere is off**, the app remembers the conflicts you resolve in the conflict tool: the conflicting text, as rerere matches it, never the file. When one comes back, the panel says **Seen before** and offers **Turn on rerere**, which sets `rerere.enabled` in the repository's own config. The conflicts still open are recorded then too, so resolving them now is what git reuses. **No thanks** puts the offer away, and it isn't made again in that repository.

## Merge drivers

git can hand a file to a merge driver of your own instead of its line merge, set in `.gitattributes` with `merge=<name>`. [mergiraf](https://mergiraf.org/), for instance, merges by syntax in the terminal too. The app never installs or bundles one; it works with whatever git runs.

To use one, name it for the files in `.gitattributes`:

```
*.go   merge=mergiraf
*.json merge=mergiraf
```

and say what it runs in your git config:

```
git config --global merge.mergiraf.name mergiraf
git config --global merge.mergiraf.driver "mergiraf merge --git %O %A %B -s %S -x %X -y %Y -p %P -l %L"
```

Then:

- The conflict tool's panel says when a file went to a driver first, such as **mergiraf ran first**, with the command from your config. What's left in the tool is what the driver couldn't settle.
- A file a driver merged on its own is listed as **merged by mergiraf**, and the strip counts it: *2 settled by mergiraf*.
- If `.gitattributes` names a driver your git config doesn't set up, git falls back to its own line merge, and the panel says so.

A merge driver is a program in your git config, so the app asks before working in a repository whose own config sets one. See [A repository that runs programs of its own](repositories.md#a-repository-that-runs-programs-of-its-own).



## While a merge or rebase is in progress

A strip at the top stays until the operation is finished. It names the operation and both sides, such as *Rebasing feature/offline-cache onto main*, and says where things stand: which commit a rebase stopped on and how far along it is, or how many of a merge's conflicted files are resolved.

![The strip for a merge in progress, with one file left to resolve](../img/operation-strip-light.webp#gh-light-mode-only)
![The strip for a merge in progress, with one file left to resolve](../img/operation-strip-dark.webp#gh-dark-mode-only)

- **Resolve N files** opens the first conflicted file in the conflict tool.
- **Continue** is off while anything is conflicted, and its tooltip names what's left. Once everything is resolved it says what it will do: **Continue with 2 more**, **Finish the rebase** or **Commit the merge**. <kbd>Ctrl</kbd>+<kbd>Enter</kbd> presses it.
- **Skip this commit** leaves out the commit a rebase, cherry-pick or revert stopped on.
- **Abort…** asks first, and says what it undoes: where your branch goes back to, and what is thrown away.

When the stop was read as code, the strip also counts what the conflict tool will find: *2 suggestions · 1 undecided*, and any files a [merge driver](#merge-drivers) settled. The count follows a moment behind, as every conflicted file is read. Suggestions are worked out again at every stop.

During a rebase the graph ticks each commit as it's written again on top of the target, and the commit in hand carries its number.

When the operation finishes, a notice says what happened and offers **Undo**, which puts the branch back where it was before the operation began. After a rebase of a branch that was already pushed, a second notice offers **Force push…**.

The strip appears for an operation started in a terminal too.

Any of these can be undone afterwards; see [Undo and redo](changes.md#undo-and-redo).
