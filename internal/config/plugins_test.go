package config

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestPluginsAreLazyAndExposeModuleConfiguration(t *testing.T) {
	pluginRoot := writeTestPlugin(t, "sample", `
local M = gopdf.plugin.register("sample", {
  options = {
    enabled = { type = "boolean", default = false, description = "Enable the sample plugin." },
    command = { type = "string", default = "sample", description = "Sample command." }
  }
})
M:register_action("hello", { description = "Say hello" }, function()
  gopdf.message("hello")
end)
M:register_command("say", { description = ":sample-say - Say something" }, function(ctx)
  gopdf.message(ctx.args[1] or "nothing")
end)
M:on("document_opened", function(event)
  gopdf.message("opened " .. event.document.name)
end)
return M
`)

	rt, err := OpenWithOptions(filepath.Join(t.TempDir(), "missing.lua"), filepath.Join(t.TempDir(), "paper.pdf"), OpenOptions{PluginPaths: []string{pluginRoot}})
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()

	if containsString(rt.ActionNames(), "sample.hello") {
		t.Fatal("discovered plugin should not be active before require")
	}
	if containsString(rt.OptionNames(), "sample.enabled") {
		t.Fatal("discovered plugin options should not be active before require")
	}

	dirty, err := rt.Eval(`
sample = require("sample")
assert(sample.enabled == false)
sample.enabled = true
sample.command = "sample-tool"
gopdf.bind("X", sample.actions.hello)
`)
	if err != nil || !dirty {
		t.Fatalf("require and configure plugin, dirty=%v err=%v", dirty, err)
	}
	if !containsString(rt.ActionNames(), "sample.hello") {
		t.Fatal("required plugin action was not registered")
	}
	if !containsString(rt.OptionNames(), "sample.enabled") {
		t.Fatal("required plugin option was not registered")
	}
	if got, err := rt.OptionValue("sample.command"); err != nil || got != `"sample-tool"` {
		t.Fatalf("plugin option value = %q, %v", got, err)
	}
	if got := rt.Config().KeyBindings["X"]; got != "sample.hello" {
		t.Fatalf("expected plugin action binding, got %q", got)
	}

	host := &stubHost{}
	rt.AttachHost(host)
	if handled, _, err := rt.RunAction("sample.hello"); !handled || err != nil {
		t.Fatalf("run plugin action, handled=%v err=%v", handled, err)
	}
	if host.message != "hello" {
		t.Fatalf("expected plugin action message, got %q", host.message)
	}

	if handled, err := rt.RunPluginCommand("sample-say", "hello world"); !handled || err != nil {
		t.Fatalf("run plugin command, handled=%v err=%v", handled, err)
	}
	if host.message != "hello" {
		t.Fatalf("expected plugin command message, got %q", host.message)
	}

	rt.EmitPluginEvent("document_opened", map[string]any{
		"document": map[string]any{"name": "paper.pdf"},
	})
	if host.message != "opened paper.pdf" {
		t.Fatalf("expected document event message, got %q", host.message)
	}
}

func TestPluginActionBindingCanBeLoadedBeforePluginRequire(t *testing.T) {
	pluginRoot := writeTestPlugin(t, "sample", `
local M = gopdf.plugin.register("sample")
M:register_action("hello", function() end)
return M
`)
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.lua")
	if err := os.WriteFile(configPath, []byte(`gopdf.bind("X", "sample.hello")`), 0o644); err != nil {
		t.Fatal(err)
	}

	rt, err := OpenWithOptions(configPath, "", OpenOptions{PluginPaths: []string{pluginRoot}})
	if err == nil {
		rt.Close()
		t.Fatal("expected a user config binding to require an active plugin")
	}
	if !strings.Contains(err.Error(), "unknown action") {
		t.Fatalf("expected unknown action error, got %v", err)
	}
}

func TestPluginReloadRequiresPluginAgain(t *testing.T) {
	pluginRoot := writeTestPlugin(t, "sample", `
local M = gopdf.plugin.register("sample", {
  options = { value = { type = "integer", default = 1 } }
})
return M
`)
	rt, err := OpenWithOptions(filepath.Join(t.TempDir(), "missing.lua"), "", OpenOptions{PluginPaths: []string{pluginRoot}})
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	if _, err := rt.Eval(`require("sample")`); err != nil {
		t.Fatal(err)
	}
	if !containsString(rt.OptionNames(), "sample.value") {
		t.Fatal("expected active plugin option")
	}
	if err := rt.Reload(); err != nil {
		t.Fatal(err)
	}
	if containsString(rt.OptionNames(), "sample.value") {
		t.Fatal("plugin should not remain active after reload without require")
	}
}

func TestPluginCannotRegisterAsAnotherPlugin(t *testing.T) {
	firstRoot := writeTestPlugin(t, "first", `return gopdf.plugin.register("second")`)
	secondRoot := writeTestPlugin(t, "second", `return gopdf.plugin.register("second")`)
	rt, err := OpenWithOptions(filepath.Join(t.TempDir(), "missing.lua"), "", OpenOptions{PluginPaths: []string{firstRoot, secondRoot}})
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	if _, err := rt.Eval(`require("first")`); err == nil || !strings.Contains(err.Error(), `can only register while require("second") is loading`) {
		t.Fatalf("expected plugin identity error, got %v", err)
	}
}

func TestPluginManifestModuleSelectsEntrypoint(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "sample")
	if err := os.MkdirAll(filepath.Join(dir, "lua", "entry"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "gopdf-plugin.json"), []byte(`{"id":"sample","api":1,"module":"entry.main"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "lua", "entry", "main.lua"), []byte(`return gopdf.plugin.register("sample")`), 0o644); err != nil {
		t.Fatal(err)
	}
	rt, err := OpenWithOptions(filepath.Join(t.TempDir(), "missing.lua"), "", OpenOptions{PluginPaths: []string{root}})
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	if _, err := rt.Eval(`assert(require("sample").id == "sample")`); err != nil {
		t.Fatal(err)
	}
}

func TestMalformedPluginManifestIsRejected(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "broken")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "gopdf-plugin.json"), []byte(`{`), 0o644); err != nil {
		t.Fatal(err)
	}
	catalog := discoverPluginCatalog([]string{root}, nil)
	if _, ok := catalog.manifests["broken"]; ok || len(catalog.warnings) == 0 {
		t.Fatalf("expected malformed manifest rejection, catalog=%+v", catalog)
	}
}

func TestNoPluginsIgnoresExplicitPluginPaths(t *testing.T) {
	root := writeTestPlugin(t, "sample", `return gopdf.plugin.register("sample")`)
	rt, err := OpenWithOptions(filepath.Join(t.TempDir(), "missing.lua"), "", OpenOptions{PluginPaths: []string{root}, NoPlugins: true})
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	if _, err := rt.Eval(`require("sample")`); err == nil {
		t.Fatal("expected --no-plugins semantics to suppress explicit plugin paths")
	}
}

func TestFailedReloadPreservesRunningPluginJobs(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.lua")
	if err := os.WriteFile(configPath, []byte(`-- valid config`), 0o644); err != nil {
		t.Fatal(err)
	}
	rt, err := Open(configPath, "")
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	ctx, cancel := context.WithCancel(context.Background())
	rt.jobs[1] = pluginJob{id: 1, generation: rt.pluginGeneration, cancel: cancel}
	if err := os.WriteFile(configPath, []byte(`this is not lua`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := rt.Reload(); err == nil {
		t.Fatal("expected reload failure")
	}
	if _, ok := rt.jobs[1]; !ok {
		t.Fatal("expected failed reload to preserve the old job")
	}
	select {
	case <-ctx.Done():
		t.Fatal("expected preserved job context to remain active")
	default:
	}
}

func TestFailedPluginEntrypointRollsBackRegistration(t *testing.T) {
	root := writeTestPlugin(t, "sample", `
local M = gopdf.plugin.register("sample")
M:register_action("partial", function() end)
error("entrypoint failed")
`)
	rt, err := OpenWithOptions(filepath.Join(t.TempDir(), "missing.lua"), "", OpenOptions{PluginPaths: []string{root}})
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	if _, err := rt.Eval(`require("sample")`); err == nil {
		t.Fatal("expected entrypoint failure")
	}
	if containsString(rt.ActionNames(), "sample.partial") {
		t.Fatal("failed plugin left a partial action registered")
	}
}

func TestPluginEntrypointMustRegister(t *testing.T) {
	root := writeTestPlugin(t, "sample", `return {}`)
	rt, err := OpenWithOptions(filepath.Join(t.TempDir(), "missing.lua"), "", OpenOptions{PluginPaths: []string{root}})
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	if _, err := rt.Eval(`require("sample")`); err == nil || !strings.Contains(err.Error(), "did not call gopdf.plugin.register") {
		t.Fatalf("expected registration postcondition error, got %v", err)
	}
}

func TestPluginLocalModulesAreIsolated(t *testing.T) {
	first := writeTestPlugin(t, "first", `
local util = require("util")
local M = gopdf.plugin.register("first")
M:register_action("value", function() gopdf.message(util.value) end)
return M
`)
	second := writeTestPlugin(t, "second", `
local util = require("util")
local M = gopdf.plugin.register("second")
M:register_action("value", function() gopdf.message(util.value) end)
return M
`)
	if err := os.WriteFile(filepath.Join(first, "first", "lua", "util.lua"), []byte(`return { value = "first" }`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(second, "second", "lua", "util.lua"), []byte(`return { value = "second" }`), 0o644); err != nil {
		t.Fatal(err)
	}
	rt, err := OpenWithOptions(filepath.Join(t.TempDir(), "missing.lua"), "", OpenOptions{PluginPaths: []string{first, second}})
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	if _, err := rt.Eval(`require("first"); require("second")`); err != nil {
		t.Fatal(err)
	}
	host := &stubHost{}
	rt.AttachHost(host)
	if _, _, err := rt.RunAction("first.value"); err != nil || host.message != "first" {
		t.Fatalf("first plugin local module = %q, err=%v", host.message, err)
	}
	if _, _, err := rt.RunAction("second.value"); err != nil || host.message != "second" {
		t.Fatalf("second plugin local module = %q, err=%v", host.message, err)
	}
}

func TestDeferredOpenRunsAfterAllEventSubscribers(t *testing.T) {
	second := filepath.Join(t.TempDir(), "second.pdf")
	root := writeTestPlugin(t, "sample", `
local M = gopdf.plugin.register("sample")
M:on("app_ready", function() gopdf.open(`+strconv.Quote(second)+`) end)
M:on("app_ready", function() gopdf.message("all callbacks ran") end)
return M
`)
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.lua")
	if err := os.WriteFile(configPath, []byte(`require("sample")`), 0o644); err != nil {
		t.Fatal(err)
	}
	rt, err := OpenWithOptions(configPath, filepath.Join(dir, "first.pdf"), OpenOptions{PluginPaths: []string{root}})
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	host := &reloadingOpenHost{rt: rt}
	rt.AttachHost(host)
	rt.EmitPluginEvent("app_ready", map[string]any{})
	if host.opened != second || host.message != "all callbacks ran" {
		t.Fatalf("expected all callbacks before deferred open, host=%+v", host)
	}
}

func writeTestPlugin(t *testing.T, id, source string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, id)
	if err := os.MkdirAll(filepath.Join(dir, "lua", id), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{"id":"` + id + `","version":"0.1.0","api":1}`
	if err := os.WriteFile(filepath.Join(dir, "gopdf-plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "lua", id, "init.lua"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
