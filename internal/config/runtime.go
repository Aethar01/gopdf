package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

func Load(explicitPath string) (Config, error) {
	rt, err := Open(explicitPath, "")
	if err != nil {
		return Config{}, err
	}
	defer rt.Close()
	return rt.Config(), nil
}

func Open(explicitPath, docPath string, verbose ...bool) (*Runtime, error) {
	docPath = AbsoluteDocumentPath(docPath)
	docName := ""
	if docPath != "" {
		docName = filepath.Base(docPath)
	}
	rt := &Runtime{
		explicitPath: explicitPath,
		docPath:      docPath,
		docName:      docName,
		docMeta:      loadDocumentMeta(docPath),
		verbose:      len(verbose) > 0 && verbose[0],
	}
	if err := rt.Reload(); err != nil {
		return nil, err
	}
	return rt, nil
}

func candidatePaths(explicitPath string) []string {
	if explicitPath != "" {
		return []string{explicitPath}
	}
	return unique(platformConfigPaths())
}

func unique(paths []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		out = append(out, path)
	}
	return out
}

func (r *Runtime) Close() {
	if r.state != nil {
		r.state.Close()
		r.state = nil
	}
}

func (r *Runtime) Config() Config {
	return r.cfg
}

func (r *Runtime) AttachHost(host Host) {
	r.host = host
}

func (r *Runtime) SetDocument(path string, pageCount ...int) error {
	path = AbsoluteDocumentPath(path)
	r.docPath = path
	r.docName = ""
	if path != "" {
		r.docName = filepath.Base(path)
	}
	r.docMeta = loadDocumentMeta(path)
	if len(pageCount) > 0 {
		r.docMeta.pageCount = pageCount[0]
		r.docMeta.hasPages = true
	}
	return r.Reload()
}

func (r *Runtime) SetPageCount(pages int) {
	r.docMeta.pageCount = pages
	r.docMeta.hasPages = true
}

func (r *Runtime) Reload() error {
	r.logf("reload config explicit=%q doc=%q", r.explicitPath, r.docPath)
	if r.state != nil {
		r.state.Close()
		r.state = nil
	}
	r.cfg = Default()
	r.callbacks = map[string]*lua.LFunction{}
	r.callbackSeq = 0
	r.dirty = false
	autogenPath := r.autogenPath()
	if autogenPath != "" {
		if info, err := os.Stat(autogenPath); err == nil && !info.IsDir() {
			r.logf("apply autogen config %q", autogenPath)
			if err := r.applyLuaConfig(autogenPath); err != nil {
				r.Close()
				return err
			}
			r.cfg.AutogenPath = autogenPath
		} else if err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	paths := candidatePaths(r.explicitPath)
	for _, path := range paths {
		r.logf("check config %q", path)
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if info.IsDir() {
			continue
		}
		r.logf("apply config %q", path)
		if err := r.applyLuaConfig(path); err != nil {
			r.Close()
			return err
		}
		r.cfg.ConfigPath = path
		r.cfg.AutogenPath = autogenPath
		r.dirty = false
		return nil
	}
	r.initLuaState()
	r.cfg.AutogenPath = autogenPath
	r.dirty = false
	r.logf("no user config loaded")
	return nil
}

func (r *Runtime) logf(format string, args ...any) {
	if r != nil && r.verbose {
		log.Printf(format, args...)
	}
}

func (r *Runtime) autogenPath() string {
	if r.explicitPath != "" {
		return filepath.Join(filepath.Dir(r.explicitPath), "autogen.lua")
	}
	return platformAutogenPath()
}

func (r *Runtime) RunAction(action string) (bool, bool, error) {
	if r == nil {
		return false, false, nil
	}
	fn, ok := r.callbacks[action]
	if !ok {
		return false, false, nil
	}
	r.dirty = false
	if err := r.state.CallByParam(lua.P{Fn: fn, NRet: 0, Protect: true}); err != nil {
		return true, r.dirty, err
	}
	return true, r.dirty, nil
}

func (r *Runtime) Eval(code string) (bool, error) {
	if r == nil || r.state == nil {
		return false, fmt.Errorf("no Lua state")
	}
	r.dirty = false
	if err := r.state.DoString(code); err != nil {
		return r.dirty, err
	}
	return r.dirty, nil
}

func (r *Runtime) RunUISelect(callback string, index int, value string) error {
	return r.runCallback(callback, lua.LNumber(index), lua.LString(value))
}

func (r *Runtime) RunUIClose(callback string) error {
	return r.runCallback(callback)
}

func (r *Runtime) runCallback(callback string, args ...lua.LValue) error {
	if r == nil || callback == "" {
		return nil
	}
	fn, ok := r.callbacks[callback]
	if !ok {
		return fmt.Errorf("unknown lua callback: %s", callback)
	}
	params := lua.P{Fn: fn, NRet: 0, Protect: true}
	return r.state.CallByParam(params, args...)
}

func loadDocumentMeta(docPath string) documentMeta {
	meta := documentMeta{ext: strings.ToLower(filepath.Ext(docPath))}
	if docPath == "" {
		return meta
	}
	info, err := os.Stat(docPath)
	if err == nil && !info.IsDir() {
		meta.exists = true
		meta.sizeBytes = info.Size()
	}
	return meta
}
