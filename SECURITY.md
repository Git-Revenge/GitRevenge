# Security policy

## Supported versions

Git Revenge is pre-1.0. Only the [latest release](https://github.com/Git-Revenge/GitRevenge/releases/latest) receives security fixes.

## Reporting a vulnerability

Please report vulnerabilities privately through [GitHub's security advisories](https://github.com/Git-Revenge/GitRevenge/security/advisories/new). Don't open a public issue or pull request.

Include the affected version, your operating system, and steps to reproduce. We aim to acknowledge a report within a week and to agree a fix and disclosure timeline with you.

## Scope

Areas where a report matters most:

- Credential handling, including the askpass bridge ([ADR 0004](docs/adr/0004-askpass-bridge.md)) and anything written to disk or logs.
- Running git commands, including how repository content, refs and file names reach the command line.
- Rendering repository content in the UI, such as diffs, commit messages and file previews. Content from a repository must never run scripts.
- Release artifacts and the update check.

Vulnerabilities in git itself or in a dependency should go to that project. Tell us too if Git Revenge exposes users to them.
