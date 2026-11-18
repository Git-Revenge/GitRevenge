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
2. Builds the Windows NSIS installer. When the Certum secrets are configured, it signs the app, the uninstaller and the installer.
3. Builds a universal macOS `.dmg`. When the Apple secrets are configured, it also signs, notarizes and staples it.
4. Attests each file's build provenance: a signed statement that this workflow built it from the tag's commit, which anyone can check with `gh attestation verify <file> --repo Git-Revenge/GitRevenge`. A file that can't be attested stops the release before it's published.
5. Creates the GitHub Release, with notes generated from merged PRs. CHANGELOG.md is the curated record; the generated notes list every PR.
6. For a stable release, tells [Git-Revenge/packages](https://github.com/Git-Revenge/packages) (the apt and dnf repos at packages.gitrevenge.com) and [Git-Revenge/homebrew-tap](https://github.com/Git-Revenge/homebrew-tap) to update. Both also check every six hours, so a missed notice only delays them.
7. For a stable release, opens a pull request on [microsoft/winget-pkgs](https://github.com/microsoft/winget-pkgs) to update `GitRevenge.GitRevenge`. Microsoft's checks and reviewers take it from there, usually within a day or two.

## Keeping the changelog

Every PR that changes what people see or do adds a line to `## [Unreleased]` in [CHANGELOG.md](../CHANGELOG.md), under the type it is ([Keep a Changelog 2.0.0](https://keepachangelog.com/en/2.0.0/)): **Added** for new features, **Changed** for changes to how something already works, **Deprecated**, **Removed**, **Fixed** for bugs, **Security** for vulnerabilities. Mark a breaking change with **Breaking:** at the start of its line. Write for people using the app, not as a commit log, and end the line with the issue and PR numbers. Chores, CI, tests and dependency bumps nobody notices don't get a line.

## Dry runs

PRs that change `release.yml` run the build and packaging steps without signing or publishing. The resulting artifacts are attached to the workflow run.

To test Windows signing without a tag, run the workflow by hand: **Actions → Release → Run workflow**, or `gh workflow run release.yml --ref main -f sign=true`. It builds everything, signs the Windows installer, and attaches the artifacts to the run, versioned `0.0.0-dryrun`. It never publishes a release: that takes a tag.

## Windows signing secrets

The Windows installer is signed with a Certum open source code signing certificate, which lives in Certum's cloud (SimplySign), not in a file. The workflow installs SimplySign Desktop on the runner and signs in with a one-time code it makes from the account's TOTP secret, using [jay0lee/certum-cloud-code-sign](https://github.com/jay0lee/certum-cloud-code-sign), pinned to a commit. The app, the uninstaller and the installer are signed and timestamped through `build/windows/installer/sign.cmd`, which does nothing in local and PR builds. If `CERTUM_USERNAME` or `CERTUM_TOTP_SECRET` is missing, the installer is published unsigned.

| Secret | Value |
|---|---|
| `CERTUM_USERNAME` | The e-mail address you sign in to SimplySign with |
| `CERTUM_TOTP_SECRET` | The secret behind the SimplySign one-time codes, in Base32: the `secret=` value of the `otpauth://` link in the QR code Certum gave you to activate the SimplySign mobile app |
| `CERTUM_CERT_SHA1` | Optional. The certificate's SHA-1 thumbprint, so the workflow signs only with that certificate. Without it, the code signing certificate SimplySign offers is used |

The TOTP secret, with the e-mail address, is enough to sign as the project for as long as the certificate lasts. If it may have leaked, generate a new QR code in SimplySign, which makes the old secret useless, and update the secret here.

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

The package has been in winget since 1.0.0 ([microsoft/winget-pkgs#481763](https://github.com/microsoft/winget-pkgs/pull/481763)), and each stable release updates it. A new version shows up in `winget` a few hours after its pull request merges.

**To submit a version by hand,** when the release job's update fails, use a fork of [microsoft/winget-pkgs](https://github.com/microsoft/winget-pkgs) once the release is published:

```sh
git clone --depth 1 --filter=blob:none --sparse https://github.com/<you>/winget-pkgs && cd winget-pkgs
git sparse-checkout set manifests/g/GitRevenge
bash /path/to/GitRevenge/build/windows/winget/manifests.sh X.Y.Z .
git switch -c GitRevenge.GitRevenge-X.Y.Z && git add manifests/g/GitRevenge && git commit -m "Update: GitRevenge.GitRevenge to X.Y.Z"
git push -u origin HEAD && gh pr create --repo microsoft/winget-pkgs --fill
```

Fill in the pull request template as it asks. Microsoft's bot checks every URL in the manifests and rejects one that doesn't return 200, so check the links in `build/windows/winget/` when the website moves.

**`WINGET_TOKEN`** lets the release do that. It must be a **classic** personal access token with the `public_repo` scope, from the account that owns the winget-pkgs fork (the job's `fork-user`), as the winget-releaser action asks. Without it, or when the update fails, the release still publishes, and you can submit by hand as above.
