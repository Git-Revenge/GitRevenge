# 0001 — Wails desktop shell

- Status: Accepted. Linux became first-class in [ADR-0010](0010-linux-first-class.md), and it uses WebKitGTK.
- Date: 2026-10-28

## Context

Git Revenge has a Go backend and must run as a desktop app on Windows and macOS, both of which are first-class. Linux is a nice-to-have. The options were a local server with a browser UI, Wails, or Electron/Tauri with a Go sidecar.

## Decision

Use **Wails v2**. Go exposes a typed API through Wails bindings and pushes updates to the UI as events. The UI runs in the system WebView: WebView2 on Windows and WKWebView on macOS.

## Consequences

- Each platform ships a single native binary with a small footprint compared to Electron, plus native dialogs and windowing.
- We need per-OS builds in CI. macOS builds also need signing and notarization.
- The Go API stays transport-agnostic. A browser mode could be added later as an HTTP adapter over the same API.
