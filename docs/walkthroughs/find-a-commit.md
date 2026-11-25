# Find a commit

Marcus remembers the tide-prediction work closed issue 14, but not when it landed. Search finds it from the trailer alone.

![Searching for "Closes: #14", then stepping through every commit by Sarah](../img/search-light.webp#gh-light-mode-only)
![Searching for "Closes: #14", then stepping through every commit by Sarah](../img/search-dark.webp#gh-dark-mode-only)

## Steps

1. **Open search.** Press <kbd>Ctrl</kbd>+<kbd>F</kbd>, or click the search box above the graph.

2. **Type what you know.** Search looks at:

   - the commit **message** (*tide height*),
   - the **author** (*Sarah*),
   - the **SHA**, or the start of it (*7c810bf*),
   - a **trailer**, such as `Closes: #14` or `Co-authored-by: Sarah`.

   As you type, a list under the box offers the trailer keys and values that contain it. Type `closes:` and it lists the issues that `Closes` names, each with how many commits close it.

3. **Step through the matches.** <kbd>Enter</kbd> goes to the next match and <kbd>Shift</kbd>+<kbd>Enter</kbd> to the previous one. The counter in the box shows which match you're on. Commits that don't match fade, so the matches stand out.

4. **Or filter to it.** Press <kbd>↓</kbd> to a suggestion, such as `#14` under `Closes`, and <kbd>Enter</kbd>. Now the graph shows only the commits with that trailer, and a strip above it says how many, with **Clear** to go back. A search moves between matches; a filter takes everything else out of the way. [Read the changelog down the history](read-the-changelog.md) goes further with filters.

5. **Look closer.** The selected commit's details show on the right. Trailers are listed on their own, and clicking one searches for every commit with the same trailer.

   ![A commit's details: message, Co-authored-by and Closes trailers, author, and a git note](../img/commit-details-light.webp#gh-light-mode-only)
   ![A commit's details: message, Co-authored-by and Closes trailers, author, and a git note](../img/commit-details-dark.webp#gh-dark-mode-only)

## Good to know

- **Search covers the whole history.** On a big repository the graph only loads what's near the screen, but search looks further back, and loads the history up to each match as you reach it.
- **For a file's history**, open the file from any commit and choose **History**. **Blame** shows who last changed each line.

  ![Blame: each line of src/feed.js with the commit and author that last changed it](../img/blame-light.webp#gh-light-mode-only)
  ![Blame: each line of src/feed.js with the commit and author that last changed it](../img/blame-dark.webp#gh-dark-mode-only)

- **A trailer search has a few more forms.** `has:reviewed-by` finds every commit with a key, and a leading `-` finds the ones that don't match: `-has:reviewed-by`. [The commit graph](../guide/graph.md#searching-by-a-trailer) lists them.
- **For branches and tags**, type in the box at the top of the sidebar.
- **For a text in the code**, rather than in a message, the box's next two rows are **Find in the files** and **Commits that add or remove** it. See [searching the code](../guide/searching-code.md).
