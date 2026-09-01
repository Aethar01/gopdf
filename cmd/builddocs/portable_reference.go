package main

import (
	"fmt"
	"strings"
)

type apiEntry struct {
	name        string
	result      string
	description string
}

var portableModules = []apiEntry{
	{"gopdf.platform", "table", "Also available from `require(\"gopdf.platform\")`. Fields: `os` (`linux`, `macos`, or `windows`), `arch`, `home_dir`, `config_dir`, `data_dir`, `cache_dir`, and `temp_dir`. Directory fields are absolute when the platform supplies a location and may be empty when it does not."},
	{"gopdf.path", "table", "Also available from `require(\"gopdf.path\")`. Fields: `separator`, `list_separator`; functions: `join(...)`, `clean(path)`, `basename(path)`, `dirname(path)`, `extension(path)`, `is_absolute(path)`, `absolute(path)`, `relative(base, target)`, and `expand_home(path)`. Operations use native platform path rules; `expand_home` expands only `~`, `~/...`, and `~\\...`."},
	{"gopdf.json", "table", "Also available from `require(\"gopdf.json\")`. `encode(value)` returns compact JSON and `decode(text)` returns a Lua value. `null` is the stable module-local JSON null sentinel. Arrays require contiguous positive integer keys; objects require string keys. Cycles, sparse or mixed tables, non-finite numbers, unsupported Lua values, and trailing input are errors. Decoded empty arrays and objects retain their shape when re-encoded."},
}

var hostFunctions = []apiEntry{
	{"gopdf.schedule(callback)", "handle", "Queue `callback()` on the main Lua thread after the current dispatch. It is asynchronous and can be cancelled through the returned handle."},
	{"gopdf.log(level, message)", "none", "Write a diagnostic without changing the viewer message. `level` must be `debug`, `info`, `warn`, or `error`; output is tagged with the loading plugin ID, or `config` outside plugin loading."},
	{"gopdf.open_external(uri_or_path)", "none", "Ask the operating system to open a URI or path with its default application. Raises an error when the viewer host is unavailable or rejects the request."},
	{"gopdf.formats.extensions()", "string[]", "Return the lower-case extensions, without a leading dot, that the linked document engine can open. The set is read from the engine, so it matches what this build supports."},
	{"gopdf.formats.supports(path)", "boolean", "Report whether a path's extension names an openable format. Matching ignores case. Opening also recognises documents by content, so a file with a missing or misleading extension may still open."},
	{"gopdf.clipboard.get_text()", "string", "Synchronously return the current UTF-8 clipboard text. Raises an error when clipboard access is unavailable."},
	{"gopdf.clipboard.set_text(text)", "none", "Synchronously replace the clipboard with UTF-8 text. Raises an error when clipboard access is unavailable or fails."},
	{"gopdf.pick_file(callback)", "none", "Open the native document picker, filtered to the formats this build can open, then call `callback(result)`. The call returns after the picker and callback complete."},
	{"gopdf.pick_directory(callback)", "none", "Open the native directory picker, then call `callback(result)`. The call returns after the picker and callback complete."},
}

var documentFunctions = []apiEntry{
	{"gopdf.document.metadata()", "table", "Return document metadata: `format`, `encryption`, `title`, `author`, `subject`, `keywords`, `creator`, `producer`, `creation_date`, and `modification_date`. Absent fields are empty strings."},
	{"gopdf.document.outline()", "table[]", "Return the outline as a tree. Each entry has `title`, `uri`, `external`, `page`, `children`, and optional `x` and `y` destination coordinates. `page` is 1-based, or 0 when the entry has no page destination."},
	{"gopdf.document.page_info(page)", "table", "Return `page`, `label`, `width`, `height`, and `bounds` for a 1-based page. Sizes are unrotated PDF points; an out-of-range page raises an error."},
	{"gopdf.document.selection()", "table", "Return the current selection: `active`, `page`, `text`, and `quads` as bounding rectangles. `page` is 0 when nothing is or was selected."},
	{"plugin.document:page_text(page, callback)", "handle", "Extract a page's text asynchronously. The result adds `page` and `text`."},
	{"plugin.document:page_links(page, callback)", "handle", "Read a page's links asynchronously. The result adds `page` and `links`; each link has `bounds`, `uri`, `external`, `page`, and optional `x` and `y`."},
}

var pluginFunctions = []apiEntry{
	{"plugin.fs:read_dir(path[, options], callback)", "handle", "Read one directory asynchronously. `options.follow_symlinks` defaults to false. Without following, a symlink supplied as `path` is rejected; entries themselves are reported as symlinks."},
	{"plugin.fs:stat(path[, options], callback)", "handle", "Stat a path asynchronously. `options.follow_symlinks` defaults to false."},
	{"plugin.timer:after(ms, callback)", "handle", "Call `callback()` once after `ms` milliseconds. `ms` may be zero."},
	{"plugin.timer:every(ms, callback)", "handle", "Call `callback()` repeatedly, waiting `ms` milliseconds between dispatches. `ms` must be greater than zero."},
	{"plugin.storage:get(key)", "value or nil", "Synchronously return the value stored for this plugin ID, or nil when absent or when no session database is available."},
	{"plugin.storage:set(key, value)", "none", "Synchronously persist a plugin-scoped value. Supports nil, booleans, finite numbers, strings, contiguous arrays, and string-keyed objects; tables cannot be cyclic or mix array and object keys. Raises an error when no session database is available; the value is not silently discarded."},
	{"plugin.storage:delete(key)", "none", "Synchronously delete a plugin-scoped value."},
	{"plugin.storage:keys()", "string[]", "Synchronously return this plugin's keys in lexical order."},
	{"plugin.http:request(spec, callback)", "handle", "Make an asynchronous HTTP request. `spec` has required `url` (`http` or `https`) and optional `method` (default `GET`), string-to-string `headers`, `body`, and positive `timeout_ms`."},
	{"plugin:job(spec, callback)", "handle", "Start a subprocess without invoking a shell. `spec.command` is required; optional fields are string-array `args`, `cwd`, positive `timeout_ms`, string-to-string `env` additions, and `stdin`."},
}

var resultSchemas = []apiEntry{
	{"picker result", "`success`, `path`, `cancelled`, `error`", "Cancellation is a normal result: `cancelled=true`, `success=false`, empty `path`, and empty `error`."},
	{"fs.stat result", "`success`, `error`, `cancelled`, `timed_out`; on success: `name`, `path`, `type`, `size_bytes`, `modified_unix`", "`type` is `file`, `directory`, `symlink`, or `other`. `modified_unix` is whole seconds since the Unix epoch."},
	{"fs.read_dir result", "`success`, `error`, `cancelled`, `timed_out`, `entries`", "On success, `entries` is an array of the same file-information fields returned by `stat`."},
	{"http result", "`success`, `status`, `headers`, `body`, `error`, `timed_out`, `cancelled`", "On transport failure, status is 0 and body and headers are empty. Response header values are arrays of strings. HTTP error status codes are completed responses and do not by themselves set `success=false`."},
	{"job result", "`id`, `success`, `code`, `stdout`, `stderr`, `error`, `timed_out`, `cancelled`", "`success` requires exit code 0, no launch/wait error, and no timeout. `code` is -1 when no process exit code is available."},
}

var apiLimits = []apiEntry{
	{"Storage value", "65,536 bytes", "Limit applies to the encoded JSON value for each key."},
	{"HTTP response body", "8 MiB", "A larger response fails the operation; request bodies have no API-specific size limit."},
	{"HTTP redirects", "10", "Following a redirect after the tenth redirect fails the operation."},
	{"Job stdout", "4 MiB", "Captured independently; additional output is discarded."},
	{"Job stderr", "4 MiB", "Captured independently; additional output is discarded."},
}

func renderPortablePluginReference(b *strings.Builder) {
	b.WriteString("\n### Portable API\n\n")
	b.WriteString("The `gopdf` tables below are available to configuration and plugin Lua code.\n\n")
	renderAPITable(b, "Module", portableModules)

	b.WriteString("\n#### Host services\n\n")
	renderAPITable(b, "Function", hostFunctions)

	b.WriteString("\n#### Document inspection\n\n")
	b.WriteString("These read the open document. The `gopdf.document` fields `path`, `name`, `extension`, `exists`, `size_bytes`, and `page_count` remain available and are refreshed when the document changes. The functions below raise an error when no document is open. Rectangles are `x0`, `y0`, `x1`, `y1` in unrotated PDF points.\n\n")
	renderAPITable(b, "Function", documentFunctions)

	b.WriteString("\n### Plugins\n\n")
	b.WriteString("Plugins are discovered without being executed. Enable one explicitly with `local plugin = require(\"plugin-id\")`; only the required plugin and its declared dependencies execute. The entrypoint must call `gopdf.plugin.register(\"plugin-id\"[, spec])`; `require` returns the registered module.\n\n")
	b.WriteString("A plugin manifest is `gopdf-plugin.json` with `id`, `version`, `module`, and `dependencies`; only `id` is required, and unrecognised fields are ignored. `module` selects `lua/<module>.lua` or `lua/<module>/init.lua`. Plugin modules contain `id`, `version`, `actions`, `fs`, `timer`, `storage`, `http`, and `document`, plus `register_action`, `register_command`, `on`, `off`, and `job`. Registered actions use `plugin-id.action`; commands use `:plugin-id-command`; options use `plugin-id.option`.\n\n")
	renderAPITable(b, "Function", pluginFunctions)

	b.WriteString("\n#### Handles and results\n\n")
	b.WriteString("Every `schedule`, filesystem, timer, HTTP, document-extraction, and job call returns a handle with numeric `id`, `cancel()`, and `active()`. `active()` becomes false before a one-shot completion callback runs. Repeating timers remain active between callbacks until cancelled. `cancel()` is idempotent, removes the operation, and does not invoke its callback, so delivered result tables always have `cancelled=false`. Callbacks run serially on the main Lua thread, never on worker goroutines.\n\n")
	renderAPITable(b, "Result", resultSchemas)

	b.WriteString("\n#### Limits\n\n")
	renderAPITable(b, "Resource", apiLimits)

	b.WriteString("\n#### Lifecycle\n\n")
	b.WriteString("Each plugin has an isolated local-module cache and search root. It may require another discovered plugin by ID only when that ID is declared in `dependencies`. Dependencies load first, and events are delivered in activation order, so dependencies receive an event before their dependents. Registration of actions, commands, and event subscriptions is allowed only while the plugin entrypoint is loading; `off(id)`, storage calls, timers, filesystem and HTTP operations, and jobs remain available afterward.\n\n")
	b.WriteString("A successful `:reload-config` replaces the Lua state and cancels all handles and jobs from the old generation without invoking their callbacks. If reload fails, the previous state and its operations remain active. A failed plugin load rolls back that plugin and cancels work it started. Viewer shutdown emits `shutdown` before runtime close; close then cancels remaining operations and jobs. Storage is keyed by plugin ID in the session database and survives reloads and application restarts.\n\n")
	b.WriteString("Supported events are `app_ready`, `document_open_pre`, `document_opened`, `document_close_pre`, `document_closed`, `document_reloaded`, `config_reloaded`, `mouse_button_pre`, `mouse_button`, `selection_changed`, `page_changed`, `zoom_changed`, `option_changed`, and `shutdown`. `page_changed` carries `page`, `label`, `previous_page`, and `page_count`; `zoom_changed` carries `scale`, `previous_scale`, and `percent`. Both are emitted once per frame with the settled value, so a continuous gesture reports where it came to rest. Event callbacks run in subscription order within each plugin; returning true marks an event consumed where the host supports consumption but does not stop later callbacks.\n\n")
	b.WriteString("Plugin search paths are the platform data/config plugin directories and are rescanned by `:reload-config`. Disable plugin discovery with `--no-plugins`, or start from built-in defaults with `--no-config`, which also skips the generated settings file.\n\n#### Single instance\n\nInstances are per document. Each window listens on a socket named after the document it is showing, and the address follows the document when the window opens another one. `--unique` hands the request to the window already showing that document and exits; with no such window, this process opens it. `--goto PAGE` opens at a page and `--goto PAGE:X:Y` at a point on it, overriding any remembered session position, whether the work is done here or by an existing window. `--command TEXT` runs a viewer command there, as typed after `:`, including any command registered by a plugin. An unrecognised command exits non-zero. `X` and `Y` are points measured from the page's top-left corner, which is also SyncTeX's convention. Sockets live under `$XDG_RUNTIME_DIR/gopdf` on Linux, the per-user temporary directory on macOS, and `%LOCALAPPDATA%\\gopdf` on Windows. They are mode 0600 and named by a hash of the document path, which keeps them within the platform socket length limit. A socket left by a crashed process is reclaimed on the next start.\n")
}

func renderAPITable(b *strings.Builder, firstColumn string, entries []apiEntry) {
	fmt.Fprintf(b, "| %s | Returns / fields | Description |\n|---|---|---|\n", firstColumn)
	for _, entry := range entries {
		fmt.Fprintf(b, "| %s | %s | %s |\n", markdownCode(entry.name), entry.result, entry.description)
	}
}
