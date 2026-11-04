# Installing

Download the Windows installer or the macOS disk image from the [latest release](https://github.com/Git-Revenge/GitRevenge/releases/latest).

- **macOS:** the disk image is signed and notarized, so it opens like any other app.
- **Windows:** the installer isn't signed yet, so SmartScreen warns you before it runs. Choose **More info**, then **Run anyway**.

When a new release comes out, the app tells you at startup. You can also check by hand with **Check for updates** in the command palette.

## What you need

- **git 2.40 or newer.** Git Revenge doesn't bundle git. It finds the one on your `PATH`, or you can point it at a specific one in [Settings](settings.md).
- **git-lfs**, if you want the [LFS features](remotes.md#lfs).

Check your version with:

```sh
git --version
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

Backups of git config files you save in the [Git Config tab](git-config.md) go in a `config-backups` folder beside it. Nothing about your repositories is stored anywhere else. If the file is ever damaged, the app moves it aside as `settings.json.corrupt-<timestamp>` and starts with defaults.
