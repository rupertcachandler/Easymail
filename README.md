# EasyMail (BOI) - A full Activesync client for Debian based OS's --**BETA**--

A people-centric email client that organizes your email by **people**, not
folders. Speaks Microsoft's Exchange ActiveSync (EAS) protocol directly —
no IMAP, no Electron. Built with Go, Wails v2 and Vue 3.

![Runtime](frontend/dist/assets/*.js)

## Features

- **People-centric view** — emails grouped by sender, not folder
- **ActiveSync (EAS)** — direct connection to Exchange, Office 365 and
  Mailcow / Z-Push servers
- **Native desktop** — WebKitGTK rendering (no Electron bloat), follows the
  system light/dark theme
- **Offline capable** — SQLite cache for all synced data
- **Built-in calendar** — day / week / month views with EAS sync
- **Contact search, attachments, signatures, multi-account**

## Architecture

- **Backend**: Go (Wails v2) — EAS protocol, SQLite cache, system integration
- **Frontend**: Vue 3 + TypeScript — 3-pane people-centric UI
- **Runtime**: `webkit2gtk` (native GTK rendering)

## Building

Requirements: Go 1.26+, Node.js, and the WebKitGTK `-dev` packages for your
distribution.

```bash
# Frontend dependencies
cd frontend && npm install && cd ..

# Production build
make build

# Development with hot reload
make dev
```

The EAS client is bundled in-repo at `go-activesync-local/` (a vendored copy
of [go-activesync](https://github.com/remdev/go-activesync), MIT) and wired
in via a `replace` directive.

## EAS Server Compatibility

ActiveSync is an open standard and works with any compliant server. Common
targets: Microsoft Exchange 2016+, Office 365, and Mailcow with the Z-Push
ActiveSync frontend.

## Packaging

`make deb` builds a Debian package (`easymail_<ver>_<arch>.deb`) installable
on any Debian-family desktop (GNOME, KDE, XFCE, Cinnamon, MATE) on
`amd64`. Runtime dependencies: `libwebkit2gtk-4.1-0`, `libgtk-3-0`,
`libsoup-3.0-0`, `libjavascriptcoregtk-4.1-0`, `libsqlite3-0`.

## License

[MIT](LICENSE) © EasyMail (BOI)

Third-party components retain their own licenses; see `go.activesync-local/LICENSE`
(BSD/MIT) and the module licenses in `go.sum`.
