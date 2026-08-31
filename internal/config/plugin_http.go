package config

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	lua "github.com/yuin/gopher-lua"
)

const pluginHTTPBodyLimit = 8 << 20

func newLuaPluginHTTP(L *lua.LState, instance *pluginInstance) *lua.LTable {
	httpModule := L.NewTable()
	L.SetField(httpModule, "request", L.NewFunction(func(L *lua.LState) int {
		spec, ok := L.Get(2).(*lua.LTable)
		if !ok {
			L.RaiseError("plugin %s http.request: expected specification table", instance.manifest.ID)
		}
		callback, ok := L.Get(3).(*lua.LFunction)
		if !ok {
			L.RaiseError("plugin %s http.request: expected callback", instance.manifest.ID)
		}
		requestSpec, err := parsePluginHTTPRequest(spec)
		if err != nil {
			L.RaiseError("plugin %s http.request: %v", instance.manifest.ID, err)
		}
		id := instance.runtime.startPluginOperation(instance.manifest.ID, "http", callback, func(ctx context.Context) map[string]any {
			return runPluginHTTPRequest(ctx, requestSpec)
		})
		L.Push(newPluginOperationHandle(L, instance.runtime, id))
		return 1
	}))
	return httpModule
}

type pluginHTTPRequest struct {
	method  string
	url     string
	headers map[string]string
	body    string
	timeout time.Duration
}

func parsePluginHTTPRequest(spec *lua.LTable) (pluginHTTPRequest, error) {
	request := pluginHTTPRequest{
		method: strings.ToUpper(strings.TrimSpace(lua.LVAsString(spec.RawGetString("method")))),
		url:    strings.TrimSpace(lua.LVAsString(spec.RawGetString("url"))),
		body:   lua.LVAsString(spec.RawGetString("body")),
	}
	if request.method == "" {
		request.method = http.MethodGet
	}
	if request.url == "" {
		return pluginHTTPRequest{}, fmt.Errorf("url is required")
	}
	if !strings.HasPrefix(request.url, "http://") && !strings.HasPrefix(request.url, "https://") {
		return pluginHTTPRequest{}, fmt.Errorf("url must use http or https")
	}
	if timeoutMS := int(lua.LVAsNumber(spec.RawGetString("timeout_ms"))); timeoutMS > 0 {
		request.timeout = time.Duration(timeoutMS) * time.Millisecond
	}
	request.headers = map[string]string{}
	if headers, ok := spec.RawGetString("headers").(*lua.LTable); ok {
		var headerErr error
		headers.ForEach(func(key, value lua.LValue) {
			if key.Type() != lua.LTString || value.Type() != lua.LTString {
				headerErr = fmt.Errorf("headers must contain string keys and values")
				return
			}
			request.headers[key.String()] = value.String()
		})
		if headerErr != nil {
			return pluginHTTPRequest{}, headerErr
		}
	}
	return request, nil
}

func runPluginHTTPRequest(parent context.Context, spec pluginHTTPRequest) map[string]any {
	ctx := parent
	cancel := func() {}
	if spec.timeout > 0 {
		ctx, cancel = context.WithTimeout(parent, spec.timeout)
	}
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, spec.method, spec.url, strings.NewReader(spec.body))
	if err != nil {
		return httpOperationError(err, false)
	}
	for key, value := range spec.headers {
		req.Header.Set(key, value)
	}
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("redirect limit exceeded")
			}
			return nil
		},
	}
	response, err := client.Do(req)
	if err != nil {
		return httpOperationError(err, ctx.Err() == context.DeadlineExceeded)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, pluginHTTPBodyLimit+1))
	if err != nil {
		return httpOperationError(err, false)
	}
	if len(body) > pluginHTTPBodyLimit {
		return httpOperationError(fmt.Errorf("response body exceeds %d byte limit", pluginHTTPBodyLimit), false)
	}
	headers := make(map[string]any, len(response.Header))
	for key, values := range response.Header {
		headers[key] = append([]string(nil), values...)
	}
	return map[string]any{
		"success": true, "status": response.StatusCode, "headers": headers,
		"body": string(body), "error": "", "timed_out": false,
	}
}

func httpOperationError(err error, timedOut bool) map[string]any {
	return map[string]any{
		"success": false, "status": 0, "headers": map[string]any{}, "body": "",
		"error": err.Error(), "timed_out": timedOut,
	}
}
