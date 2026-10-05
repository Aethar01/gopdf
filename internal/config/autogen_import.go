package config

import (
	"bufio"
	"bytes"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// gopdf used to keep keybindings changed in the viewer in a generated
// autogen.lua beside config.lua. importAutogen moves one it finds into the
// session database, once, and renames it so it is not read again; this can
// go once few are likely to be left.

// autogenLine is one line gopdf wrote to autogen.lua: an unbind, or a bind
// to a built-in action (gopdf.name) or a plugin's ("plugin.name").
var autogenLine = regexp.MustCompile(`^gopdf\.(bind|unbind)\(("(?:[^"\\]|\\.)*")(?:,\s*(?:gopdf\.([a-z0-9_]+)|("(?:[^"\\]|\\.)*")))?\)$`)

func (r *Runtime) autogenPath() string {
	if r.explicitPath != "" {
		return filepath.Join(filepath.Dir(r.explicitPath), "autogen.lua")
	}
	return platformAutogenPath()
}

// importAutogen moves the keybindings in an old autogen.lua into the
// database, where a key already stored keeps its binding, and renames the
// file to autogen.lua.bak.
func (r *Runtime) importAutogen() {
	path := r.autogenPath()
	if path == "" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("autogen.lua: %v", err)
		}
		return
	}
	bindings := parseAutogen(path, data)
	stored, err := storedKeyBindings()
	if err != nil {
		log.Printf("autogen.lua: not imported: %v", err)
		return
	}
	for key := range stored {
		delete(bindings, key)
	}
	if len(bindings) > 0 {
		if err := storeKeyBindings(bindings); err != nil {
			log.Printf("autogen.lua: not imported: %v", err)
			return
		}
	}
	backup := path + ".bak"
	if err := os.Rename(path, backup); err != nil {
		log.Printf("autogen.lua: imported, but not renamed: %v", err)
		return
	}
	log.Printf("moved the keybindings in %s into the session database; the file is now %s", path, backup)
}

// parseAutogen reads the bindings gopdf wrote to an autogen.lua, as a key's
// action or "" for a key unbound. Lines it did not write are skipped.
func parseAutogen(path string, data []byte) map[string]string {
	bindings := map[string]string{}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "--") {
			continue
		}
		m := autogenLine.FindStringSubmatch(line)
		if m == nil {
			log.Printf("%s: skipping %q", path, line)
			continue
		}
		key, err := strconv.Unquote(m[2])
		if err != nil {
			log.Printf("%s: skipping %q", path, line)
			continue
		}
		switch {
		case m[1] == "unbind" && m[3] == "" && m[4] == "":
			bindings[key] = ""
		case m[1] == "bind" && m[3] != "":
			bindings[key] = m[3]
		case m[1] == "bind" && m[4] != "":
			action, err := strconv.Unquote(m[4])
			if err != nil {
				log.Printf("%s: skipping %q", path, line)
				continue
			}
			bindings[key] = action
		default:
			log.Printf("%s: skipping %q", path, line)
		}
	}
	return bindings
}
