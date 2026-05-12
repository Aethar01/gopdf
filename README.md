# gopdf

MuPDF-backend PDF viewer written in Go with Lua configuration.

## Requirements

- [libmupdf](https://mupdf.com/)
- [SDL2](https://libsdl.org/)

## Installation

```bash
go build
```

## Usage

```bash
gopdf /path/to/file.pdf      # open file
gopdf --page 20 file.pdf     # start at page 20
gopdf --config custom.lua file.pdf
gopdf -v                     # print version
```

If no file is provided, gopdf reopens the last viewed file when one is saved in its state file.

## Configuration

Start from [`config.example.lua`](./config.example.lua). Config is Lua, loaded once at startup and again when `reload_config` or `:reload-config` is used.

### Config File Locations

The first existing file in this order is used:

| Priority | Path |
|----------|------|
| 1 | `--config <path>` argument |
| 2 | `~/.config/gopdf/config.lua` |
| 3 | `$XDG_CONFIG_HOME/gopdf/config.lua` |
| 4 | Each `$XDG_CONFIG_DIRS/gopdf/config.lua` |
| 5 | `/etc/xdg/gopdf/config.lua` |

### Options

```lua
gopdf.options.status_bar_visible = true
gopdf.options.mouse_text_select = true
gopdf.options.natural_scroll = false
gopdf.options.anti_aliasing = 8                -- 0 disables AA; MuPDF clamps values to 0-8
gopdf.options.alt_colors = false
gopdf.options.render_oversample = 1            -- >1 supersamples, <1 undersamples

gopdf.options.render_mode = "continuous"       -- "continuous" or "single"
gopdf.options.dual_page = false
gopdf.options.first_page_offset = true
gopdf.options.fit_mode = "page"                 -- "page", "width", or "manual"

gopdf.options.page_gap = 0                      -- sets vertical gap too
gopdf.options.spread_gap = 0                    -- sets horizontal gap too
gopdf.options.page_gap_vertical = 0
gopdf.options.page_gap_horizontal = 0

gopdf.options.status_bar_height = 28
gopdf.options.ui_font_size = 14
gopdf.options.ui_font_path = ""                 -- empty = default font
gopdf.options.sequence_timeout_ms = 700

gopdf.options.outline_initial_depth = 1
gopdf.options.outline_width_percent = 70
gopdf.options.outline_height_percent = 80
gopdf.options.completion_max_items = 10

-- Colors are { red, green, blue }, 0-255.
gopdf.options.background = { 255, 255, 255 }
gopdf.options.page_background = { 255, 255, 255 }
gopdf.options.foreground = { 17, 17, 17 }
gopdf.options.status_bar_color = { 17, 17, 17 }
gopdf.options.alt_background = { 17, 17, 17 }
gopdf.options.alt_page_background = { 17, 17, 17 }
gopdf.options.alt_foreground = { 255, 255, 255 }
gopdf.options.alt_status_bar_color = { 17, 17, 17 }
gopdf.options.highlight_foreground = { 0, 0, 0 }
gopdf.options.highlight_background = { 255, 224, 102 }
```

### Status Bar

```lua
gopdf.status_bar.height = 28
gopdf.status_bar.left = "{message}"
gopdf.status_bar.right = "{page}/{total} {mode} fit={fit} rot={rot} {zoom}"
```

Available placeholders:

| Placeholder | Description |
|-------------|-------------|
| `{message}` | Current status message or input prompt |
| `{page}` | Current page, or current spread range in dual-page mode |
| `{total}` | Total pages |
| `{mode}` | Render mode: continuous or single |
| `{fit}` | Fit mode: page, width, or manual |
| `{rot}` | Rotation in degrees |
| `{zoom}` | Zoom percentage |
| `{dual}` | `dual` or `single` |
| `{cover}` | `cover` or `flat` |
| `{search}` | Search match counter |
| `{document}` | Document filename |
| `{input}` | Current input text |
| `{prompt}` | Search prompt, `/` or `?` |

Use `$$` for a literal `$` in status bar templates.

Example:

```lua
gopdf.status_bar.height = 32
gopdf.options.ui_font_size = 13
gopdf.options.ui_font_path = "/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf"
gopdf.status_bar.left = " {document} {message}"
gopdf.status_bar.right = " {page}/{total} | {mode} | {search} | {zoom} "
```

## Keybindings

Use `gopdf.bind(key, action)` and `gopdf.bind_mouse(event, action)`. The global aliases `bind`, `unbind`, `bind_mouse`, and `unbind_mouse` are also available in config files.

```lua
gopdf.bind("j", gopdf.scroll_down)
gopdf.bind("J", gopdf.next_page)
gopdf.bind("gg", gopdf.first_page)
gopdf.bind("<C-o>", gopdf.jump_backward)
gopdf.bind("<Space>", gopdf.pan)       -- hold Space and move mouse to pan

gopdf.bind_mouse("wheel_down", gopdf.scroll_down)
gopdf.bind_mouse("<C-wheel_up>", gopdf.zoom_in)
gopdf.bind_mouse("middle_down", gopdf.pan)
```

Custom callbacks run after the viewer is active:

```lua
gopdf.bind("H", function()
  gopdf.goto_page(1)
  gopdf.message("first page")
end)

gopdf.bind("<C-l>", function()
  gopdf.command(":reload-config")
end)
```

Unbind keys or mouse events:

```lua
gopdf.unbind("j")
gopdf.unbind_mouse("wheel_down")
```

### Supported Keys

Key names are case-sensitive for printable letters and normalized for angle-bracket names.

| Form | Examples |
|------|----------|
| Printable letters | `a` through `z`, `A` through `Z` |
| Printable digits | `0` through `9` |
| Printable punctuation | `/`, `?`, `;`, `:`, `=`, `+`, `-` |
| Space | `" "` or `<Space>` |
| Special keys | `<CR>`, `<Enter>`, `<Return>`, `<Esc>`, `<BS>`, `<PgDn>`, `<PgUp>`, `<Tab>` |
| Ctrl keys | `<C-a>`, `<C-S-a>`, `<C-1>`, `<C-S-1>`, `<C-Space>`, `<C-Tab>`, `<C-Enter>`, `<C-Esc>`, `<C-BS>`, `<C-PgDn>`, `<C-PgUp>` |
| Shift special keys | `<S-CR>`, `<S-Esc>`, `<S-BS>`, `<S-PgDn>`, `<S-PgUp>`, `<S-Tab>` |
| Sequences | `gg`, `tb`, `co`, `<C-x>g` |

Supported mouse events:

| Event | Description |
|-------|-------------|
| `wheel_up`, `wheel_down` | Vertical wheel scroll |
| `wheel_left`, `wheel_right` | Horizontal wheel scroll |
| `<C-wheel_up>`, `<C-wheel_down>` | Ctrl-wheel events |
| `left_down`, `left_up` | Left mouse button |
| `middle_down`, `middle_up` | Middle mouse button |
| `right_down`, `right_up` | Right mouse button |
| `x1_down`, `x1_up` | Extra mouse button 1 |
| `x2_down`, `x2_up` | Extra mouse button 2 |

### Default Keybindings

| Key | Action |
|-----|--------|
| `j` / `k` | Scroll down / up |
| `h` / `l` | Scroll left / right |
| `J` / `K` | Next / previous page |
| `Space` / `<PgDn>` / `<PgUp>` | Next page / next page / previous page |
| `gg` / `G` | First / last page |
| `g` | Page prompt |
| `Ng` | Jump to page N |
| `Nj`, `Nk`, `Nh`, `Nl` | Repeat scroll action N times |
| `NJ` / `NK` | Jump N pages/spreads forward / backward |
| `d` | Toggle dual-page mode |
| `m` | Toggle continuous/single render mode |
| `tb` | Toggle alternate colors |
| `co` | Toggle first-page offset |
| `s` | Toggle status bar |
| `f` | Toggle fullscreen |
| `o` | Open/close outline menu |
| `<CR>` | Confirm input or selected outline item |
| `+` / `=` / `-` / `0` | Zoom in / zoom in / zoom out / reset zoom |
| `w` / `z` | Fit width / fit page |
| `r` / `R` | Rotate clockwise / counter-clockwise |
| `/` / `?` | Search forward / backward |
| `n` / `N` | Next / previous search match |
| `:` | Command prompt |
| `<Tab>` / `<S-Tab>` | Show or cycle command completion / previous completion |
| `<Esc>` | Close active UI, clear search, or clear pending keys/count |
| `<C-i>` / `<C-o>` | Jump forward / backward in jump history |
| `q` | Quit |

Default mouse bindings:

| Mouse | Action |
|-------|--------|
| `wheel_up` / `wheel_down` | Scroll up / down |
| `wheel_left` / `wheel_right` | Scroll left / right |
| `<C-wheel_up>` / `<C-wheel_down>` | Zoom in / out |
| `middle_down` | Pan while held |
| Left-drag | Text selection when `mouse_text_select` is true |

## Commands

| Command | Description |
|---------|-------------|
| `:page N`, `:p N`, `:N` | Jump to page N |
| `:search <text>` | Search document |
| `:fit width` / `:fit page` / `:fit manual` | Set fit mode |
| `:mode continuous` / `:mode single` | Set render mode |
| `:colors normal` / `:colors alt` | Set color mode |
| `:set dual_page!` | Toggle dual-page mode |
| `:set alt_colors!` | Toggle alternate colors |
| `:set render_mode!` | Toggle render mode |
| `:set first_page_offset!` | Toggle first-page offset |
| `:set status_bar!` | Toggle status bar |
| `:open <filename>` | Open another PDF, relative to the current document directory |
| `:reload-config` | Reload config file |
| `:help` | Show command help in the status bar |
| `:quit`, `:q` | Exit |

## Lua API

### Viewer State

| Function | Description |
|----------|-------------|
| `gopdf.page()` | Current page number, 1-indexed |
| `gopdf.page_count()` | Total pages |
| `gopdf.goto_page(n)` | Jump to page n |
| `gopdf.mode()` | Current UI mode |
| `gopdf.current_count()` | Pending numeric count |
| `gopdf.pending_keys()` | Pending key sequence table |
| `gopdf.clear_pending_keys()` | Clear pending keys and count |

### Display

| Function | Description |
|----------|-------------|
| `gopdf.fit_mode()` / `gopdf.set_fit_mode("width"\|"page"\|"manual")` | Get/set fit mode |
| `gopdf.render_mode()` / `gopdf.set_render_mode("continuous"\|"single")` | Get/set render mode |
| `gopdf.zoom()` / `gopdf.set_zoom(n)` | Get/set zoom scale |
| `gopdf.rotation()` / `gopdf.set_rotation(deg)` | Get/set rotation |
| `gopdf.fullscreen()` / `gopdf.set_fullscreen(bool)` | Get/set fullscreen |
| `gopdf.status_bar_visible()` / `gopdf.set_status_bar_visible(bool)` | Get/set status bar visibility |

### Search

| Function | Description |
|----------|-------------|
| `gopdf.search(query[, backward])` | Search document |
| `gopdf.search_query()` | Current search term |
| `gopdf.search_match_index()` | Current match, 1-indexed, or nil |
| `gopdf.search_match_count()` | Total matches |

### Utilities

| Function | Description |
|----------|-------------|
| `gopdf.message()` / `gopdf.message("text")` | Get/set status message |
| `gopdf.command(":fit width")` | Execute command |
| `gopdf.open(path)` | Open another PDF |
| `gopdf.bind(key, action)` / `gopdf.unbind(key)` | Bind/unbind keyboard action |
| `gopdf.bind_mouse(event, action)` / `gopdf.unbind_mouse(event)` | Bind/unbind mouse action |
| `gopdf.set(name, value)` | Set config option |

### Custom UI

Lua callbacks can open a simple modal list overlay. The overlay uses the same navigation actions as the outline menu: `scroll_down`, `scroll_up`, `confirm`, and `close`.

| Function | Description |
|----------|-------------|
| `gopdf.ui.show(spec)` | Show a modal list UI |
| `gopdf.ui.menu(spec)` | Alias for `gopdf.ui.show(spec)` |
| `gopdf.ui.close()` | Close the active Lua UI without running `on_close` |
| `gopdf.ui.visible()` | Return whether a Lua UI is visible |
| `gopdf.ui.set_rows(rows)` | Replace the current UI rows |
| `gopdf.ui.set_selected(index)` | Set the selected row, 1-indexed |

`show` and `menu` accept this table:

| Field | Description |
|-------|-------------|
| `title` | Optional title shown in the header |
| `rows` | Array of strings to show |
| `selected` | Optional initial selected row, 1-indexed |
| `on_select(index, value)` | Optional callback run when a row is confirmed or clicked |
| `on_close()` | Optional callback run when the UI is closed by the viewer |

Example file browser bound to `fo`. It starts in the user's home directory, shows directories first, lets you open `..`, and opens selected PDFs:

```lua
local function shell_quote(s)
  return "'" .. s:gsub("'", "'\\''") .. "'"
end

local function list_dir(dir)
  local rows = {}
  local command = "find " .. shell_quote(dir) .. " -maxdepth 1 -mindepth 1 " ..
    '\\( -type d -printf "d:%f\\n" -o -type f -iname "*.pdf" -printf "f:%f\\n" \\)'
  local handle = io.popen(command)
  if not handle then
    return rows
  end
  for line in handle:lines() do
    local kind, name = line:match("^([df]):(.*)$")
    if kind == "d" then
      rows[#rows + 1] = name .. "/"
    elseif kind == "f" then
      rows[#rows + 1] = name
    end
  end
  handle:close()
  table.sort(rows)
  table.insert(rows, 1, "../")
  return rows
end

local function join_path(dir, name)
  if dir == "/" then
    return "/" .. name
  end
  return dir .. "/" .. name
end

local function parent_dir(dir)
  if dir == "/" then
    return "/"
  end
  return dir:match("^(.*)/[^/]+/?$") or "/"
end

local function show_file_browser(dir)
  gopdf.ui.menu({
    title = "Open PDF: " .. dir,
    rows = list_dir(dir),
    on_select = function(_, name)
      if name == "../" then
        show_file_browser(parent_dir(dir))
        return
      end

      if name:sub(-1) == "/" then
        show_file_browser(join_path(dir, name:sub(1, -2)))
        return
      end

      gopdf.ui.close()
      gopdf.open(dir .. "/" .. name)
    end,
  })
end

gopdf.bind("fo", function()
  show_file_browser(os.getenv("HOME") or ".")
end)
```

### Actions

Actions can be bound directly, called from callbacks, or executed with `gopdf.command` where a matching command exists.

```lua
gopdf.next_page()        gopdf.prev_page()
gopdf.scroll_down()      gopdf.scroll_up()
gopdf.scroll_left()      gopdf.scroll_right()
gopdf.next_spread()      gopdf.prev_spread()
gopdf.first_page()       gopdf.last_page()
gopdf.command_mode()     gopdf.goto_page_prompt()
gopdf.search_prompt()    gopdf.search_prompt_backward()
gopdf.search_next()      gopdf.search_prev()
gopdf.clear_search()     gopdf.close()
gopdf.toggle_dual_page() gopdf.toggle_render_mode()
gopdf.toggle_alt_colors()
gopdf.toggle_first_page_offset()
gopdf.toggle_status_bar()
gopdf.toggle_fullscreen()
gopdf.outline()          gopdf.confirm()
gopdf.zoom_in()          gopdf.zoom_out()
gopdf.reset_zoom()
gopdf.fit_width()        gopdf.fit_page()
gopdf.rotate_cw()        gopdf.rotate_ccw()
gopdf.jump_forward()     gopdf.jump_backward()
gopdf.pan()              gopdf.reload_config()
gopdf.quit()
```

### Cache

```lua
gopdf.cache.entries()       -- rendered pages in cache
gopdf.cache.pending()       -- pending renders
gopdf.cache.limit()         -- current cache entry limit
gopdf.cache.set_limit(n)    -- set cache entry limit
gopdf.cache.clear()         -- clear rendered page cache
```

### Document Metadata

Available during config load:

| Property | Description |
|----------|-------------|
| `gopdf.document.name` | Filename |
| `gopdf.document.path` | Full path |
| `gopdf.document.extension` | File extension |
| `gopdf.document.page_count` | Total pages, if readable |
| `gopdf.document.size_bytes` | File size, if the file exists |
| `gopdf.document.exists` | Whether the file exists |

Example:

```lua
if gopdf.document.page_count and gopdf.document.page_count > 200 then
  gopdf.options.dual_page = true
end
```

## License

gopdf is licensed under [AGPL](./LICENSE).

Links against [MuPDF](https://mupdf.com/), which is licensed under AGPL unless you have a separate commercial license.
