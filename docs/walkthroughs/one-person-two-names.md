# One person, two names

Jessica fixed a drifting gauge from her own laptop one evening, and the laptop commits under her personal address. Those two commits now count as somebody else's: Tidewater's history has two Jessica Nguyens, one with 127 commits and one with 2.

A mailmap fixes that. It's a text file, `.mailmap`, with a line saying that the two addresses are one person. Tidewater doesn't have one yet, so this walkthrough makes it.

![Opening People, saying the two Jessicas are the same person, and creating the .mailmap](../img/mailmap-light.webp#gh-light-mode-only)
![Opening People, saying the two Jessicas are the same person, and creating the .mailmap](../img/mailmap-dark.webp#gh-dark-mode-only)

## Steps

1. **Open People.** Open the command palette (<kbd>Ctrl</kbd>+<kbd>K</kbd>) and choose **See people**, or click **People** in the sidebar's **Repository** panel. It lists everyone the history names, most commits first.

2. **Read the offer.** At the top, **Looks like the same person** has one match: *Same name, another address*, with Jessica Nguyen at `jessica@tidewater.example` and Jessica Nguyen at `jessica.nguyen@harbourmail.example`. It's an offer. Nothing has been joined.

3. **Say they're the same.** Click **Same person…**. The dialog asks who to show them as, and shows the line it will add:

   ```
   Jessica Nguyen <jessica@tidewater.example> <jessica.nguyen@harbourmail.example>
   ```

   That reads: show commits from the second address as the first. Click **Add the line**.

4. **Look before you save.** The list now has one Jessica, with 129 commits and the other address under her. The bottom of the tab says **1 line not saved**. Nothing has been written yet: **Revert** would put it all back.

5. **Create the file.** Click **Create .mailmap…**. Because the repository has no mailmap, Git Revenge asks first, and shows where the file goes and what it will hold. Click **Create .mailmap**.

6. **Commit it.** Back on the repository's tab, `.mailmap` is a new file among your changes. Tidewater already has a file staged, `src/units.js`, so unstage that first, then stage `.mailmap` and commit it with a message such as "Add a mailmap". [Stage and commit](first-commit.md) covers the staging.

7. **See it in the graph.** Search (<kbd>Ctrl</kbd>+<kbd>F</kbd>) for `drifting gauge` and select the commit. Its author is Jessica Nguyen, and under the name the panel says **This commit says** with the address it was written under. Click her name for a card that counts all 129 commits as hers.

## Good to know

- **No commit changed.** A mailmap changes the name that's shown. The two commits still hold the address they were written with, and deleting the line puts everything back.
- **It counts as soon as it's saved.** The graph follows the `.mailmap` on disk, as `git log` does, so the names changed at step 5. Committing it shares it with everyone else.
- **Insights waits for the commit.** [Insights](../guide/insights.md) reads only a committed `.mailmap`, so its reports are the same for everyone.
- **Not the same person?** **Different people** takes an offer away and remembers the answer for this repository.
- **Joined the wrong two?** **Take apart**, beside the joined name in People, removes the line again.
- **Nobody offered?** Choose **Same person as…** on one person's row, then **This one** on the other. That works for any two people.

The guide has the rest: [People and the mailmap](../guide/people.md).
