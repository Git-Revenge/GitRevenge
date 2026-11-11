# Find what broke it

The station map says the wind at Harbour Light is 12 km/h. The harbour master's own gauge says 22. The map was right in release 0.2.0, so something since then broke it, and Leo doesn't know which commit.

That's what a bisect is for. You name one commit where it's broken and one where it worked, and git checks out commits in between for you to try. Each answer halves what's left, so even a thousand commits take about ten tries.

![Starting a bisect from the command palette, answering Good and Bad, and arriving at the commit that broke it](../img/bisect-light.webp#gh-light-mode-only)
![Starting a bisect from the command palette, answering Good and Bad, and arriving at the commit that broke it](../img/bisect-dark.webp#gh-dark-mode-only)

## Before you start

Commit or stash what you're working on. A bisect checks out other commits, and git won't do that over changes it would overwrite.

## Steps

1. **Start it.** Open the command palette (<kbd>Ctrl</kbd>+<kbd>K</kbd>) and choose **Find the commit that broke it…**. To start from an older commit instead of the one you're on, right-click it in the graph and choose **Find what broke it, up to here…**.

2. **Name the two ends.** **Bad** is the commit where it's broken, and starts as the one you're on. **Good** is a commit, tag or branch where it still worked. The dialog suggests the last tag, here `v0.2.0`, and says how many commits lie between the two and about how many steps that is.

3. **Try the commit you're given.** Git Revenge checks out a commit from the middle of the range, and a strip under the tabs asks whether it works there. Build it, or run it, and try the thing that's broken. For Leo that means opening the map and reading the wind.

4. **Answer.**

   | Answer | Key | When |
   |---|---|---|
   | **Good** | <kbd>G</kbd> | It works at this commit. |
   | **Bad** | <kbd>B</kbd> | It's broken at this commit. |
   | **Can't tell** | <kbd>S</kbd> | This commit won't build, or can't be tested. Git picks another nearby. |

   Each answer rules out half of what's left, and the next commit to try is checked out. The graph keeps score: a tick or a cross on the commits you've answered for, a ring on the one under test, a teal band on those still in question, and everything ruled out fades.

5. **Read the answer.** When one commit is left, the strip says how many steps it took, and the graph marks that commit "it broke here". Here it's *Show wind in knots*: the feed changed to knots, and the map went on labelling the number km/h. The panel on the right shows what the commit changed, and offers to open its changes, start a branch for the fix, or revert it.

6. **Finish.** **Done** takes you back to the branch you started on.

## Good to know

- **Pressed the wrong one?** **Undo last** takes back your last answer and checks that commit out again.
- **Changed your mind?** **Stop…** ends the bisect and returns you to where you started. Nothing about your branches has changed.
- **Writing it up?** **Copy the log** copies your answers as text, ready to paste into an issue.
- **Too many "Can't tell"?** If the commits you skipped hide the answer, Git Revenge tells you which commits it could be instead of guessing.
- **It's git's own bisect.** One started with `git bisect` in a terminal shows up in the app, and one started here carries on in a terminal.

The guide has the rest: [Finding the commit that broke something](../guide/history.md#finding-the-commit-that-broke-something).
