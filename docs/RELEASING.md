# Releasing

Releases are built by [`.github/workflows/release.yml`](../.github/workflows/release.yml) whenever a semver tag is pushed.

## Cutting a release

1. Make sure `main` is green and the milestone's issues are closed.
2. Run the benchmark suite against a large repo (see [PLAN.md](PLAN.md#performance-targets)).
3. Update [CHANGELOG.md](../CHANGELOG.md) in a PR. Rename `## [Unreleased]` to `## [X.Y.Z] - YYYY-MM-DD` (the day you'll tag), add a fresh empty `## [Unreleased]` above it, and update the links at the bottom: `[Unreleased]` compares `vX.Y.Z...HEAD`, and `[X.Y.Z]` compares the previous release with `vX.Y.Z`. Release candidates don't get their own section: their changes stay under Unreleased until the final release.
4. Tag and push:

   ```sh
   git tag -a v0.1.0 -m "v0.1.0"
   git push origin v0.1.0
   ```

   A tag with a pre-release suffix, such as `v0.2.0-rc.1`, is published as a GitHub pre-release.

The workflow then does the following:

1. Stamps the tag's version into the binary (`internal/version.Version`) and into `wails.json`.
2. Builds the Windows NSIS installer. It is unsigned, so SmartScreen will warn.
3. Builds a universal macOS `.dmg`. When the Apple secrets are configured, it also signs, notarizes and staples it.
4. Creates the GitHub Release, with notes generated from merged PRs. CHANGELOG.md is the curated record; the generated notes list every PR.
5. For a stable release, tells [Git-Revenge/packages](https://github.com/Git-Revenge/packages) (the apt and dnf repos at packages.gitrevenge.com) and [Git-Revenge/homebrew-tap](https://github.com/Git-Revenge/homebrew-tap) to update. Both also check every six hours, so a missed notice only delays them.
6. For a stable release, opens a pull request on [microsoft/winget-pkgs](https://github.com/microsoft/winget-pkgs) to update `GitRevenge.GitRevenge`. Microsoft's checks and reviewers take it from there, usually within a day or two.

## Keeping the changelog

Every PR that changes what people see or do adds a line to `## [Unreleased]` in [CHANGELOG.md](../CHANGELOG.md), under the type it is ([Keep a Changelog 2.0.0](https://keepachangelog.com/en/2.0.0/)): **Added** for new features, **Changed** for changes to how something already works, **Deprecated**, **Removed**, **Fixed** for bugs, **Security** for vulnerabilities. Mark a breaking change with **Breaking:** at the start of its line. Write for people using the app, not as a commit log, and end the line with the issue and PR numbers. Chores, CI, tests and dependency bumps nobody notices don't get a line.

## Dry runs

PRs that change `release.yml` run the build and packaging steps without signing or publishing. The resulting artifacts are attached to the workflow run.

## macOS signing secrets

Set these in **Settings → Secrets and variables → Actions**. If `APPLE_CERTIFICATE_P12` or `APPLE_API_KEY` is missing, the macOS build is published unsigned.

Signing uses a Developer ID Application certificate. Notarization uses an App Store Connect API key: in App Store Connect, **Users and Access → Integrations → App Store Connect API → Team Keys**, generate a key with the **Developer** role and download its `.p8` file (it can be downloaded only once).

| Secret | Value |
|---|---|
| `APPLE_CERTIFICATE_P12` | Your "Developer ID Application" certificate and private key, exported as `.p12` and base64-encoded (`base64 -i cert.p12 \| pbcopy`) |
| `APPLE_CERTIFICATE_PASSWORD` | The password used when exporting the `.p12` |
| `APPLE_SIGNING_IDENTITY` | For example `Developer ID Application: Your Name (TEAMID)` |
| `APPLE_API_KEY` | The full text of the `AuthKey_<KEY ID>.p8` file, including the `BEGIN` and `END` lines |
| `APPLE_API_KEY_ID` | The key's Key ID, shown in its row on the Team Keys page |
| `APPLE_API_ISSUER` | The Issuer ID, shown above the Team Keys table |

## Package repo token

`PACKAGE_REPOS_TOKEN` lets the release notify the package repos. Without it that step is skipped. Create a fine-grained personal access token with **Git-Revenge** as the resource owner, access to only `packages` and `homebrew-tap`, and the **Contents: Read and write** permission. Store it as a repository secret here. When it expires, the release still publishes and the step warns.

## winget

The package is `GitRevenge.GitRevenge`. Its manifest templates are in `build/windows/winget/`, and `build/windows/winget/manifests.sh VERSION OUTDIR` fills them in for a published release, taking the installer's sha256 from the release. `winget validate --manifest <folder>` checks the result.

**The first version is submitted by hand,** because the release job can only update a package winget already has. Fork [microsoft/winget-pkgs](https://github.com/microsoft/winget-pkgs) to your account, then, once the release is published:

```sh
git clone https://github.com/<you>/winget-pkgs && cd winget-pkgs
bash /path/to/GitRevenge/build/windows/winget/manifests.sh 1.0.0 .
git switch -c GitRevenge.GitRevenge-1.0.0 && git add manifests/g/GitRevenge && git commit -m "New package: GitRevenge.GitRevenge version 1.0.0"
git push -u origin HEAD && gh pr create --repo microsoft/winget-pkgs --fill
```

Fill in the pull request template as it asks. After that, each release updates it.

**`WINGET_TOKEN`** lets the release do that. It must be a **classic** personal access token with the `public_repo` scope, from the account that owns the winget-pkgs fork (the job's `fork-user`), as the winget-releaser action asks. Without it, or when the update fails, the release still publishes, and you can submit by hand as above.
