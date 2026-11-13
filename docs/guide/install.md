# Installing

Git Revenge is free, and there's nothing to sign up for. Download the Windows installer, the macOS disk image, or a Linux package from the [latest release](https://github.com/Git-Revenge/GitRevenge/releases/latest). Each is under 20 MB.

- **macOS:** the disk image is signed and notarized, so it opens like any other app. With [Homebrew](https://brew.sh), install and update it from the project's tap instead:

  ```sh
  brew install --cask git-revenge/tap/gitrevenge
  ```

- **Windows:** the installer isn't signed yet, so SmartScreen warns you before it runs. Choose **More info**, then **Run anyway**.
- **Linux:** see [Linux](#linux) for which package to use.

When a new release comes out, the app tells you at startup. You can also check by hand with **Check for updates** in the command palette. That check is the only request the app makes on its own, and [Settings](settings.md) turns it off.

## What you need

- **git 2.40 or newer.** Git Revenge doesn't bundle git. It finds the one on your `PATH`, or you can point it at a specific one in [Settings](settings.md).
- **git-lfs**, if you want the [LFS features](remotes.md#lfs).

Check your version with:

```sh
git --version
```

## Linux

Git Revenge supports Ubuntu 24.04, Debian 13, RHEL 9, and RHEL 10 with EPEL, or newer, on x86-64. Older Ubuntu and Debian releases ship a `git` older than 2.40.

| System | Package | Install with |
|---|---|---|
| Ubuntu, Debian | `GitRevenge-<version>-linux-amd64.deb` | `sudo apt install ./GitRevenge-*.deb` |
| RHEL 9 | `GitRevenge-<version>-el9.x86_64.rpm` | `sudo dnf install ./GitRevenge-*-el9.x86_64.rpm` |
| RHEL 10 | `GitRevenge-<version>-el10.x86_64.rpm` | `sudo dnf install ./GitRevenge-*-el10.x86_64.rpm` |

These commands also install what the app needs, including `git`. Packages aren't signed, and there's no apt or dnf repository, so you update by installing the next release the same way.

### RHEL 10

Red Hat removed WebKitGTK from RHEL 10, so it has to come from [EPEL](https://docs.fedoraproject.org/en-US/epel/). Turn EPEL on before installing the package:

```sh
sudo subscription-manager repos --enable codeready-builder-for-rhel-10-$(arch)-rpms
sudo dnf install https://dl.fedoraproject.org/pub/epel/epel-release-latest-10.noarch.rpm
```

### If live refresh is off

On Linux, watching a repository takes one inotify watch per folder, including ignored ones like `node_modules`. If a repository needs more than your system allows, Git Revenge tells you when you open it, and changes made outside the app won't show up on their own. To raise the limits:

```sh
printf 'fs.inotify.max_user_watches=524288\nfs.inotify.max_user_instances=512\n' | sudo tee /etc/sysctl.d/90-gitrevenge.conf
sudo sysctl --system
```

Then open the repository again.

### If the window is blank

Some graphics setups, especially with NVIDIA drivers, can leave WebKitGTK with a blank window. Start the app like this to try without its DMABUF renderer:

```sh
WEBKIT_DISABLE_DMABUF_RENDERER=1 gitrevenge
```

## Large repositories

The commit graph loads history as you scroll, so it doesn't need the whole history up front. It's still much faster to open a big repository if git has a commit-graph file for it. Write one once:

```sh
git commit-graph write --reachable
```

and have git keep it current every time you fetch:

```sh
git config fetch.writeCommitGraph true
```

## Where your data lives

Settings, project groups, recent repositories and open tabs are kept in one file:

- **Windows:** `%APPDATA%\GitRevenge\settings.json`
- **macOS:** `~/Library/Application Support/GitRevenge/settings.json`
- **Linux:** `~/.config/GitRevenge/settings.json`, or under `$XDG_CONFIG_HOME` if you set it

Backups of git config files you save in the [Git Config tab](git-config.md) go in a `config-backups` folder beside it. A repository's own settings, such as its branch kinds and smart links, are kept in its git config. Nothing about your repositories is stored anywhere else, and nothing is sent anywhere; see [what leaves your machine](#what-leaves-your-machine). If the file is ever damaged, the app moves it aside as `settings.json.corrupt-<timestamp>` and starts with defaults.

## What leaves your machine

Git Revenge is local-first. It makes one request of its own, and everything else that reaches the network is something you asked for. The [privacy policy](../privacy.md) covers the website and the download channels too.

| What | Where it goes | When |
|---|---|---|
| The update check | GitHub's releases API, for the latest version number | At startup, and when you choose **Check for updates**. Turn it off in [Settings](settings.md). |
| Fetch, pull, push and clone | Your remotes, through your own `git`, credential helpers and SSH agent | When you ask for one |
| A link | Your web browser | When you click a link, such as a [smart link](settings.md#links) or one in a commit message. The app only builds the address |

Everything else stays on your machine, including the parts that could easily have reached out:

- **Rendered previews** fetch nothing. A Markdown or notebook image from the web is shown as a link, an HTML page runs in a sandbox with no network, a GeoJSON file is drawn without map tiles, and audio, video and PDFs are read from the repository.
- **The Map and Insights** are worked out from the repository alone, with no hosting service's API.
- **No telemetry.** The app keeps no usage statistics and sends no crash reports.
- **The docs site** loads its fonts from itself, not from a font service.
