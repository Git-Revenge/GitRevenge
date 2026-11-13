# 0010 — Linux is first-class

- Status: Accepted
- Date: 2026-11-04

## Context

Linux started as a nice-to-have (ADR-0001). Making it first-class raised questions the other platforms don't have: which systems to support when they ship different WebKitGTK versions, how to package an app that relies on the user's own `git` (ADR-0002), and how to render when hardware acceleration can fail outright.

We checked the distributions on 2026-11-04:

| System | `git` | WebKitGTK | glibc |
|---|---|---|---|
| Ubuntu 22.04 | 2.34 | 4.1 | 2.35 |
| Ubuntu 24.04 | 2.43 | 4.1 | 2.39 |
| Debian 12 | 2.39 | 4.1 | 2.36 |
| Debian 13 | 2.47 | 4.1 | 2.41 |
| RHEL 9 | 2.52 | 4.0 only | 2.34 |
| RHEL 10 | 2.52 | Removed by Red Hat; 4.1 from EPEL | 2.39 |

## Decision

- **Support Ubuntu 24.04, Debian 13, RHEL 9, and RHEL 10 with EPEL,** and anything newer, on x86-64. These are the oldest releases of each whose own `git` meets our 2.40 minimum.
- **Ship a `.deb` and two `.rpm`s.** RHEL 9 and 10 use different WebKitGTK versions, so they can't share a package. The `.deb` is built on Ubuntu 24.04 and also serves Debian 13.
- **Leave out Flatpak, Snap and AppImage for now.** Sandboxes conflict with using the system `git`, SSH setup and credential helpers. AppImage can't readily bundle WebKit.
- **Render in software** (`WebviewGpuPolicyNever`). Hardware acceleration can leave a blank window on some NVIDIA and DMABUF setups, and a blank window is worse than a slower graph. On the Linux kernel repository with a commit-graph file, on a 16-thread i9-11900KF, steady scrolling held 55–61 fps.
- **Test what we ship.** CI runs the Go suite on Ubuntu and builds for EL9 and EL10. A release installs each package on each supported system and starts the app before it publishes. AlmaLinux stands in for RHEL.

## Consequences

- RHEL 10 support depends on a community repository, and EPEL's WebKitGTK can lag behind other distributions.
- Packages are unsigned, and there's no apt or dnf repository, so people update by downloading a new package. The app says when one is out. *Amended 2026-11-13 (#704): signed apt and dnf repositories at packages.gitrevenge.com now carry each stable release, and sign the RPMs they serve. The downloads on a release are still unsigned.*
- inotify takes one watch per directory, so a very large tree can hit the system's limit. The app says so when it happens at open. A limit reached later, such as by a new folder, isn't reported.
- Graph performance hasn't been measured on slower machines or real Linux desktops, and the blank-window risk is untested here. Revisit the rendering choice when it can be.
