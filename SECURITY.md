# Security policy

## Supported versions

Security fixes go into the newest minor release of the current major version, as a patch release. When 1.1.0 comes out, for example, 1.0.x stops getting fixes, so update to 1.1.x.

| Version | Security fixes |
|---|---|
| Newest 1.x minor release | Yes |
| Older 1.x minor releases | No: update to the newest |
| 0.x | No |

The [latest release](https://github.com/Git-Revenge/GitRevenge/releases/latest) is always supported. Release candidates are for testing and never get fixes of their own.

## Reporting a vulnerability

Please report vulnerabilities privately through [GitHub's security advisories](https://github.com/Git-Revenge/GitRevenge/security/advisories/new). Don't open a public issue or pull request. If you can't use GitHub's form, contact the maintainer, Ris Adams, through the details on [their GitHub profile](https://github.com/risadams) and say it's a security report, without the details.

Include the affected version, your operating system, and steps to reproduce. A proof of concept helps, but isn't required.

## What happens next

1. **Acknowledgement within 7 days**, saying whether we can reproduce it and what we need from you.
2. **A fix**, developed in a private fork on the advisory. We aim to release it within 90 days of the report, sooner for something actively exploited or easy to exploit, and we'll tell you if it will take longer.
3. **Disclosure** when the fix is released: the advisory is published with a CVE where one applies, and the release notes and changelog say which version fixed it. We agree the date with you, and if 90 days pass without a fix, you're free to disclose.
4. **Credit** in the advisory, under the name you choose, unless you'd rather not be named.

Git Revenge is maintained by volunteers and has no bug bounty.

## Scope

Areas where a report matters most:

- **Credentials**, including the askpass bridge ([ADR 0004](docs/adr/0004-askpass-bridge.md)) and anything written to disk or logs.
- **Running git**, including how repository content, refs, file names and config reach the command line.
- **Repository content in the UI**: diffs, commit messages, notes, and the rendered previews of Markdown, HTML, notebooks, AsciiDoc, reStructuredText, PDFs, diagrams, GeoJSON, audio and video. Content from a repository must never run scripts, and a preview must never fetch anything from the network.
- **What we ship**: the release artifacts, the update check, the apt and dnf repositories at packages.gitrevenge.com ([Git-Revenge/packages](https://github.com/Git-Revenge/packages)), and the Homebrew tap ([Git-Revenge/homebrew-tap](https://github.com/Git-Revenge/homebrew-tap)).
- **The website**, gitrevenge.com.

Out of scope:

- Vulnerabilities in git, WebView2, WebKit or a dependency. Report those to that project, but tell us too if Git Revenge exposes users to them or should work around them.
- Anything that needs an attacker to already control your account or your machine.
- The Windows installer being unsigned. SmartScreen's warning is known and expected.

## Verifying what you download

- **Every release asset** has a sha256 digest on the release page. Releases are immutable: once published, an asset or tag can't be replaced.
- **The macOS disk image** is signed with a Developer ID certificate and notarized by Apple.
- **The apt and dnf repositories** sign their metadata, and the dnf repositories sign each RPM, with the key whose fingerprint is `03A4 C486 592C CA4D A332  B396 1221 F135 7F3C 5ECA`.
- **The Homebrew cask** pins the disk image's sha256.
