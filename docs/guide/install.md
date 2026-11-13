# Installing

Git Revenge is free, and there's nothing to sign up for. Download the Windows installer or the macOS disk image from the [latest release](https://github.com/Git-Revenge/GitRevenge/releases/latest), or install with Homebrew, apt or dnf. Each download is under 20 MB.

- **macOS:** the disk image is signed and notarized, so it opens like any other app. With [Homebrew](https://brew.sh), install and update it from the project's tap instead:

  ```sh
  brew install --cask git-revenge/tap/gitrevenge
  ```

- **Windows:** the installer isn't signed yet, so SmartScreen warns you before it runs. Choose **More info**, then **Run anyway**.
- **Linux:** install from the project's apt or dnf repository, so updates come with the rest of your system. See [Linux](#linux).

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

The project keeps signed apt and dnf repositories at [packages.gitrevenge.com](https://packages.gitrevenge.com). Add one once, and Git Revenge updates along with the rest of your system. Installing also brings in what the app needs, including `git`.

### Ubuntu and Debian

```sh
sudo install -d -m 0755 /etc/apt/keyrings
sudo curl -fsSLo /etc/apt/keyrings/gitrevenge.asc https://packages.gitrevenge.com/gitrevenge.asc
sudo tee /etc/apt/sources.list.d/gitrevenge.sources >/dev/null <<'EOF'
Types: deb
URIs: https://packages.gitrevenge.com/apt
Suites: stable
Components: main
Architectures: amd64
Signed-By: /etc/apt/keyrings/gitrevenge.asc
EOF
sudo apt update && sudo apt install gitrevenge
```

### RHEL 9 and 10

On RHEL 10, [turn on EPEL](#rhel-10) first. Then:

```sh
sudo curl -fsSLo /etc/yum.repos.d/gitrevenge.repo \
  "https://packages.gitrevenge.com/rpm/gitrevenge-el$(rpm -E %rhel).repo"
sudo dnf install gitrevenge
```

The first time, dnf asks you to accept the repository's key. AlmaLinux and Rocky Linux use the same commands.

### The signing key

The repositories and the RPMs in them are signed with one key. Before you trust it, check that its fingerprint is:

```text
03A4 C486 592C CA4D A332  B396 1221 F135 7F3C 5ECA
```

### Without a repository

Each release also has the packages as downloads. Install one with `sudo apt install ./GitRevenge-*.deb` or `sudo dnf install ./GitRevenge-*.rpm`, and update by installing the next release the same way. These downloads aren't signed themselves; check them against the sha256 digest shown beside each one on the release page.

| System | Package |
|---|---|
| Ubuntu, Debian | `GitRevenge-<version>-linux-amd64.deb` |
| RHEL 9 | `GitRevenge-<version>-el9.x86_64.rpm` |
| RHEL 10 | `GitRevenge-<version>-el10.x86_64.rpm` |

### RHEL 10

Red Hat removed WebKitGTK from RHEL 10, so it has to come from [EPEL](https://docs.fedoraproject.org/en-US/epel/). Turn EPEL on before installing Git Revenge. On RHEL:

```sh
sudo subscription-manager repos --enable codeready-builder-for-rhel-10-$(arch)-rpms
sudo dnf install https://dl.fedoraproject.org/pub/epel/epel-release-latest-10.noarch.rpm
```

On AlmaLinux or Rocky Linux, `sudo dnf install epel-release` is enough.

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
