package config

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	lua "github.com/yuin/gopher-lua"
)

func TestPluginFSReadDirAndStat(t *testing.T) {
	dir := t.TempDir()
	name := "unicode \u03c0 \tline\n.pdf"
	if runtime.GOOS == "windows" {
		name = "unicode \u03c0 line.pdf"
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("pdf"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(path, filepath.Join(dir, "paper-link")); err != nil && runtime.GOOS != "windows" {
		t.Fatal(err)
	}

	rt := openPluginRuntime(t, `
local M = gopdf.plugin.register("sample")
M.fs:read_dir(`+luaQuote(dir)+`, function(result)
  M.fs_result = result
end)
M.fs:stat(`+luaQuote(path)+`, function(result)
  M.stat_result = result
end)
return M
`)
	pollRuntimeUntil(t, rt, func() bool {
		return evalBool(rt, `sample.fs_result and sample.fs_result.success and sample.stat_result and sample.stat_result.success`)
	})

	if _, err := rt.Eval(`
assert(#sample.fs_result.entries >= 2)
local found_file, found_empty, found_link = false, false, false
for _, entry in ipairs(sample.fs_result.entries) do
  if entry.name == ` + luaQuote(name) + ` then
    found_file = entry.type == "file" and entry.size_bytes == 3
  elseif entry.name == "empty" then
    found_empty = entry.type == "directory"
  elseif entry.name == "paper-link" then
    found_link = entry.type == "symlink"
  end
end
assert(found_file and found_empty)
assert(sample.stat_result.type == "file" and sample.stat_result.path == ` + luaQuote(path) + `)
`); err != nil {
		t.Fatal(err)
	}
}

func TestPluginOperationCancellationAndReload(t *testing.T) {
	rt := openPluginRuntime(t, `
local M = gopdf.plugin.register("sample")
M.cancelled_callback = false
M.op = M.timer:after(5000, function() M.cancelled_callback = true end)
return M
`)
	if len(rt.operations) != 1 {
		t.Fatalf("operations = %d", len(rt.operations))
	}
	if _, err := rt.Eval(`assert(sample.op:active()); sample.op:cancel(); assert(not sample.op:active())`); err != nil {
		t.Fatal(err)
	}
	if len(rt.operations) != 0 {
		t.Fatal("cancelled operation remains registered")
	}
	if _, err := rt.Eval(`sample.op = sample.timer:after(5000, function() sample.cancelled_callback = true end)`); err != nil {
		t.Fatal(err)
	}
	if err := rt.Reload(); err != nil {
		t.Fatal(err)
	}
	if len(rt.operations) != 0 {
		t.Fatal("reload did not cancel old-generation operations")
	}
}

func TestPluginTimersAndSchedule(t *testing.T) {
	rt := openPluginRuntime(t, `
local M = gopdf.plugin.register("sample")
M.ticks = 0
M.timer_handle = M.timer:every(1, function()
  M.ticks = M.ticks + 1
  if M.ticks == 2 then M.timer_handle:cancel() end
end)
gopdf.schedule(function() M.scheduled = true end)
return M
`)
	pollRuntimeUntil(t, rt, func() bool {
		return evalBool(rt, `sample.scheduled == true and sample.ticks == 2 and not sample.timer_handle:active()`)
	})
}

func TestPluginHTTPRedirectTimeoutAndBodyLimit(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/redirect":
			http.Redirect(w, req, "/ok", http.StatusFound)
		case "/large":
			_, _ = w.Write([]byte(strings.Repeat("x", pluginHTTPBodyLimit+1)))
		case "/slow":
			time.Sleep(100 * time.Millisecond)
			_, _ = w.Write([]byte("late"))
		default:
			w.Header().Set("X-Test", "yes")
			_, _ = w.Write([]byte("ok"))
		}
	}))
	defer server.Close()

	// The test server uses a private CA. Override only this runtime's requests by
	// replacing the process default transport for the duration of the test.
	oldTransport := http.DefaultTransport
	http.DefaultTransport = server.Client().Transport
	defer func() { http.DefaultTransport = oldTransport }()

	rt := openPluginRuntime(t, fmt.Sprintf(`
local M = gopdf.plugin.register("sample")
M.http:request({url=%q}, function(r) M.redirect_result = r end)
M.http:request({url=%q, timeout_ms=5}, function(r) M.timeout_result = r end)
M.http:request({url=%q}, function(r) M.large_result = r end)
return M
`, server.URL+"/redirect", server.URL+"/slow", server.URL+"/large"))
	pollRuntimeUntil(t, rt, func() bool {
		return evalBool(rt, `sample.redirect_result ~= nil and sample.timeout_result ~= nil and sample.large_result ~= nil`)
	})
	if _, err := rt.Eval(`
assert(sample.redirect_result.success and sample.redirect_result.status == 200 and sample.redirect_result.body == "ok")
assert(not sample.timeout_result.success and sample.timeout_result.timed_out)
assert(not sample.large_result.success and string.find(sample.large_result.error, "exceeds"))
`); err != nil {
		t.Fatal(err)
	}
}

func TestPluginJobEnvironmentAndStdin(t *testing.T) {
	rt := openPluginRuntime(t, fmt.Sprintf(`
local M = gopdf.plugin.register("sample")
M.job_handle = M:job({
  command=%q,
  args={"-test.run=TestPluginJobEnvironmentAndStdin"},
  env={GOPDF_PLUGIN_JOB_HELPER="1", GOPDF_TEST_VALUE="value"},
  stdin="input"
}, function(r) M.job_result = r end)
assert(M.job_handle:active())
return M
`, os.Args[0]))
	pollRuntimeUntil(t, rt, func() bool { return evalBool(rt, `sample.job_result ~= nil`) })
	if _, err := rt.Eval(`assert(sample.job_result.success and sample.job_result.stdout == "value:input" and not sample.job_handle:active())`); err != nil {
		t.Fatal(err)
	}
}

func TestMain(m *testing.M) {
	if os.Getenv("GOPDF_PLUGIN_JOB_HELPER") == "1" {
		data, _ := io.ReadAll(os.Stdin)
		fmt.Print(os.Getenv("GOPDF_TEST_VALUE") + ":" + string(data))
		os.Exit(0)
	}
	if sleep := os.Getenv("GOPDF_PLUGIN_JOB_SLEEP_MS"); sleep != "" {
		ms, _ := strconv.Atoi(sleep)
		time.Sleep(time.Duration(ms) * time.Millisecond)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func openPluginRuntime(t *testing.T, source string) *Runtime {
	t.Helper()
	root := writeTestPlugin(t, "sample", source)
	rt, err := OpenWithOptions(filepath.Join(t.TempDir(), "missing.lua"), "", OpenOptions{PluginPaths: []string{root}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(rt.Close)
	if _, err := rt.Eval(`sample = require("sample")`); err != nil {
		t.Fatal(err)
	}
	return rt
}

func pollRuntimeUntil(t *testing.T, rt *Runtime, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		rt.PollPluginOperations()
		if ready() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for plugin operation (operations=%d jobs=%d)", len(rt.operations), len(rt.jobs))
}

func luaGlobalBool(rt *Runtime, global string, field ...string) bool {
	value := rt.state.GetGlobal(global)
	if len(field) > 0 {
		if table, ok := value.(*lua.LTable); ok {
			value = table.RawGetString(field[0])
		}
	}
	return lua.LVAsBool(value)
}

func evalBool(rt *Runtime, expression string) bool {
	if err := rt.state.DoString(`__api_test_ready = not not (` + expression + `)`); err != nil {
		return false
	}
	return lua.LVAsBool(rt.state.GetGlobal("__api_test_ready"))
}
