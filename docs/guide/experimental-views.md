# Experimental views

The graph has two views besides Timeline and Compact, for looking at history another way. They're experimental: off until you ask for them, and they may change or go away between versions.

To try one, open [Settings](settings.md) (<kbd>Ctrl</kbd>+<kbd>,</kbd>) and tick **Horizontal** or **Map** under **Experimental views**. The view joins the [view menu](graph.md#views) above the graph, under a handwritten "experimental" label. Pick it there, and pick Timeline to go back. The view you pick is remembered, as Timeline and Compact are. Untick a view in Settings and the graph goes back to the rows.

Both views read the same history as the timeline and share its selection, so a commit you choose in one is the commit selected when you switch back.

## Horizontal

Horizontal turns the timeline on its side: time runs across, a column for each commit, with the newest on the left, and each lane is a row.

![The Horizontal view: lanes as rows, commits as columns, newest on the left](../img/view-horizontal-light.webp#gh-light-mode-only)
![The Horizontal view: lanes as rows, commits as columns, newest on the left](../img/view-horizontal-dark.webp#gh-dark-mode-only)

- **Lane names.** A pinned column on the left names each lane after the branch whose tip started it, even once that tip has scrolled away. Lanes are reused from one branch to the next, so the names belong to the commits at the left edge and change as you scroll. A lane that a merge closed is named from the merge's subject, such as the branch a pull request came from.
- **Labels.** Branch and tag labels sit on their commits, and a row along the top holds the tags.
- **Days.** A band along the top marks where each day starts.
- **Working tree.** Your uncommitted changes are the leftmost column, before the newest commit. Click it, or press <kbd>←</kbd> from the newest commit, to open them.
- **Folded lanes.** [Bot and Docs lanes](graph.md#folded-lanes) fold here too: a day's commits on such a lane take one column. Click the column to open it.
- **Ruler.** The scrollbar along the bottom is the timeline's [ruler](graph.md#the-ruler) turned on its side: its length is the whole history, with ticks for branches, tags, the checked-out commit and search matches. Drag the box, or press anywhere on it to go there.

The wheel scrolls through time. With <kbd>Shift</kbd> held, it scrolls the lanes, when there are more than fit. From the keyboard, <kbd>→</kbd> moves to an older commit and <kbd>←</kbd> to a newer one, <kbd>Page Up</kbd> and <kbd>Page Down</kbd> move a screenful, and <kbd>Home</kbd> and <kbd>End</kbd> go to the newest and the oldest.

Selecting a commit, right-clicking it for its menu, and dragging one commit onto another work as they do in the timeline.

## Map

Map has no commits in it. Each branch and tag is a node, held around the trunk in the middle. A node's size is how many commits it has of its own, and its distance from the trunk is how long since it last moved, so the branches being worked on gather close in and the forgotten ones drift out. It's a way to see the shape of a busy repository and find your way into it, not a place to work.

![The Map view: branches and tags around the trunk, sized by commits, with rings for time](../img/view-map-light.webp#gh-light-mode-only)
![The Map view: branches and tags around the trunk, sized by commits, with rings for time](../img/view-map-dark.webp#gh-dark-mode-only)

The key in the bottom left tells the nodes apart: a local branch, a branch that's only on the remote, a branch with nothing of its own left (merged), and a tag. The checked-out branch is always named, and says "you're here". A branch checked out in another [worktree](branch-care.md) carries a worktree badge, and a branch [stacked](branch-care.md#stacked-branches) on another hangs off that branch instead of the trunk. Your working tree is a small node beside the checked-out branch; click it to open your changes.

Click a node for its card: what it is, how many commits, when it last moved, and buttons to **Show it in the timeline** or, for a branch, **Focus the timeline on it**. Right-click a node for the same menu it has in the sidebar.

- **Rings.** Rings round the trunk mark time, and each is named along whichever side is clearest of nodes. Zoom with the **+** and **−** buttons in the bottom right, the <kbd>+</kbd> and <kbd>−</kbd> keys, or the wheel: all the way in, the map is one week with a ring a day; out, the rings become weeks, then months, then years. The map opens at the zoom that fits most of the repository, and remembers the zoom you choose for each one. What's older than the last ring isn't drawn: a tally past it says how many, and zooming out brings them in.
- **Names.** In a crowd there isn't room for every name. The checked-out branch, nodes you've pinned and local branches are named first, then the rest by size, and a name that would cover another is left off. Hover over a node, or move to it with <kbd>Tab</kbd>, to see its name.
- **Pins.** Drag a node to put it somewhere else. It stays there for that repository, and the rest make room for it. Its card has **Let it go back to its place**.
- **Groups.** A repository with hundreds of remote branches would bury the map. Remote-only branches that share the first part of their name (`dependabot/`, `release/`) become one node when there are three or more. Click a group, then **Show its branches**: they stand round it, and only then are their commits counted.
- **All, Local, Remote.** The switch in the top left shows every branch, only your local ones, or only those that are on the remote alone. The choice is kept for each repository.

The bar above the graph says what the map holds, such as "8 branches · 13 tags".
