package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

const serveTestSource = `
let hits = 0;

func hello(req) {
  return {"status": 200, "headers": {"X-Test": "yes"}, "body": "hello " + req["params"]["name"]};
}

let routes = {
  "GET /": func(req) { return "root"; },
  "GET /hello/:name": hello,
  "POST /echo": func(req) { return {"body": req["body"], "headers": {"Content-Type": "text/plain"}}; },
  "GET /json": func(req) { return [1, 2, 3]; },
  "GET /object": func(req) { return {"ok": true}; },
  "GET /inspect": func(req) { return req["query"]["q"] + "|" + req["headers"]["X-Probe"]; },
  "GET /users/:id/posts/:post": func(req) { return req["params"]["id"] + "/" + req["params"]["post"]; },
  "GET /fail": func(req) { throw "boom"; },
  "GET /bad-status": func(req) { return {"status": "nope"}; },
  "GET /hits": func(req) { hits += 1; return str(hits); },
  "POST /created": func(req) { return {"status": 201, "body": {"made": true}}; },
};
`

// runServerTestProgram compiles and runs src, then returns the router built
// from its top-level "routes" hash.
func runServerTestProgram(t *testing.T, src string) *veriRouter {
	t.Helper()
	lexer := NewLexer(src)
	parser := NewParser(lexer)
	program := parser.ParseProgram()
	if errs := parser.Errors(); len(errs) > 0 {
		t.Fatalf("parse errors:\n%s", strings.Join(errs, "\n"))
	}
	compiler := NewCompilerWithSource("<serve-test>", src)
	if err := compiler.Compile(program); err != nil {
		t.Fatal(err)
	}
	bc := compiler.Bytecode()
	vm := NewVM(bc)
	vm.SetAllowIO(true)
	if err := vm.Run(); err != nil {
		t.Fatal(err)
	}

	var routes *Hash
	for i, name := range bc.Symbols {
		if name != "routes" {
			continue
		}
		hash, ok := vm.globals[i].(*Hash)
		if !ok {
			t.Fatalf("routes global is %T", vm.globals[i])
		}
		routes = hash
	}
	if routes == nil {
		t.Fatal("routes global not found")
	}
	router, err := newRouter(vm, routes)
	if err != nil {
		t.Fatal(err)
	}
	return router
}

func doRequest(router http.Handler, method, target, body string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func TestServeRoutesParamsAndResponses(t *testing.T) {
	router := runServerTestProgram(t, serveTestSource)

	tests := []struct {
		name        string
		method      string
		target      string
		body        string
		headers     map[string]string
		wantStatus  int
		wantBody    string
		wantContent string
	}{
		{name: "root", method: "GET", target: "/", wantStatus: 200, wantBody: "root", wantContent: "text/plain"},
		{name: "param", method: "GET", target: "/hello/Ada", wantStatus: 200, wantBody: "hello Ada", wantContent: "text/plain"},
		{name: "trailing slash", method: "GET", target: "/hello/Ada/", wantStatus: 200, wantBody: "hello Ada"},
		{name: "two params", method: "GET", target: "/users/7/posts/9", wantStatus: 200, wantBody: "7/9"},
		{name: "echo body", method: "POST", target: "/echo", body: "ping", wantStatus: 200, wantBody: "ping", wantContent: "text/plain"},
		{name: "json array", method: "GET", target: "/json", wantStatus: 200, wantBody: "[1,2,3]", wantContent: "application/json"},
		{name: "json object", method: "GET", target: "/object", wantStatus: 200, wantBody: `{"ok":true}`, wantContent: "application/json"},
		{name: "query and headers", method: "GET", target: "/inspect?q=needle", headers: map[string]string{"X-Probe": "probe"}, wantStatus: 200, wantBody: "needle|probe"},
		{name: "created", method: "POST", target: "/created", wantStatus: 201, wantBody: `{"made":true}`, wantContent: "application/json"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := doRequest(router, test.method, test.target, test.body, test.headers)
			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d (body %q)", recorder.Code, test.wantStatus, recorder.Body.String())
			}
			if strings.TrimRight(recorder.Body.String(), "\n") != test.wantBody {
				t.Fatalf("body = %q, want %q", recorder.Body.String(), test.wantBody)
			}
			if test.wantContent != "" {
				if got := recorder.Header().Get("Content-Type"); !strings.HasPrefix(got, test.wantContent) {
					t.Fatalf("content type = %q, want %q", got, test.wantContent)
				}
			}
		})
	}
}

func TestServeCustomHeader(t *testing.T) {
	router := runServerTestProgram(t, serveTestSource)
	recorder := doRequest(router, "GET", "/hello/Bob", "", nil)
	if got := recorder.Header().Get("X-Test"); got != "yes" {
		t.Fatalf("X-Test = %q", got)
	}
}

func TestServeHandlerStatePersistsAcrossRequests(t *testing.T) {
	router := runServerTestProgram(t, serveTestSource)
	for i := 1; i <= 2; i++ {
		recorder := doRequest(router, "GET", "/hits", "", nil)
		if recorder.Body.String() != strconv.Itoa(i) {
			t.Fatalf("request %d body = %q", i, recorder.Body.String())
		}
	}
}

func TestServeErrors(t *testing.T) {
	router := runServerTestProgram(t, serveTestSource)

	notFound := doRequest(router, "GET", "/missing", "", nil)
	if notFound.Code != http.StatusNotFound {
		t.Fatalf("404 status = %d", notFound.Code)
	}

	notAllowed := doRequest(router, "POST", "/hello/Ada", "", nil)
	if notAllowed.Code != http.StatusMethodNotAllowed {
		t.Fatalf("405 status = %d", notAllowed.Code)
	}
	if got := notAllowed.Header().Get("Allow"); got != "GET" {
		t.Fatalf("Allow = %q", got)
	}

	failed := doRequest(router, "GET", "/fail", "", nil)
	if failed.Code != http.StatusInternalServerError {
		t.Fatalf("500 status = %d", failed.Code)
	}
	if !strings.Contains(failed.Body.String(), "boom") {
		t.Fatalf("failure body = %q", failed.Body.String())
	}

	badStatus := doRequest(router, "GET", "/bad-status", "", nil)
	if badStatus.Code != http.StatusInternalServerError {
		t.Fatalf("bad status response = %d", badStatus.Code)
	}
	if !strings.Contains(badStatus.Body.String(), "status must be an integer") {
		t.Fatalf("bad status body = %q", badStatus.Body.String())
	}

	// A failed request must not poison the VM for the next one.
	recovered := doRequest(router, "GET", "/json", "", nil)
	if recovered.Code != http.StatusOK || recovered.Body.String() != "[1,2,3]" {
		t.Fatalf("request after failure = %d %q", recovered.Code, recovered.Body.String())
	}
}

func TestServeOverRealHTTP(t *testing.T) {
	router := runServerTestProgram(t, serveTestSource)
	server := httptest.NewServer(router)
	defer server.Close()

	response, err := http.Get(server.URL + "/hello/Net")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "hello Net" {
		t.Fatalf("body = %q", body)
	}
}

func TestServeRouteValidation(t *testing.T) {
	vm := NewVM(&Bytecode{})
	integer := &Integer{Value: 1}
	handler := &Builtin{Name: "h", Fn: func(vm *VM, args ...Object) Object { return &Null{} }}
	twoParams := &Closure{Fn: &CompiledFunction{NumParams: 2}}

	tests := []struct {
		name   string
		routes []httpRouteKey
		want   string
	}{
		{name: "bad key", routes: []httpRouteKey{{"GET", integer}}, want: "must look like"},
		{name: "bad path", routes: []httpRouteKey{{"GET x", handler}}, want: "path must start with /"},
		{name: "non function", routes: []httpRouteKey{{"GET /x", integer}}, want: "handler must be a function"},
		{name: "wrong arity", routes: []httpRouteKey{{"GET /x", twoParams}}, want: "must take 1 parameter"},
		{name: "empty param", routes: []httpRouteKey{{"GET /x/:", handler}}, want: "missing a name"},
		{name: "duplicate param", routes: []httpRouteKey{{"GET /x/:id/:id", handler}}, want: "duplicate path parameter"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			routes := &Hash{Pairs: map[string]HashPairObj{}}
			for _, entry := range test.routes {
				routes.Pairs[cellKey(entry.key)] = HashPairObj{Key: &String{Value: entry.key}, Value: entry.value}
			}
			_, err := newRouter(vm, routes)
			if err == nil {
				t.Fatal("expected a validation error")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

type httpRouteKey struct {
	key   string
	value Object
}

func TestServeGatedInWebIDE(t *testing.T) {
	_, err := runSourceWithIO(t, `serve(8080, {"GET /": func(req) { return null; }});`, false)
	if err == nil {
		t.Fatal("expected serve to be gated")
	}
	if !strings.Contains(err.Error(), "disabled in the web IDE") {
		t.Fatalf("error = %v", err)
	}
}

func TestServeInvalidPort(t *testing.T) {
	_, err := runSourceWithIO(t, `serve(70000, {"GET /": func(req) { return null; }});`, true)
	if err == nil {
		t.Fatal("expected a port error")
	}
	if !strings.Contains(err.Error(), "between 1 and 65535") {
		t.Fatalf("error = %v", err)
	}
}

func TestServeReportsBadRoutesBeforeListening(t *testing.T) {
	_, err := runSourceWithIO(t, `
func two(req, extra) { return null; }
serve(9, {"GET /x": two});
`, true)
	if err == nil {
		t.Fatal("expected a route error")
	}
	if !strings.Contains(err.Error(), "must take 1 parameter") {
		t.Fatalf("error = %v", err)
	}
}

func TestServeHandlerCanPrintAndUseStore(t *testing.T) {
	path := storePath(t)
	router := runServerTestProgram(t, fmt.Sprintf(`
let db = db_open(%q);

func bump(req) {
  let next = db_get(db, "count");
  if next == null { next = 0; }
  db_set(db, "count", next + 1);
  print "hit " + str(next + 1);
  return str(next + 1);
}

let routes = {
  "GET /bump": bump,
  "GET /count": func(req) { return str(db_get(db, "count")); },
};
`, path))

	for i := 1; i <= 2; i++ {
		recorder := doRequest(router, "GET", "/bump", "", nil)
		if recorder.Body.String() != strconv.Itoa(i) {
			t.Fatalf("bump %d body = %q", i, recorder.Body.String())
		}
	}
	recorder := doRequest(router, "GET", "/count", "", nil)
	if recorder.Body.String() != "2" {
		t.Fatalf("count body = %q", recorder.Body.String())
	}
}
