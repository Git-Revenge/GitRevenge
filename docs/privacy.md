# Privacy policy

*Effective 13 November 2026. Changes to this policy are made in public, in this file's [history](https://github.com/Git-Revenge/GitRevenge/commits/main/docs/privacy.md).*

Git Revenge is a desktop git client made by Ris Adams and its contributors ("we"). It has no accounts, no servers of its own and no telemetry. This policy covers the app, the website at gitrevenge.com, and the ways we distribute the app. In short: we don't collect personal data, and the few services that see your requests are run by GitHub or by projects you chose to use.

## The app

**What the app sends on its own.** One thing: the update check. At startup, and when you choose **Check for updates**, the app asks GitHub's releases API for the latest version number. The request carries the app's version in its User-Agent (for example `GitRevenge/1.0.0`) and nothing that identifies you or your repositories. GitHub sees your IP address, as it does for any request. Turn the startup check off in [Settings](guide/settings.md).

**What it sends because you asked.** Fetch, pull, push and clone go to your remotes through your own `git`, credential helpers and SSH agent. Links you click, such as a smart link or one in a commit message, open in your web browser. Those requests are between you and the services you're reaching, under their policies.

**What it doesn't do.** It keeps no usage statistics, sends no crash reports, and has no analytics, advertising or tracking. Rendered previews fetch nothing from the network: images from the web are shown as links, HTML runs in a sandbox with no network, and GeoJSON is drawn without map tiles. The Map and Insights are worked out from the repository alone.

**What it keeps on your machine.** Settings, project groups, recent repositories and open tabs, in one `settings.json` file; backups of git config files you save from the app; and a repository's own settings in its git config. The [install guide](guide/install.md#where-your-data-lives) says where they are. They never leave your machine. To remove them, delete that folder after uninstalling. Passwords and passphrases are passed from the prompt straight to `git` and are never written down by the app.

## The website

gitrevenge.com and packages.gitrevenge.com are static sites hosted on GitHub Pages. They set no cookies and run no analytics, advertising or tracking scripts, and the fonts are served by the site itself. GitHub, as the host, receives your IP address and request details and may log them for security; see the [GitHub General Privacy Statement](https://docs.github.com/en/site-policy/privacy-policies/github-general-privacy-statement). We don't have access to those logs.

## Downloads and package managers

- **GitHub Releases**: downloading a release is a request to GitHub. GitHub publishes download counts for each file, which we can see; they aren't tied to anyone.
- **apt and dnf** (packages.gitrevenge.com): your package manager's requests go to GitHub Pages, as above.
- **Homebrew**: Homebrew collects its own anonymous analytics unless you turn them off. That's Homebrew's, not ours; see [Homebrew's analytics page](https://docs.brew.sh/Analytics).

## When you contact us

Issues, discussions, pull requests and security advisories live on GitHub, and what you post there is handled under GitHub's privacy statement. Issues and pull requests are public. A security advisory is private between you and the maintainers until it's published, and we credit reporters only with their agreement. If you sponsor the project through GitHub Sponsors, GitHub handles the payment; we see what GitHub Sponsors shows any maintainer, such as your GitHub username and tier, unless you sponsor privately.

## Your rights

Because we don't collect personal data, there's nothing of yours for us to look up, correct, export or delete. For anything you've posted on GitHub, you can edit or delete it there, or ask GitHub under its privacy statement.

## Questions

Ask in a [GitHub issue](https://github.com/Git-Revenge/GitRevenge/issues), or contact the maintainer, Ris Adams, through the details on [their GitHub profile](https://github.com/risadams) for anything you'd rather not post in public.
