package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

const maxRequestBody = 8 << 20 // 8 MiB

// httpRoute is one compiled "METHOD /path" entry. Segments prefixed with ":"
// capture a request value under that name.
type httpRoute struct {
	method   string
	segments []string
	handler  Object
}

// veriRouter dispatches HTTP requests into VeriCode handlers. Handlers run on
// the program's VM one at a time: the VM is not re-entrant, but it is idle in
// serve(), so serializing calls is enough to keep it safe.
type veriRouter struct {
	vm     *VM
	routes []httpRoute
	mu     sync.Mutex
}

func serveBuiltins() []*Builtin {
	return []*Builtin{
		{
			Name: "serve",
			Fn: func(vm *VM, args ...Object) Object {
				if len(args) != 2 {
					return newError("serve() takes 2 arguments (port, routes), got %d", len(args))
				}
				port, ok := args[0].(*Integer)
				if !ok {
					return newError("serve() port must be an integer, got %s", args[0].Type())
				}
				if port.Value < 1 || port.Value > 65535 {
					return newError("serve() port must be between 1 and 65535, got %d", port.Value)
				}
				routes, ok := args[1].(*Hash)
				if !ok {
					return newError("serve() routes must be a hash of \"METHOD /path\" keys to handlers, got %s", args[1].Type())
				}
				if !vm.allowIO {
					return newError("serve() is disabled in the web IDE; run with the CLI (veri -run file.veri) to start a server")
				}

				router, err := newRouter(vm, routes)
				if err != nil {
					return newError("serve() %v", err)
				}
				addr := fmt.Sprintf(":%d", port.Value)
				listener, err := net.Listen("tcp", addr)
				if err != nil {
					return newError("serve() cannot listen on port %d: %v", port.Value, err)
				}
				vm.out.append(fmt.Sprintf("serving on http://localhost:%d", port.Value))

				server := &http.Server{
					Handler:           router,
					ReadHeaderTimeout: 10 * time.Second,
				}
				if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
					return newError("serve() stopped: %v", err)
				}
				return &Null{}
			},
		},
	}
}

// newRouter validates the route table and returns an http.Handler.
func newRouter(vm *VM, routes *Hash) (*veriRouter, error) {
	keys := make([]string, 0, len(routes.Pairs))
	for _, pair := range routes.Pairs {
		key, ok := pair.Key.(*String)
		if !ok {
			return nil, fmt.Errorf("route keys must be strings, got %s", pair.Key.Type())
		}
		keys = append(keys, key.Value)
	}
	sort.Strings(keys)

	router := &veriRouter{vm: vm}
	seen := make(map[string]bool, len(keys))
	for _, key := range keys {
		method, path, err := parseRouteKey(key)
		if err != nil {
			return nil, err
		}
		segments, err := parseRoutePath(path)
		if err != nil {
			return nil, fmt.Errorf("route %q: %v", key, err)
		}
		handler := routes.Pairs[cellKey(key)].Value
		if err := validateHandler(handler); err != nil {
			return nil, fmt.Errorf("route %q: %v", key, err)
		}
		id := method + " " + strings.Join(segments, "/")
		if seen[id] {
			return nil, fmt.Errorf("duplicate route %q", key)
		}
		seen[id] = true
		router.routes = append(router.routes, httpRoute{method: method, segments: segments, handler: handler})
	}

	// More literal segments means a more specific route; it should win over a
	// pattern like "/:id" regardless of hash iteration order.
	sort.SliceStable(router.routes, func(i, j int) bool {
		return countStatic(router.routes[i].segments) > countStatic(router.routes[j].segments)
	})
	return router, nil
}

func parseRouteKey(key string) (string, string, error) {
	fields := strings.Fields(key)
	if len(fields) != 2 {
		return "", "", fmt.Errorf("route %q must look like \"GET /path\"", key)
	}
	method := strings.ToUpper(fields[0])
	if strings.ContainsAny(method, "/:") {
		return "", "", fmt.Errorf("route %q has an invalid method %q", key, fields[0])
	}
	path := fields[1]
	if !strings.HasPrefix(path, "/") {
		return "", "", fmt.Errorf("route %q path must start with /", key)
	}
	if strings.ContainsAny(path, "?") {
		return "", "", fmt.Errorf("route %q path must not include a query string", key)
	}
	return method, path, nil
}

func parseRoutePath(path string) ([]string, error) {
	segments := splitPath(path)
	names := make(map[string]bool, len(segments))
	for _, segment := range segments {
		if !strings.HasPrefix(segment, ":") {
			continue
		}
		name := segment[1:]
		if name == "" {
			return nil, fmt.Errorf("a path parameter is missing a name")
		}
		if names[name] {
			return nil, fmt.Errorf("duplicate path parameter :%s", name)
		}
		names[name] = true
	}
	return segments, nil
}

func validateHandler(handler Object) error {
	switch fn := handler.(type) {
	case *Closure:
		if fn.Fn.NumParams != 1 {
			return fmt.Errorf("handler must take 1 parameter (request), got %d", fn.Fn.NumParams)
		}
	case *CompiledFunction:
		if fn.NumParams != 1 {
			return fmt.Errorf("handler must take 1 parameter (request), got %d", fn.NumParams)
		}
	case *Builtin:
		// Builtins read their own arguments; arity is checked per request.
	default:
		return fmt.Errorf("handler must be a function, got %s", handler.Type())
	}
	return nil
}

func countStatic(segments []string) int {
	count := 0
	for _, segment := range segments {
		if !strings.HasPrefix(segment, ":") {
			count++
		}
	}
	return count
}

func splitPath(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return nil
	}
	return strings.Split(path, "/")
}

func matchRoute(pattern, parts []string) (map[string]string, bool) {
	if len(pattern) != len(parts) {
		return nil, false
	}
	params := make(map[string]string)
	for i, segment := range pattern {
		if strings.HasPrefix(segment, ":") {
			params[segment[1:]] = parts[i]
			continue
		}
		if segment != parts[i] {
			return nil, false
		}
	}
	return params, true
}

func (rt *veriRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	parts := splitPath(r.URL.Path)

	var matched *httpRoute
	var params map[string]string
	allowed := make([]string, 0)
	for i := range rt.routes {
		route := &rt.routes[i]
		routeParams, ok := matchRoute(route.segments, parts)
		if !ok {
			continue
		}
		allowed = append(allowed, route.method)
		if matched == nil && route.method == r.Method {
			matched = route
			params = routeParams
		}
	}

	if matched == nil {
		if len(allowed) > 0 {
			w.Header().Set("Allow", strings.Join(uniqueStrings(allowed), ", "))
			writeText(w, http.StatusMethodNotAllowed, "405 Method Not Allowed")
			return
		}
		writeText(w, http.StatusNotFound, "404 Not Found")
		return
	}

	body, err := readRequestBody(w, r)
	if err != nil {
		writeText(w, http.StatusBadRequest, "400 Bad Request: "+err.Error())
		return
	}

	request := &Hash{Pairs: map[string]HashPairObj{
		cellKey("method"):  {Key: &String{Value: "method"}, Value: &String{Value: r.Method}},
		cellKey("path"):    {Key: &String{Value: "path"}, Value: &String{Value: r.URL.Path}},
		cellKey("params"):  {Key: &String{Value: "params"}, Value: paramsHash(params)},
		cellKey("query"):   {Key: &String{Value: "query"}, Value: valuesHash(r.URL.Query())},
		cellKey("headers"): {Key: &String{Value: "headers"}, Value: headerHashFromHTTP(r.Header)},
		cellKey("body"):    {Key: &String{Value: "body"}, Value: &String{Value: body}},
	}}

	rt.mu.Lock()
	result, callErr := rt.vm.CallFn(matched.handler, request)
	rt.mu.Unlock()

	if callErr != nil {
		writeText(w, http.StatusInternalServerError, "500 Internal Server Error\n\n"+callErr.Error())
		return
	}
	if errObj, ok := result.(*Error); ok {
		writeText(w, http.StatusInternalServerError, "500 Internal Server Error\n\n"+errObj.Message)
		return
	}
	if err := writeResponse(w, result); err != nil {
		writeText(w, http.StatusInternalServerError, "500 Internal Server Error\n\n"+err.Error())
	}
}

func readRequestBody(w http.ResponseWriter, r *http.Request) (string, error) {
	if r.Body == nil {
		return "", nil
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxRequestBody))
	if err != nil {
		return "", fmt.Errorf("cannot read body: %v", err)
	}
	return string(raw), nil
}

// writeResponse interprets a handler result. A hash containing status,
// headers, or body is a response envelope; anything else is the body itself.
func writeResponse(w http.ResponseWriter, result Object) error {
	status := http.StatusOK
	headers := map[string]string{}
	body := result

	if hash, ok := result.(*Hash); ok {
		envelope := false
		if value, ok := hashLookup(hash, "status"); ok {
			envelope = true
			code, ok := value.(*Integer)
			if !ok {
				return fmt.Errorf("response status must be an integer, got %s", value.Type())
			}
			status = int(code.Value)
		}
		if value, ok := hashLookup(hash, "headers"); ok {
			envelope = true
			parsed, err := parseHeaders(value)
			if err != nil {
				return err
			}
			headers = parsed
		}
		if value, ok := hashLookup(hash, "body"); ok {
			envelope = true
			body = value
		}
		if !envelope {
			body = result
		}
	}

	if status < 100 || status > 599 {
		return fmt.Errorf("response status %d is outside 100-599", status)
	}
	data, contentType, err := encodeBody(body)
	if err != nil {
		return err
	}
	for name, value := range headers {
		w.Header().Set(name, value)
	}
	if contentType != "" && w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.WriteHeader(status)
	_, err = w.Write(data)
	return err
}

func encodeBody(body Object) ([]byte, string, error) {
	switch value := body.(type) {
	case nil, *Null:
		return nil, "", nil
	case *String:
		return []byte(value.Value), "text/plain; charset=utf-8", nil
	case *Error:
		return nil, "", fmt.Errorf("%s", value.Message)
	default:
		encoded, err := objectToJSON(body)
		if err != nil {
			return nil, "", err
		}
		data, err := json.Marshal(encoded)
		if err != nil {
			return nil, "", err
		}
		return data, "application/json", nil
	}
}

func parseHeaders(o Object) (map[string]string, error) {
	hash, ok := o.(*Hash)
	if !ok {
		return nil, fmt.Errorf("response headers must be a hash, got %s", o.Type())
	}
	headers := make(map[string]string, len(hash.Pairs))
	for _, pair := range hash.Pairs {
		name, ok := pair.Key.(*String)
		if !ok {
			return nil, fmt.Errorf("response header names must be strings, got %s", pair.Key.Type())
		}
		value, ok := pair.Value.(*String)
		if !ok {
			return nil, fmt.Errorf("response header %q must be a string, got %s", name.Value, pair.Value.Type())
		}
		headers[name.Value] = value.Value
	}
	return headers, nil
}

func hashLookup(hash *Hash, name string) (Object, bool) {
	pair, ok := hash.Pairs[cellKey(name)]
	if !ok || pair.Value == nil {
		return nil, false
	}
	return pair.Value, true
}

func paramsHash(params map[string]string) *Hash {
	pairs := make(map[string]HashPairObj, len(params))
	for name, value := range params {
		pairs[cellKey(name)] = HashPairObj{Key: &String{Value: name}, Value: &String{Value: value}}
	}
	return &Hash{Pairs: pairs}
}

func valuesHash(values map[string][]string) *Hash {
	pairs := make(map[string]HashPairObj, len(values))
	for name, list := range values {
		value := ""
		if len(list) > 0 {
			value = list[0]
		}
		pairs[cellKey(name)] = HashPairObj{Key: &String{Value: name}, Value: &String{Value: value}}
	}
	return &Hash{Pairs: pairs}
}

func headerHashFromHTTP(headers http.Header) *Hash {
	pairs := make(map[string]HashPairObj, len(headers))
	for name, list := range headers {
		value := ""
		if len(list) > 0 {
			value = list[0]
		}
		pairs[cellKey(name)] = HashPairObj{Key: &String{Value: name}, Value: &String{Value: value}}
	}
	return &Hash{Pairs: pairs}
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func writeText(w http.ResponseWriter, status int, text string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	io.WriteString(w, text+"\n")
}
