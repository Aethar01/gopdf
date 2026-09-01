# gopdf

A minimal, keyboard-driven document viewer backed by MuPDF and configured with Lua.

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

<details open>
<summary>Linux</summary>

Run the AppImage directly:

```bash
chmod +x gopdf-*-linux-x86_64.AppImage
./gopdf-*-linux-x86_64.AppImage file.pdf
```

Arch-based systems can install [gopdf-git from the AUR](https://aur.archlinux.org/packages/gopdf-git):

```bash
yay -S gopdf-git
```

</details>

<details>
<summary>macOS</summary>

Install the release matching Intel or Apple silicon, or use Homebrew:

```bash
brew install Aethar01/gopdf/gopdf
```

</details>

<details>
<summary>Windows</summary>

The release provides an installer with optional PDF file association and a portable zip.

</details>

## Usage

```bash
gopdf /path/to/file.pdf      # open a document
gopdf --goto 20 file.pdf     # start on page 20
gopdf --config custom.lua file.pdf
gopdf --no-config file.pdf   # built-in defaults only
gopdf --no-plugins file.pdf  # skip plugin loading
gopdf -v                     # print version
gopdf -V                     # enable verbose logs
```

Use `F1` to inspect or edit keybindings and `:help` to view available commands.

### Single instance

Instances are per document: each window can be reached by whoever opens the
same file.

```bash
gopdf --unique file.pdf                  # reuse the window showing file.pdf
gopdf --unique --goto 42 file.pdf        # ... and go to page 42
gopdf --unique --goto 42:100:250 f.pdf   # ... to a point on page 42
gopdf --goto 42 file.pdf                 # always a new window, opened at 42
```

`X` and `Y` are points from the page's top-left corner.

With no window showing that document, `--unique` simply opens one, applying any
`--goto` as it starts. A different document is a different instance, so opening
two files gives two windows. The address follows the document, so a window that
switches files becomes reachable under the new one.

Sockets are created mode 0600 under `$XDG_RUNTIME_DIR/gopdf` on Linux, the
per-user temporary directory on macOS, and `%LOCALAPPDATA%\gopdf` on Windows,
and are named by a hash of the document path. One left behind by a crash is
reclaimed automatically.

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
| macOS | `~/.config/gopdf/config.lua` |
| Windows | `%APPDATA%\gopdf\config.lua` |

Interactive keybinding changes are stored in `autogen.lua`. It is loaded before `config.lua`, so explicit user configuration takes precedence.

Session data is stored in `session.sqlite` under the platform application-data directory:

| Platform | Location |
|---|---|
| Linux | `$XDG_DATA_HOME/gopdf` or `~/.local/share/gopdf` |
| macOS | `~/Library/Application Support/gopdf` |
| Windows | `%APPDATA%\gopdf` |

## Documentation

The [documentation site](https://aethar01.github.io/gopdf_docs/) covers:

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
