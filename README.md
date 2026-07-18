# gopdf

A minimal, keyboard-driven PDF viewer backed by MuPDF and configured with Lua.

gopdf provides Vim-style navigation, continuous and single-page layouts, dual-page spreads, text search and selection, outlines, links, persistent sessions and marks, configurable colors, commands, and scriptable keybindings without a permanent toolbar.

## Quick Start

```bash
gopdf file.pdf
```

If no file is provided, gopdf reopens the most recently viewed document.

Useful defaults:

| Key | Action |
|---|---|
| `j` / `k` | Scroll down / up |
| `J` / `K` | Next / previous page |
| `/` / `?` | Search forward / backward |
| `n` / `N` | Next / previous match |
| `o` | Open the document outline |
| `gr` | Open recent files |
| `:` | Open the command prompt |
| `F1` | View and edit keybindings |
| `q` | Quit |

## Installation

Download a package for Linux, macOS, or Windows from the [latest release](https://github.com/Aethar01/gopdf/releases/latest).

### Linux

Run the AppImage directly:

```bash
chmod +x gopdf-*-linux-x86_64.AppImage
./gopdf-*-linux-x86_64.AppImage file.pdf
```

Arch-based systems can install [gopdf-git from the AUR](https://aur.archlinux.org/packages/gopdf-git):

```bash
yay -S gopdf-git
```

### macOS

Install the release matching Intel or Apple silicon, or use Homebrew:

```bash
brew install Aethar01/homebrew-gopdf/gopdf
```

### Windows

The release provides an installer with optional PDF file association and a portable zip.

## Usage

```bash
gopdf /path/to/file.pdf      # open a PDF
gopdf --page 20 file.pdf     # start on page 20
gopdf --config custom.lua file.pdf
gopdf -v                     # print version
gopdf -V                     # enable verbose logs
```

Use `F1` to inspect or edit keybindings and `:help` to view available commands.

## Configuration

Configuration is optional and written in Lua. Start with [`config.example.lua`](./config.example.lua), or create a small file containing only the values and bindings you want to change. Reload it with `:reload-config`.

The first existing configuration file for the current platform is loaded:

| Platform | Location |
|---|---|
| Any | Path passed with `--config` |
| Linux | `~/.config/gopdf/config.lua` |
| Linux | `$XDG_CONFIG_HOME/gopdf/config.lua` |
| Linux | Each `$XDG_CONFIG_DIRS/gopdf/config.lua` |
| Linux | `/etc/xdg/gopdf/config.lua` |
| macOS | `~/Library/Application Support/gopdf/config.lua` |
| Windows | `%APPDATA%\gopdf\config.lua` |

Interactive keybinding changes are stored in `autogen.lua`. It is loaded before `config.lua`, so explicit user configuration takes precedence.

Session data is stored in `session.sqlite` under the platform application-data directory:

| Platform | Location |
|---|---|
| Linux | `$XDG_DATA_HOME/gopdf` or `~/.local/share/gopdf` |
| macOS | `~/Library/Application Support/gopdf` |
| Windows | `%APPDATA%\gopdf` |

## Documentation

The [documentation site](https://aethar01.github.io/gopdf/) covers:

- Configuration options and defaults
- Commands and search flags
- Lua functions and tables
- Bindable actions and default keys

The site provides documentation for the current `git` branch and an immutable snapshot for each tagged release. Reference content and the example configuration are generated from the same registrations used by the application:

```bash
go generate ./...
```

## Building

Requirements:

- Go 1.25+
- MuPDF 1.25.6+
- SDL3
- pkg-config/pkgconf
- A C compiler supported by CGO

```bash
go build
go test ./...
```

On Windows, install the dependencies from MSYS2 UCRT64:

```bash
pacman -S --needed mingw-w64-ucrt-x86_64-go mingw-w64-ucrt-x86_64-gcc mingw-w64-ucrt-x86_64-pkgconf mingw-w64-ucrt-x86_64-sdl3 mingw-w64-ucrt-x86_64-mupdf
go build -o gopdf.exe
```

## License

gopdf is licensed under the [AGPL](./LICENSE).

It links against [MuPDF](https://mupdf.com/), which is licensed under the AGPL unless you have a separate commercial license.
