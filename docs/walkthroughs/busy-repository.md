# Find your way round a busy repository

Maya is back from two weeks away, and Tidewater has moved on without her. Before reading commits, she wants to know what's there: which branches are live, which have gone quiet, and what's been merged. The Map shows that at a glance, and the timeline, cut down to one branch, shows the rest.

![From the Map to a branch: its card, then the timeline focused on it, then everything again](../img/busy-repository-light.webp#gh-light-mode-only)
![From the Map to a branch: its card, then the timeline focused on it, then everything again](../img/busy-repository-dark.webp#gh-dark-mode-only)

## Steps

1. **Switch the Map on.** It's [experimental](../guide/experimental-views.md), so it's off until asked for. Open **Settings** (<kbd>Ctrl</kbd>+<kbd>,</kbd>) and tick **Map** under **Experimental views**.

2. **Open the Map.** Pick **Map** from the view menu above the graph. Each branch and tag is a node around `main`. A bigger node has more commits of its own, and a node further out has sat still for longer, so recent work is close in and forgotten work is at the edge.

3. **Read it.** The key in the corner tells the nodes apart. A hollow ring has nothing of its own left: it's merged, and is a candidate for [cleaning up](../guide/branch-care.md#cleaning-up-merged-branches). A dashed outline is only on the remote. A tag hangs off the branch that holds it. If the rings are too close or too far for this repository, zoom with **+** and **−**; the Map remembers the zoom for each repository.

4. **Cut the noise.** On a repository with hundreds of remote branches, the switch at the top left shows **Local** branches only, and remote-only branches that share a path, like `dependabot/`, are already gathered into one node.

5. **Pick a branch.** Click `feature/offline-cache` for its card: how many commits, and when it last moved. Choose **Focus the timeline on it**.

6. **Read the branch.** The timeline now shows only that branch and the history it's built on. Its commits are all there is to scroll through, and the bar above the graph says what's focused.

7. **Go back to everything.** Click **×** beside the focus in the bar, or pick **Map** again from the view menu for another branch.

## Good to know

- **Kinds and Fold** cut the timeline down without focusing. The **Kinds** menu above the graph hides whole kinds of branch, such as a bot's, and **Fold** turns a Bot or Docs lane's commits of a day into one line. See [filtering by kind](../guide/graph.md#filtering-by-kind).
- **Pin a node** by dragging it somewhere, to keep a branch you're watching in the same place each visit. Double-click it to let it go.
- **Needs care**, in the sidebar's Branches panel, lists branches whose upstream is gone or whose stack parent was merged, and the merged branches that are safe to delete. It's often the next stop after a look at the Map.
