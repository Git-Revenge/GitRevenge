# Find a commit

Marcus remembers the tide-prediction work closed issue 14, but not when it landed. Search finds it from the trailer alone.

![Searching for "Closes: #14", then stepping through every commit by Sarah](../img/search-light.webp#gh-light-mode-only)
![Searching for "Closes: #14", then stepping through every commit by Sarah](../img/search-dark.webp#gh-dark-mode-only)

## Steps

1. **Open search.** Press <kbd>Ctrl</kbd>+<kbd>F</kbd>, or click the search box above the graph.

2. **Type what you know.** Search looks at:

   - the commit **message** (*tide height*),
   - the **author** (*Sarah*),
   - the **SHA**, or the start of it (*6fd96d0*),
   - a **trailer**, such as `Closes: #14` or `Co-authored-by: Sarah`.

3. **Step through the matches.** <kbd>Enter</kbd> goes to the next match and <kbd>Shift</kbd>+<kbd>Enter</kbd> to the previous one. The counter in the box shows which match you're on. Commits that don't match fade, so the matches stand out.

4. **Look closer.** The selected commit's details show on the right. Trailers are listed on their own, and clicking one searches for every commit with the same trailer.

   ![A commit's details: message, Co-authored-by and Closes trailers, author, and a git note](../img/commit-details-light.webp#gh-light-mode-only)
   ![A commit's details: message, Co-authored-by and Closes trailers, author, and a git note](../img/commit-details-dark.webp#gh-dark-mode-only)

## Good to know

- **Search covers the whole history.** On a big repository the graph only loads what's near the screen, but search looks further back, and loads the history up to each match as you reach it.
- **For a file's history**, open the file from any commit and choose **History**. **Blame** shows who last changed each line.

  ![Blame: each line of src/feed.js with the commit and author that last changed it](../img/blame-light.webp#gh-light-mode-only)
  ![Blame: each line of src/feed.js with the commit and author that last changed it](../img/blame-dark.webp#gh-dark-mode-only)

- **For branches and tags**, type in the box at the top of the sidebar.
