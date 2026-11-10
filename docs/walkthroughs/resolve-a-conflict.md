# Resolve a merge conflict

Maya switched the dashboard's wind readings to knots on `main`. Meanwhile Sam, on `feature/metric-units`, switched them to metres per second. Both changed the same lines of `src/feed.js`, so when Maya merges Sam's branch, git can't choose between them. That's a conflict, and it needs a person to decide.

![Merging a branch, opening the conflict tool, taking one side, and finishing the merge](../img/conflict-light.webp#gh-light-mode-only)
![Merging a branch, opening the conflict tool, taking one side, and finishing the merge](../img/conflict-dark.webp#gh-dark-mode-only)

## Steps

1. **Merge.** In the sidebar, open the `feature` folder, right-click `metric-units`, and choose **Merge feature/metric-units into main…**. A preview names the files that will conflict; press **Merge, and resolve 1 conflict**. Git merges everything it can and stops at the conflict. A strip at the top says *Merging feature/metric-units into main* and how many files need resolving.

2. **Find the conflicted files.** Click the row at the top of the graph that says how many files changed. Conflicted files are listed first, under **Conflicts**.

3. **Decide, file by file.** For each file, either:

   - choose **Ours** (keep your branch's version) or **Theirs** (take the incoming one) right there in the list, or
   - click the file to open the **conflict tool**, when you need to see the detail or combine both.

4. **Use the conflict tool.** It shows your version (*ours*) and the incoming one (*theirs*) side by side, and below them the **result**, which you can edit.

   ![The conflict tool: ours and theirs side by side, with the editable result below](../img/conflict-tool-light.webp#gh-light-mode-only)
   ![The conflict tool: ours and theirs side by side, with the editable result below](../img/conflict-tool-dark.webp#gh-dark-mode-only)

   - **Use ours** and **Use theirs** at the top take one whole version and mark the file resolved, with nothing more to do in the tool.
   - For each conflict in the file, the numbered buttons take **Ours**, **Theirs**, **Both**, or the **Base** they started from.
   - Or edit the result by hand, for the cases where the answer is a bit of each.

   After taking the conflicts one by one or editing by hand, click **Mark resolved** when the result is right.

5. **Finish the merge.** Once nothing is left in **Conflicts**, click **Commit the merge** in the strip. Git Revenge makes the merge commit, and a notice offers **Undo**.

## Good to know

- **Not ready to decide?** **Abort…** in the strip says what it undoes, then puts everything back as it was before the merge.
- **Rebases and cherry-picks stop the same way.** The strip says which operation is in progress and which commit it stopped on, and the steps are the same. A rebase may stop more than once, once for each commit that conflicts.
- **Started the merge in a terminal?** Git Revenge notices, and shows the same strip and conflict list.
- **Merged and regretted it?** <kbd>Ctrl</kbd>+<kbd>Z</kbd> takes the merge back. See [Undo a mistake](undo-a-mistake.md).
