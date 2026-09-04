# EasyMail

A people-centric email client for GNOME/Wayland, inspired by UniBox.

Uses ActiveSync (EAS) protocol via [go-activesync](https://github.com/remdev/go-activesync) for Exchange/Mailcow connectivity.

## Architecture

- **Backend**: Go (Wails v2) — EAS protocol, SQLite cache, system integration
- **Frontend**: Vue 3 + TypeScript — People-centric 3-pane UI (contacts → conversation → detail)
- **Runtime**: webkit2gtk (native GNOME rendering, no Electron bloat)

## Features

- **People-centric view**: Emails grouped by sender, not folder
- **ActiveSync support**: Connect to Exchange, Office 365, Mailcow (Z-Push)
- **GNOME native**: Uses webkit2gtk, follows system theme (Adwaita dark/light)
- **Wayland ready**: No X11 dependencies
- **Offline capable**: SQLite cache for all synced data
- **Contact search**: Find people fast
- **Attachment views**: Grid and list views (UniBox-style)

## Building

```bash
# Install dependencies
cd frontend && npm install && cd ..

# Build
wails build

# Development
wails dev
```

## EAS Server Compatibility

Tested against:
- Mailcow with Z-Push (mex.btl.io)
- Microsoft Exchange 2016+
- Office 365

## License

MIT