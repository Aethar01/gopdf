package viewer

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"gopdf/internal/config"
)

// The viewer names plugin events as literals; this keeps them in step with
// the events plugins may subscribe to, which the reference documents.
func TestEmittedPluginEventsAreTheSubscribableOnes(t *testing.T) {
	sources, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	emit := regexp.MustCompile(`[eE]mitPluginEvent\("([a-z_]+)"`)
	emitted := map[string]bool{}
	for _, path := range sources {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, match := range emit.FindAllStringSubmatch(string(source), -1) {
			emitted[match[1]] = true
			if !slices.Contains(config.PluginEvents, match[1]) {
				t.Errorf("%s emits %q, which plugins cannot subscribe to", path, match[1])
			}
		}
	}
	for _, event := range config.PluginEvents {
		if !emitted[event] {
			t.Errorf("plugins can subscribe to %q, which the viewer never emits", event)
		}
	}
}
