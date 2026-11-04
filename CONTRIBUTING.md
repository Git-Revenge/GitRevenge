# Contributing to Git Revenge

Thanks for helping. This page covers how work is organised; [docs/PLAN.md](docs/PLAN.md) has the scope, architecture and decisions, and [docs/adr](docs/adr) records why.

By taking part you agree to follow the [Code of Conduct](CODE_OF_CONDUCT.md). Report security problems privately, as described in [SECURITY.md](SECURITY.md).

## Before you start

1. Look for an existing issue. Each feature area has an epic, and its tasks are sub-issues sized for one PR.
2. For anything not already planned, open an issue first and wait for a reply. Large unplanned PRs are hard to review and may be declined.
3. Check [docs/PLAN.md](docs/PLAN.md) for what is out of scope for v1.

## Workflow

- **One issue, one branch, one PR.** Name the branch `<issue#>-short-slug`. The PR says `Closes #N` and is squash-merged into `main`.
- **Commits** follow [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, `chore:`, `docs:`, …). Release notes are generated from them.
- **Review.** The maintainer reviews feature and bug-fix PRs. Chore PRs may be merged once CI is green.
- **Stacked PRs** are fine. Say the merge order in the description.

## Setting up

See [Development](README.md#development) in the README for toolchain versions and commands. In short:

```sh
cd frontend && npm ci && npm run build && cd ..
go test ./...
cd frontend && npm run check && npm run lint && npm test && npm run test:browser
```

Also run `golangci-lint run` and, with `npx playwright install webkit`, `npm run test:browser:webkit`. CI runs everything on Windows, macOS and Linux.

## Guidelines

- **Tests.** Go tests build fixture repositories on the fly with real git. Frontend logic is covered with Vitest. Anything about how a screen renders or responds is covered in `frontend/e2e` with Playwright, which mounts the real component in a real browser against a stubbed Go backend, so it doesn't need `wails dev`: add a mount to `harness.ts` if the component needs one, then a test. Add a test with each fix or feature.
- **Wails bindings.** `frontend/wailsjs` is generated from the Go API. Commit exactly what `wails generate module` writes, and don't reorder it by hand.
- **Git calls** go through the git layer in `internal/git`, which runs the system git ([ADR 0002](docs/adr/0002-system-git-cli.md)). Don't shell out elsewhere.
- **Match the surrounding code**: naming, comment density and idiom.
- **Accessibility.** UI changes must work with the keyboard and in both themes. See [ACCESSIBILITY.md](ACCESSIBILITY.md).
- **Performance.** The targets are in [docs/PLAN.md](docs/PLAN.md#performance-targets). Say so in the PR if a change could affect them.

## Architecture decisions

A change that alters an architectural decision should add or amend an ADR in [docs/adr](docs/adr) in the same PR.
