package config

import (
	"context"
	"fmt"
	"time"

	lua "github.com/yuin/gopher-lua"
)

type pluginOperation struct {
	id         int
	plugin     string
	generation int
	kind       string
	cancel     context.CancelFunc
	callback   *lua.LFunction
	interval   time.Duration
}

type pluginOperationResult struct {
	id         int
	plugin     string
	generation int
	values     map[string]any
}

func (r *Runtime) startPluginOperation(pluginID, kind string, callback *lua.LFunction, run func(context.Context) map[string]any) int {
	ctx, cancel := context.WithCancel(context.Background())
	r.nextOperationID++
	id := r.nextOperationID
	op := &pluginOperation{id: id, plugin: pluginID, generation: r.pluginGeneration, kind: kind, cancel: cancel, callback: callback}
	r.operations[id] = op
	go func() {
		values := run(ctx)
		select {
		case r.operationResults <- pluginOperationResult{id: id, plugin: pluginID, generation: op.generation, values: values}:
		case <-ctx.Done():
		}
	}()
	return id
}

func (r *Runtime) startPluginTimer(pluginID string, delay time.Duration, repeat bool, callback *lua.LFunction) int {
	ctx, cancel := context.WithCancel(context.Background())
	r.nextOperationID++
	id := r.nextOperationID
	op := &pluginOperation{id: id, plugin: pluginID, generation: r.pluginGeneration, kind: "timer", cancel: cancel, callback: callback}
	if repeat {
		op.interval = delay
	}
	r.operations[id] = op
	r.waitPluginTimer(ctx, op, delay)
	return id
}

func (r *Runtime) waitPluginTimer(ctx context.Context, op *pluginOperation, delay time.Duration) {
	go func() {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-timer.C:
			select {
			case r.operationResults <- pluginOperationResult{id: op.id, plugin: op.plugin, generation: op.generation}:
			case <-ctx.Done():
			}
		case <-ctx.Done():
		}
	}()
}

func newPluginOperationHandle(L *lua.LState, r *Runtime, id int) *lua.LTable {
	handle := L.NewTable()
	L.SetField(handle, "id", lua.LNumber(id))
	L.SetField(handle, "cancel", L.NewFunction(func(L *lua.LState) int {
		r.cancelPluginOperation(id)
		return 0
	}))
	L.SetField(handle, "active", L.NewFunction(func(L *lua.LState) int {
		op, ok := r.operations[id]
		L.Push(lua.LBool(ok && op.generation == r.pluginGeneration))
		return 1
	}))
	return handle
}

func (r *Runtime) cancelPluginOperation(id int) {
	if op, ok := r.operations[id]; ok {
		op.cancel()
		delete(r.operations, id)
	}
}

func (r *Runtime) cancelPluginOperationsFor(pluginID string) {
	for id, op := range r.operations {
		if op.plugin == pluginID && op.generation == r.pluginGeneration {
			op.cancel()
			delete(r.operations, id)
		}
	}
}

func (r *Runtime) cancelPluginOperations() { cancelPluginOperationMap(r.operations) }

func cancelPluginOperationMap(operations map[int]*pluginOperation) {
	for id, op := range operations {
		op.cancel()
		delete(operations, id)
	}
}

func (r *Runtime) pollPluginOperations() bool {
	if r == nil {
		return false
	}
	changed := false
	for {
		select {
		case result := <-r.operationResults:
			op, ok := r.operations[result.id]
			if !ok || result.generation != r.pluginGeneration || result.plugin != op.plugin || r.state == nil {
				continue
			}
			if op.interval == 0 {
				delete(r.operations, result.id)
			}
			var err error
			if op.kind == "timer" || op.kind == "schedule" {
				err = r.callPluginLua(op.plugin, lua.P{Fn: op.callback, NRet: 0, Protect: true})
			} else {
				values := result.values
				if values == nil {
					values = map[string]any{}
				}
				if _, ok := values["success"]; !ok {
					values["success"] = false
				}
				if _, ok := values["error"]; !ok {
					values["error"] = ""
				}
				values["cancelled"] = false
				if _, ok := values["timed_out"]; !ok {
					values["timed_out"] = false
				}
				err = r.callPluginLua(op.plugin, lua.P{Fn: op.callback, NRet: 0, Protect: true}, luaTableFromMap(r.state, values))
			}
			if err != nil {
				r.logf("plugin %s %s operation %d callback: %v", op.plugin, op.kind, op.id, err)
			}
			if current, ok := r.operations[result.id]; ok && current == op && op.interval > 0 {
				ctx, cancel := context.WithCancel(context.Background())
				op.cancel = cancel
				r.waitPluginTimer(ctx, op, op.interval)
			}
			changed = true
		default:
			return changed
		}
	}
}

func (r *Runtime) pluginOperationsActive() bool {
	return r != nil && (len(r.operations) > 0 || len(r.jobs) > 0)
}

func (r *Runtime) schedule(callback *lua.LFunction) (int, error) {
	if r == nil || r.state == nil {
		return 0, fmt.Errorf("Lua runtime unavailable")
	}
	return r.startPluginTimer(r.owningPluginID(), 0, false, callback), nil
}

func (r *Runtime) PollPluginOperations() bool {
	return r.pollPluginOperations() || r.pollPluginJobs()
}

func (r *Runtime) PluginOperationsActive() bool { return r.pluginOperationsActive() }

// callPluginLua dispatches a plugin-owned callback with the plugin attributed
// so diagnostics such as gopdf.log() name the owner rather than the loader.
func (r *Runtime) callPluginLua(pluginID string, params lua.P, args ...lua.LValue) error {
	previous := r.activePlugin
	r.activePlugin = pluginID
	defer func() { r.activePlugin = previous }()
	return r.callLua(params, args...)
}

// owningPluginID returns the plugin that owns work started during the current
// dispatch, or "" when configuration Lua started it.
func (r *Runtime) owningPluginID() string {
	if r.activePlugin != "" {
		return r.activePlugin
	}
	return r.loadingPlugin
}

// diagnosticPluginID names the plugin responsible for the current dispatch.
func (r *Runtime) diagnosticPluginID() string {
	if r == nil {
		return "config"
	}
	if r.activePlugin != "" {
		return r.activePlugin
	}
	if r.loadingPlugin != "" {
		return r.loadingPlugin
	}
	return "config"
}
