# People and the mailmap

Over a few years one person commits under several names and addresses: a work address and a personal one, a new surname, a laptop that was never set up. Git counts each as someone else. A **mailmap** is git's fix for that: a text file, `.mailmap`, that says which names and addresses are one person.

Git Revenge reads the mailmap wherever it shows a name, and the **People** tab is where you see who a history names, join the ones that are the same person, and edit the file.

A mailmap never changes a commit. It only changes the name that's shown, and taking a line out puts every name back.

A repository doesn't need a mailmap. Everything here works without one, and Git Revenge never creates one unless you say so.

## Which mailmap is read

Git Revenge shows the names `git log` shows in a terminal. That means the mailmap git itself reads:

- **`.mailmap`** at the top of the repository, as it is on disk. A change you've saved counts at once, before you commit it.
- **`mailmap.file`** and **`mailmap.blob`**, if you've set either in your git config. These are yours alone; nobody else's graph follows them.

The graph, a commit's details, file history, blame, search, the lists of commits in merge and rebase previews, and the Map's Authors mode all follow those.

[Insights](insights.md) is the exception. It reads only the `.mailmap` committed on the branch it reports on, and leaves your own mailmap file out, so that anyone running the same report gets the same people. If your `.mailmap` has changes you haven't committed, Insights can show someone under a name the graph has already joined.

## Opening People

Open it with the **People** row in the sidebar's **Repository** panel, or **See people** in the command palette. It opens as a tab beside your repositories. With more than one repository open, the menu at the top left chooses which one it shows.

![The People tab: likely matches at the top, then everyone the history names, most commits first](../img/people-light.webp#gh-light-mode-only)
![The People tab: likely matches at the top, then everyone the history names, most commits first](../img/people-dark.webp#gh-dark-mode-only)

The panel on the left lists the mailmap files: the repository's `.mailmap` with its number of lines, or "none", and under **from your git config** any `mailmap.file` or `mailmap.blob`. Those two can be read here and not edited.

The first time you open People, it reads every commit to find who's named. Most repositories take a moment; the Linux kernel's 1.5 million commits take about half a minute. The answer is kept until a branch moves.

## The list

**People** lists everyone the history names, most commits first. Each person has:

- the name and address they're shown as;
- how many commits they wrote, and how many more credit them in a `Co-authored-by` trailer;
- under them, every other name and address that's read as them, with its own count and the line of the mailmap that joins it;
- a **bot** mark where the repository's [bot authors](settings.md#bot-authors) match.

The box above the list finds a person by any of their names or addresses, shown or written. The count beside it says how many names and bots the history has, on how many commits.

## Saying two people are one

### Likely matches

Above the list, **Looks like the same person** offers the matches Git Revenge is fairly sure of: the same name under another address, or the same address under another name. These are offers only. Nothing is joined unless you say so.

Each offer has two answers:

- **Same person…** opens a dialog that shows the line before anything changes.
- **Different people** takes the offer away. It's remembered for this repository, in its git config, and the pair moves to **Kept apart**, where **Offer again** brings it back.

Some matches aren't offered, because they're usually wrong. They're folded away under **Bots and shared addresses**, each with the reason:

- a bot;
- a no-reply address, which a host gives to many people;
- a name many machines commit under, such as `root`;
- an address that three or more unlike names share, such as a team's.

Open that section to join any of them yourself.

### Any two people

**Same person as…** on a person's row joins any two people, offered or not. Choose it on one, then **This one** on the other.

### The dialog

![The Same person dialog: who to show them as, and the line it will add](../img/people-same-person-light.webp#gh-light-mode-only)
![The Same person dialog: who to show them as, and the line it will add](../img/people-same-person-dark.webp#gh-dark-mode-only)

**Show them as** chooses which of them is shown from now on. You can type a name and address of your own instead.

Under that is the exact line the mailmap will get. Git Revenge writes the shortest line that works:

- For another address, `Proper Name <proper@email> <commit@email>`.
- For another name on the same address, `Proper Name <commit@email>`.

Neither spells out the old name. A line names the old name only when the address alone isn't enough, because someone you aren't joining uses that address too. The dialog says when that's so.

Before offering a line, Git Revenge checks it against everyone in the history, and doesn't use one that would change anybody outside the join.

The line goes beside the person's other lines if they have any. Otherwise, in a file that's kept in order, it goes where the order puts it, and failing that at the end. No existing line moves.

**Add the line** makes it an edit. Nothing is written until you [save](#saving).

### Taking someone apart

**Take apart**, beside a joined name, removes the line that joins it, and only for that one name. A single line for an address joins every name under it, so if that line was joining others too, each of them gets a line of its own and stays joined. A note at the top says so when it happens.

Only lines in the repository's `.mailmap` can be taken apart here. A join that comes from your `mailmap.file` is changed in that file.

## The mailmap as text

**Text**, at the top right, shows the file one line to a row, each with what it does now:

- **joins 2 commits**, or how many names and commits it joins;
- **changes nothing**, for a line whose people already read that way, such as one that only names the proper spelling for an address;
- **matches nobody now**, for a line about someone who isn't in the history;
- **a later line replaces this**, for a line git no longer uses;
- **git skips this**, with why, for a line git can't read.

![The mailmap as text, each line with what it does](../img/people-text-light.webp#gh-light-mode-only)
![The mailmap as text, each line with what it does](../img/people-text-dark.webp#gh-dark-mode-only)

Double-click a line to edit it, delete it with the bin, or add one in the field at the bottom. **Edit as plain text** swaps the rows for the whole file in one box, for comments or many changes at once.

The list, the notes and "Who is this?" all follow your edits as you make them, saved or not, so you can see what a line does before you keep it.

A mailmap line takes one of four forms:

| Line | What it does |
|---|---|
| `Proper Name <commit@email>` | Shows that name for the address. |
| `<proper@email> <commit@email>` | Shows another address for it. |
| `Proper Name <proper@email> <commit@email>` | Shows both another name and another address. |
| `Proper Name <proper@email> Commit Name <commit@email>` | The same, for that one name on the address only. |

Matching ignores the case of names and addresses. These are git's rules; `git help check-mailmap` and gitmailmap(5) have the detail.

## Who is this?

The panel on the right answers for one name or address. Type either, as a commit has it, and it says who that's read as, by which line, and for how many commits. Type an address no commit uses to see how the mailmap would read it.

## Saving

**Save .mailmap** writes the file. It then shows up among your [changes](changes.md) like any other edit, ready to commit, and the graph and everything else shows the new names at once.

Where the repository has no `.mailmap`, the button reads **Create .mailmap…** and asks first, showing where the file will go and the lines it will hold. **Not now** leaves the repository as it was.

**Revert** drops your edits. Closing the tab with unsaved edits asks before discarding them.

Commit the `.mailmap` and push it, and everyone else's graph, and `git log`, follow it too.

## In the graph

Once names are joined, a commit that was written under another name still says so. See [Authors](graph.md#authors) in the commit graph guide for "This commit says" and the card about an author.

[One person, two names](../walkthroughs/one-person-two-names.md) walks through joining someone on the practice repository.
