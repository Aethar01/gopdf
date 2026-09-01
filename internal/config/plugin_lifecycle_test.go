package config

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPluginFSSymlinkHandling(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires elevation on Windows")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	linkToDir := filepath.Join(dir, "link-to-dir")
	if err := os.Symlink(target, linkToDir); err != nil {
		t.Fatal(err)
	}
	broken := filepath.Join(dir, "broken")
	if err := os.Symlink(filepath.Join(dir, "missing"), broken); err != nil {
		t.Fatal(err)
	}

	rt := openPluginRuntime(t, fmt.Sprintf(`
local M = gopdf.plugin.register("sample")
M.fs:read_dir(%[1]q, function(r) M.unfollowed = r end)
M.fs:read_dir(%[1]q, {follow_symlinks=true}, function(r) M.followed = r end)
M.fs:stat(%[2]q, function(r) M.broken_lstat = r end)
M.fs:stat(%[2]q, {follow_symlinks=true}, function(r) M.broken_stat = r end)
return M
`, linkToDir, broken))
	pollRuntimeUntil(t, rt, func() bool {
		return evalBool(rt, `sample.unfollowed and sample.followed and sample.broken_lstat and sample.broken_stat`)
	})
	if _, err := rt.Eval(`
-- A symlink supplied as the directory path is only traversed on request.
assert(not sample.unfollowed.success and sample.unfollowed.error ~= "")
assert(sample.followed.success and #sample.followed.entries == 0)
-- A broken symlink still stats as a symlink; following it reports the failure.
assert(sample.broken_lstat.success and sample.broken_lstat.type == "symlink")
assert(not sample.broken_stat.success and sample.broken_stat.error ~= "")
`); err != nil {
		t.Fatal(err)
	}
}

func TestPluginFSUnreadableAndMissingDirectories(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory permission bits are not enforced the same way on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}
	dir := t.TempDir()
	unreadable := filepath.Join(dir, "unreadable")
	if err := os.Mkdir(unreadable, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(unreadable, 0o755) })

	rt := openPluginRuntime(t, fmt.Sprintf(`
local M = gopdf.plugin.register("sample")
M.fs:read_dir(%q, function(r) M.unreadable = r end)
M.fs:read_dir(%q, function(r) M.missing = r end)
return M
`, unreadable, filepath.Join(dir, "does-not-exist")))
	pollRuntimeUntil(t, rt, func() bool { return evalBool(rt, `sample.unreadable and sample.missing`) })
	if _, err := rt.Eval(`
assert(not sample.unreadable.success and sample.unreadable.error ~= "")
assert(not sample.missing.success and sample.missing.error ~= "")
assert(sample.unreadable.cancelled == false and sample.unreadable.timed_out == false)
`); err != nil {
		t.Fatal(err)
	}
}

func TestPluginShutdownCancelsOperations(t *testing.T) {
	rt := openPluginRuntime(t, `
local M = gopdf.plugin.register("sample")
M.timer:every(5000, function() end)
M.timer:after(5000, function() end)
return M
`)
	if len(rt.operations) != 2 {
		t.Fatalf("operations = %d, want 2", len(rt.operations))
	}
	rt.Close()
	if len(rt.operations) != 0 {
		t.Fatalf("close left %d operations registered", len(rt.operations))
	}
}

func TestPluginDisabledIDIsNotLoadable(t *testing.T) {
	root := writeTestPlugin(t, "sample", `
local M = gopdf.plugin.register("sample")
M.timer:every(5000, function() end)
return M
`)
	rt, err := OpenWithOptions(filepath.Join(t.TempDir(), "missing.lua"), "", OpenOptions{
		PluginPaths:     []string{root},
		DisabledPlugins: []string{"sample"},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(rt.Close)
	if _, err := rt.Eval(`sample = require("sample")`); err == nil {
		t.Fatal("expected requiring a disabled plugin to fail")
	}
	if len(rt.operations) != 0 {
		t.Fatalf("disabled plugin started %d operations", len(rt.operations))
	}
}

func TestPluginCallbackErrorDoesNotAbortDispatch(t *testing.T) {
	rt := openPluginRuntime(t, `
local M = gopdf.plugin.register("sample")
M.errored = false
M.after = false
M.timer:after(1, function() M.errored = true; error("callback exploded") end)
M.timer:after(1, function() M.after = true end)
return M
`)
	pollRuntimeUntil(t, rt, func() bool { return evalBool(rt, `sample.errored == true and sample.after == true`) })
	// The failing callback must not leave its operation registered or poison the
	// Lua state for later callbacks.
	if len(rt.operations) != 0 {
		t.Fatalf("operations = %d after both callbacks ran", len(rt.operations))
	}
	if _, err := rt.Eval(`assert(sample.after)`); err != nil {
		t.Fatal(err)
	}
}

func TestPluginHTTPCancellationAndTLSFailure(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	// DefaultTransport is left untouched, so the private test CA is untrusted and
	// the request must fail certificate verification rather than succeed.
	rt := openPluginRuntime(t, fmt.Sprintf(`
local M = gopdf.plugin.register("sample")
M.http:request({url=%q}, function(r) M.tls_result = r end)
M.cancelled = M.http:request({url=%q}, function(r) M.cancelled_result = r end)
M.cancelled:cancel()
return M
`, server.URL, server.URL))
	pollRuntimeUntil(t, rt, func() bool { return evalBool(rt, `sample.tls_result ~= nil`) })
	if _, err := rt.Eval(`
assert(not sample.tls_result.success and sample.tls_result.status == 0)
assert(sample.tls_result.error ~= "" and not sample.tls_result.timed_out)
assert(sample.cancelled_result == nil and not sample.cancelled:active())
`); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(luaStringField(rt, "sample", "tls_result", "error"), "certificate") {
		t.Fatalf("expected a certificate verification error, got %q", luaStringField(rt, "sample", "tls_result", "error"))
	}
}

func TestPluginJobCancellationAndTimeout(t *testing.T) {
	rt := openPluginRuntime(t, fmt.Sprintf(`
local M = gopdf.plugin.register("sample")
M.timed_out_handle = M:job({command=%[1]q, env={GOPDF_PLUGIN_JOB_SLEEP_MS="5000"}, timeout_ms=20}, function(r)
  M.timed_out_result = r
end)
M.cancelled_handle = M:job({command=%[1]q, env={GOPDF_PLUGIN_JOB_SLEEP_MS="5000"}}, function(r)
  M.cancelled_result = r
end)
M.cancelled_handle:cancel()
return M
`, os.Args[0]))
	pollRuntimeUntil(t, rt, func() bool { return evalBool(rt, `sample.timed_out_result ~= nil`) })
	if _, err := rt.Eval(`
assert(not sample.timed_out_result.success and sample.timed_out_result.timed_out)
assert(sample.cancelled_result == nil and not sample.cancelled_handle:active())
`); err != nil {
		t.Fatal(err)
	}
}

func TestPluginManifestAPIVersions(t *testing.T) {
	for _, testCase := range []struct {
		name      string
		manifest  string
		loadable  bool
		wantError string
	}{
		{name: "api 1 still loads", manifest: `{"id":"sample","version":"0.1.0","api":1}`, loadable: true},
		{name: "api 2 loads", manifest: `{"id":"sample","version":"0.1.0","api":2}`, loadable: true},
		{name: "omitted api defaults to current", manifest: `{"id":"sample","version":"0.1.0"}`, loadable: true},
		{name: "future api is not discovered", manifest: `{"id":"sample","version":"0.1.0","api":3}`, wantError: "requires unsupported API 3"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "sample")
			if err := os.MkdirAll(filepath.Join(dir, "lua", "sample"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "gopdf-plugin.json"), []byte(testCase.manifest), 0o644); err != nil {
				t.Fatal(err)
			}
			source := `local M = gopdf.plugin.register("sample")
M.storage:set("key", "value")
return M`
			if err := os.WriteFile(filepath.Join(dir, "lua", "sample", "init.lua"), []byte(source), 0o644); err != nil {
				t.Fatal(err)
			}
			rt, err := OpenWithOptions(filepath.Join(t.TempDir(), "missing.lua"), "", OpenOptions{PluginPaths: []string{root}})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(rt.Close)
			_, err = rt.Eval(`sample = require("sample")`)
			if testCase.loadable {
				if err != nil {
					t.Fatalf("plugin did not load: %v", err)
				}
				// API 1 plugins reach the same module surface; nothing is gated by version.
				if _, err := rt.Eval(`assert(sample.storage:get("key") == "value" and sample.fs and sample.http and sample.timer)`); err != nil {
					t.Fatal(err)
				}
				return
			}
			// A plugin needing a newer API is dropped at discovery with a warning
			// rather than failing at require time.
			if err == nil {
				t.Fatal("expected requiring a future-API plugin to fail")
			}
			if !containsSubstring(rt.pluginCatalog.warnings, testCase.wantError) {
				t.Fatalf("warnings = %v, want one containing %q", rt.pluginCatalog.warnings, testCase.wantError)
			}
		})
	}
}

func TestPluginReloadDropsPendingCallbacks(t *testing.T) {
	// A callback already queued by a worker must not run against the new
	// generation's Lua state.
	rt := openPluginRuntime(t, `
local M = gopdf.plugin.register("sample")
M.fired = false
M.timer:after(1, function() M.fired = true end)
return M
`)
	if err := rt.Reload(); err != nil {
		t.Fatal(err)
	}
	if _, err := rt.Eval(`sample = require("sample")`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 200; i++ {
		rt.PollPluginOperations()
	}
	if _, err := rt.Eval(`assert(sample.fired == false)`); err != nil {
		t.Fatalf("a pre-reload callback ran against the new generation: %v", err)
	}
}

func containsSubstring(values []string, want string) bool {
	for _, value := range values {
		if strings.Contains(value, want) {
			return true
		}
	}
	return false
}

func luaStringField(rt *Runtime, global string, fields ...string) string {
	if err := rt.state.DoString(`__api_test_value = ` + global + `.` + strings.Join(fields, ".")); err != nil {
		return ""
	}
	return rt.state.GetGlobal("__api_test_value").String()
}

func TestPluginStorageSurvivesReload(t *testing.T) {
	setTestDataDir(t)
	rt := openPluginRuntime(t, `
local M = gopdf.plugin.register("sample")
if M.storage:get("loads") == nil then M.storage:set("loads", 0) end
M.storage:set("loads", M.storage:get("loads") + 1)
return M
`)
	if err := rt.Reload(); err != nil {
		t.Fatal(err)
	}
	if _, err := rt.Eval(`sample = require("sample")`); err != nil {
		t.Fatal(err)
	}
	if _, err := rt.Eval(`assert(sample.storage:get("loads") == 2, "storage did not survive reload")`); err != nil {
		t.Fatal(err)
	}
}

func TestLogAttributesTheRunningPlugin(t *testing.T) {
	var output strings.Builder
	previous := log.Writer()
	log.SetOutput(&output)
	log.SetFlags(0)
	t.Cleanup(func() { log.SetOutput(previous) })

	rt := openPluginRuntime(t, `
local M = gopdf.plugin.register("sample")
gopdf.log("info", "during load")
M.timer:after(1, function() gopdf.log("warn", "during callback") end)
return M
`)
	pollRuntimeUntil(t, rt, func() bool { return strings.Contains(output.String(), "during callback") })
	// Configuration Lua is not a plugin, so it is attributed to "config".
	if _, err := rt.Eval(`gopdf.log("error", "from config")`); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"plugin=sample level=info during load",
		"plugin=sample level=warn during callback",
		"plugin=config level=error from config",
	} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("log output %q is missing %q", output.String(), want)
		}
	}
}

func TestLogRejectsUnknownLevels(t *testing.T) {
	rt := openPluginRuntime(t, `return gopdf.plugin.register("sample")`)
	if _, err := rt.Eval(`gopdf.log("trace", "message")`); err == nil || !strings.Contains(err.Error(), "invalid level") {
		t.Fatalf("err = %v, want one containing \"invalid level\"", err)
	}
}

func TestPluginFSReadDirSurvivesUnstattableEntries(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires elevation on Windows")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "good.pdf"), []byte("pdf"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "missing"), filepath.Join(dir, "broken")); err != nil {
		t.Fatal(err)
	}

	rt := openPluginRuntime(t, fmt.Sprintf(`
local M = gopdf.plugin.register("sample")
M.fs:read_dir(%[1]q, {follow_symlinks=true}, function(r) M.followed = r end)
M.fs:read_dir(%[1]q, function(r) M.unfollowed = r end)
return M
`, dir))
	pollRuntimeUntil(t, rt, func() bool { return evalBool(rt, `sample.followed and sample.unfollowed`) })
	if _, err := rt.Eval(`
-- Following symlinks cannot stat the broken one, but the listing still succeeds
-- and the readable sibling is present.
assert(sample.followed.success, "a broken symlink discarded the whole listing")
local by_name = {}
for _, entry in ipairs(sample.followed.entries) do by_name[entry.name] = entry end
assert(by_name["good.pdf"].type == "file" and by_name["good.pdf"].size_bytes == 3)
assert(by_name["good.pdf"].error == "")
assert(by_name["broken"].type == "symlink" and by_name["broken"].error ~= "")
assert(by_name["broken"].size_bytes == 0)
-- Without following, the same entry stats fine as a symlink.
for _, entry in ipairs(sample.unfollowed.entries) do
  if entry.name == "broken" then assert(entry.error == "" and entry.type == "symlink") end
end
`); err != nil {
		t.Fatal(err)
	}
}
