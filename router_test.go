package router

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/valyala/fasthttp"
)

type readWriter struct {
	net.Conn
	r bytes.Buffer
	w bytes.Buffer
}

var httpMethods = []string{
	fasthttp.MethodGet,
	fasthttp.MethodHead,
	fasthttp.MethodPost,
	fasthttp.MethodPut,
	fasthttp.MethodPatch,
	fasthttp.MethodDelete,
	fasthttp.MethodConnect,
	fasthttp.MethodOptions,
	fasthttp.MethodTrace,
	MethodWild,
}

var zeroTCPAddr = &net.TCPAddr{
	IP: net.IPv4zero,
}

func (rw *readWriter) Close() error {
	return nil
}

func (rw *readWriter) Read(b []byte) (int, error) {
	return rw.r.Read(b)
}

func (rw *readWriter) Write(b []byte) (int, error) {
	return rw.w.Write(b)
}

func (rw *readWriter) RemoteAddr() net.Addr {
	return zeroTCPAddr
}

func (rw *readWriter) LocalAddr() net.Addr {
	return zeroTCPAddr
}

func (rw *readWriter) SetReadDeadline(t time.Time) error {
	return nil
}

func (rw *readWriter) SetWriteDeadline(t time.Time) error {
	return nil
}

type assertFn func(rw *readWriter)

func assertWithTestServer(t *testing.T, uri string, handler fasthttp.RequestHandler, fn assertFn) {
	s := &fasthttp.Server{
		Handler: handler,
	}

	rw := &readWriter{}
	ch := make(chan error)

	rw.r.WriteString(uri)
	go func() {
		ch <- s.ServeConn(rw)
	}()
	select {
	case err := <-ch:
		if err != nil {
			t.Fatalf("return error %s", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("timeout")
	}

	fn(rw)
}

func catchPanic(testFunc func()) (recv interface{}) {
	defer func() {
		recv = recover()
	}()

	testFunc()
	return
}

func TestRouter(t *testing.T) {
	router := New()

	routed := false
	router.Handle(fasthttp.MethodGet, "/user/{name}", func(ctx *fasthttp.RequestCtx) {
		routed = true
		want := "gopher"

		param, ok := ctx.UserValue("name").(string)

		if !ok {
			t.Fatalf("wrong wildcard values: param value is nil")
		}

		if param != want {
			t.Fatalf("wrong wildcard values: want %s, got %s", want, param)
		}
	})

	ctx := new(fasthttp.RequestCtx)
	ctx.Request.SetRequestURI("/user/gopher")

	router.Handler(ctx)

	if !routed {
		t.Fatal("routing failed")
	}
}

func TestRouterAPI(t *testing.T) {
	var handled, get, head, options, post, put, patch, delete, any bool

	httpHandler := func(ctx *fasthttp.RequestCtx) {
		handled = true
	}

	router := New()
	router.GET("/GET", func(ctx *fasthttp.RequestCtx) {
		get = true
	})
	router.HEAD("/GET", func(ctx *fasthttp.RequestCtx) {
		head = true
	})
	router.OPTIONS("/GET", func(ctx *fasthttp.RequestCtx) {
		options = true
	})
	router.POST("/POST", func(ctx *fasthttp.RequestCtx) {
		post = true
	})
	router.PUT("/PUT", func(ctx *fasthttp.RequestCtx) {
		put = true
	})
	router.PATCH("/PATCH", func(ctx *fasthttp.RequestCtx) {
		patch = true
	})
	router.DELETE("/DELETE", func(ctx *fasthttp.RequestCtx) {
		delete = true
	})
	router.ANY("/ANY", func(ctx *fasthttp.RequestCtx) {
		any = true
	})
	router.Handle(fasthttp.MethodGet, "/Handler", httpHandler)

	ctx := new(fasthttp.RequestCtx)

	var request = func(method, path string) {
		ctx.Request.Header.SetMethod(method)
		ctx.Request.SetRequestURI(path)
		router.Handler(ctx)
	}

	request(fasthttp.MethodGet, "/GET")
	if !get {
		t.Error("routing GET failed")
	}

	request(fasthttp.MethodHead, "/GET")
	if !head {
		t.Error("routing HEAD failed")
	}

	request(fasthttp.MethodOptions, "/GET")
	if !options {
		t.Error("routing OPTIONS failed")
	}

	request(fasthttp.MethodPost, "/POST")
	if !post {
		t.Error("routing POST failed")
	}

	request(fasthttp.MethodPut, "/PUT")
	if !put {
		t.Error("routing PUT failed")
	}

	request(fasthttp.MethodPatch, "/PATCH")
	if !patch {
		t.Error("routing PATCH failed")
	}

	request(fasthttp.MethodDelete, "/DELETE")
	if !delete {
		t.Error("routing DELETE failed")
	}

	request(fasthttp.MethodGet, "/Handler")
	if !handled {
		t.Error("routing Handler failed")
	}

	for _, method := range httpMethods {
		request(method, "/ANY")
		if !any {
			t.Error("routing ANY failed")
		}

		any = false
	}
}

func TestRouterInvalidInput(t *testing.T) {
	router := New()

	handle := func(_ *fasthttp.RequestCtx) {}

	recv := catchPanic(func() {
		router.Handle("", "/", handle)
	})
	if recv == nil {
		t.Fatal("registering empty method did not panic")
	}

	recv = catchPanic(func() {
		router.GET("", handle)
	})
	if recv == nil {
		t.Fatal("registering empty path did not panic")
	}

	recv = catchPanic(func() {
		router.GET("noSlashRoot", handle)
	})
	if recv == nil {
		t.Fatal("registering path not beginning with '/' did not panic")
	}

	recv = catchPanic(func() {
		router.GET("/", nil)
	})
	if recv == nil {
		t.Fatal("registering nil handler did not panic")
	}
}

func TestRouterChaining(t *testing.T) {
	router1 := New()
	router2 := New()
	router1.NotFound = router2.Handler

	fooHit := false
	router1.POST("/foo", func(ctx *fasthttp.RequestCtx) {
		fooHit = true
		ctx.SetStatusCode(fasthttp.StatusOK)
	})

	barHit := false
	router2.POST("/bar", func(ctx *fasthttp.RequestCtx) {
		barHit = true
		ctx.SetStatusCode(fasthttp.StatusOK)
	})

	ctx := new(fasthttp.RequestCtx)

	ctx.Request.Header.SetMethod(fasthttp.MethodPost)
	ctx.Request.SetRequestURI("/foo")
	router1.Handler(ctx)

	if !(ctx.Response.StatusCode() == fasthttp.StatusOK && fooHit) {
		t.Errorf("Regular routing failed with router chaining.")
		t.FailNow()
	}

	ctx.Request.Header.SetMethod(fasthttp.MethodPost)
	ctx.Request.SetRequestURI("/bar")
	router1.Handler(ctx)

	if !(ctx.Response.StatusCode() == fasthttp.StatusOK && barHit) {
		t.Errorf("Chained routing failed with router chaining.")
		t.FailNow()
	}

	ctx.Request.Header.SetMethod(fasthttp.MethodPost)
	ctx.Request.SetRequestURI("/qax")
	router1.Handler(ctx)

	if !(ctx.Response.StatusCode() == fasthttp.StatusNotFound) {
		t.Errorf("NotFound behavior failed with router chaining.")
		t.FailNow()
	}
}

func TestRouterGroup(t *testing.T) {
	r1 := New()
	r2 := r1.Group("/boo")
	r3 := r1.Group("/goo")
	r4 := r1.Group("/moo")
	r5 := r4.Group("/foo")
	r6 := r5.Group("/foo")

	hit := false

	r1.POST("/foo", func(ctx *fasthttp.RequestCtx) {
		hit = true
		ctx.SetStatusCode(fasthttp.StatusOK)
	})
	r2.POST("/bar", func(ctx *fasthttp.RequestCtx) {
		hit = true
		ctx.SetStatusCode(fasthttp.StatusOK)
	})
	r3.POST("/bar", func(ctx *fasthttp.RequestCtx) {
		hit = true
		ctx.SetStatusCode(fasthttp.StatusOK)
	})
	r4.POST("/bar", func(ctx *fasthttp.RequestCtx) {
		hit = true
		ctx.SetStatusCode(fasthttp.StatusOK)
	})
	r5.POST("/bar", func(ctx *fasthttp.RequestCtx) {
		hit = true
		ctx.SetStatusCode(fasthttp.StatusOK)
	})
	r6.POST("/bar", func(ctx *fasthttp.RequestCtx) {
		hit = true
		ctx.SetStatusCode(fasthttp.StatusOK)
	})
	r6.ServeFiles("/static/{filepath:*}", "./")
	r6.ServeFilesCustom("/custom/static/{filepath:*}", &fasthttp.FS{Root: "./"})

	uris := []string{
		"POST /foo HTTP/1.1\r\n\r\n",
		// testing router group - r2 (grouped from r1)
		"POST /boo/bar HTTP/1.1\r\n\r\n",
		// testing multiple router group - r3 (grouped from r1)
		"POST /goo/bar HTTP/1.1\r\n\r\n",
		// testing multiple router group - r4 (grouped from r1)
		"POST /moo/bar HTTP/1.1\r\n\r\n",
		// testing sub-router group - r5 (grouped from r4)
		"POST /moo/foo/bar HTTP/1.1\r\n\r\n",
		// testing multiple sub-router group - r6 (grouped from r5)
		"POST /moo/foo/foo/bar HTTP/1.1\r\n\r\n",
		// testing multiple sub-router group - r6 (grouped from r5) to serve files
		"GET /moo/foo/foo/static/router.go HTTP/1.1\r\n\r\n",
		// testing multiple sub-router group - r6 (grouped from r5) to serve files with custom settings
		"GET /moo/foo/foo/custom/static/router.go HTTP/1.1\r\n\r\n",
	}

	for _, uri := range uris {
		hit = false

		assertWithTestServer(t, uri, r1.Handler, func(rw *readWriter) {
			br := bufio.NewReader(&rw.w)
			var resp fasthttp.Response
			if err := resp.Read(br); err != nil {
				t.Fatalf("Unexpected error when reading response: %s", err)
			}
			if !(resp.Header.StatusCode() == fasthttp.StatusOK) {
				t.Fatalf("Status code %d, want %d", resp.Header.StatusCode(), fasthttp.StatusOK)
			}
			if !strings.Contains(uri, "static") && !hit {
				t.Fatalf("Regular routing failed with router chaining. %s", uri)
			}
		})
	}

	assertWithTestServer(t, "POST /qax HTTP/1.1\r\n\r\n", r1.Handler, func(rw *readWriter) {
		br := bufio.NewReader(&rw.w)
		var resp fasthttp.Response
		if err := resp.Read(br); err != nil {
			t.Fatalf("Unexpected error when reading response: %s", err)
		}
		if !(resp.Header.StatusCode() == fasthttp.StatusNotFound) {
			t.Errorf("NotFound behavior failed with router chaining.")
			t.FailNow()
		}
	})
}

func TestRouterOPTIONS(t *testing.T) {
	handlerFunc := func(_ *fasthttp.RequestCtx) {}

	router := New()
	router.POST("/path", handlerFunc)

	ctx := new(fasthttp.RequestCtx)

	var checkHandling = func(path, expectedAllowed string, expectedStatusCode int) {
		ctx.Request.Header.SetMethod(fasthttp.MethodOptions)
		ctx.Request.SetRequestURI(path)
		router.Handler(ctx)

		if !(ctx.Response.StatusCode() == expectedStatusCode) {
			t.Errorf("OPTIONS handling failed: Code=%d, Header=%v", ctx.Response.StatusCode(), ctx.Response.Header.String())
		} else if allow := string(ctx.Response.Header.Peek("Allow")); allow != expectedAllowed {
			t.Error("unexpected Allow header value: " + allow)
		}
	}

	// test not allowed
	// * (server)
	checkHandling("*", "OPTIONS, POST", fasthttp.StatusOK)

	// path
	checkHandling("/path", "OPTIONS, POST", fasthttp.StatusOK)

	ctx.Request.Header.SetMethod(fasthttp.MethodOptions)
	ctx.Request.SetRequestURI("/doesnotexist")
	router.Handler(ctx)
	if !(ctx.Response.StatusCode() == fasthttp.StatusNotFound) {
		t.Errorf("OPTIONS handling failed: Code=%d, Header=%v", ctx.Response.StatusCode(), ctx.Response.Header.String())
	}

	// add another method
	router.GET("/path", handlerFunc)

	// set a global OPTIONS handler
	router.GlobalOPTIONS = func(ctx *fasthttp.RequestCtx) {
		// Adjust status code to 204
		ctx.SetStatusCode(fasthttp.StatusNoContent)
	}

	// test again
	// * (server)
	checkHandling("*", "GET, OPTIONS, POST", fasthttp.StatusNoContent)

	// path
	checkHandling("/path", "GET, OPTIONS, POST", fasthttp.StatusNoContent)

	// custom handler
	var custom bool
	router.OPTIONS("/path", func(ctx *fasthttp.RequestCtx) {
		custom = true
	})

	// test again
	// * (server)
	checkHandling("*", "GET, OPTIONS, POST", fasthttp.StatusNoContent)
	if custom {
		t.Error("custom handler called on *")
	}

	// path
	ctx.Request.Header.SetMethod(fasthttp.MethodOptions)
	ctx.Request.SetRequestURI("/path")
	router.Handler(ctx)
	if !(ctx.Response.StatusCode() == fasthttp.StatusNoContent) {
		t.Errorf("OPTIONS handling failed: Code=%d, Header=%v", ctx.Response.StatusCode(), ctx.Response.Header.String())
	}
	if !custom {
		t.Error("custom handler not called")
	}
}

func TestRouterNotAllowed(t *testing.T) {
	handlerFunc := func(_ *fasthttp.RequestCtx) {}

	router := New()
	router.POST("/path", handlerFunc)

	ctx := new(fasthttp.RequestCtx)

	var checkHandling = func(path, expectedAllowed string, expectedStatusCode int) {
		ctx.Request.Header.SetMethod(fasthttp.MethodGet)
		ctx.Request.SetRequestURI(path)
		router.Handler(ctx)

		if !(ctx.Response.StatusCode() == expectedStatusCode) {
			t.Errorf("NotAllowed handling failed:: Code=%d, Header=%v", ctx.Response.StatusCode(), ctx.Response.Header.String())
		} else if allow := string(ctx.Response.Header.Peek("Allow")); allow != expectedAllowed {
			t.Error("unexpected Allow header value: " + allow)
		}
	}

	// test not allowed
	checkHandling("/path", "OPTIONS, POST", fasthttp.StatusMethodNotAllowed)

	// add another method
	router.DELETE("/path", handlerFunc)
	router.OPTIONS("/path", handlerFunc) // must be ignored

	// test again
	checkHandling("/path", "DELETE, OPTIONS, POST", fasthttp.StatusMethodNotAllowed)

	// test custom handler
	responseText := "custom method"
	router.MethodNotAllowed = func(ctx *fasthttp.RequestCtx) {
		ctx.SetStatusCode(fasthttp.StatusTeapot)
		ctx.Write([]byte(responseText))
	}

	ctx.Response.Reset()
	router.Handler(ctx)

	if got := string(ctx.Response.Body()); !(got == responseText) {
		t.Errorf("unexpected response got %q want %q", got, responseText)
	}
	if ctx.Response.StatusCode() != fasthttp.StatusTeapot {
		t.Errorf("unexpected response code %d want %d", ctx.Response.StatusCode(), fasthttp.StatusTeapot)
	}
	if allow := string(ctx.Response.Header.Peek("Allow")); allow != "DELETE, OPTIONS, POST" {
		t.Error("unexpected Allow header value: " + allow)
	}
}

func TestRouterNotFound(t *testing.T) {
	handlerFunc := func(_ *fasthttp.RequestCtx) {}
	host := "fast"

	var buildLocation = func(path string) string {
		return fmt.Sprintf("http://%s%s", host, path)
	}

	router := New()
	router.GET("/path", handlerFunc)
	router.GET("/dir/", handlerFunc)
	router.GET("/", handlerFunc)
	router.GET("/{proc}/StaTus", handlerFunc)
	router.GET("/USERS/{name}/enTRies/", handlerFunc)
	router.GET("/static/{filepath:*}", handlerFunc)

	testRoutes := []struct {
		route    string
		code     int
		location string
	}{
		{"/path/", fasthttp.StatusMovedPermanently, buildLocation("/path")},                                   // TSR -/
		{"/dir", fasthttp.StatusMovedPermanently, buildLocation("/dir/")},                                     // TSR +/
		{"", fasthttp.StatusOK, ""},                                                                           // TSR +/ (Not clean by router, this path is cleaned by fasthttp `ctx.Path()`)
		{"/PATH", fasthttp.StatusMovedPermanently, buildLocation("/path")},                                    // Fixed Case
		{"/DIR/", fasthttp.StatusMovedPermanently, buildLocation("/dir/")},                                    // Fixed Case
		{"/PATH/", fasthttp.StatusMovedPermanently, buildLocation("/path")},                                   // Fixed Case -/
		{"/DIR", fasthttp.StatusMovedPermanently, buildLocation("/dir/")},                                     // Fixed Case +/
		{"/paTh/?name=foo", fasthttp.StatusMovedPermanently, buildLocation("/path?name=foo")},                 // Fixed Case With Query Params +/
		{"/paTh?name=foo", fasthttp.StatusMovedPermanently, buildLocation("/path?name=foo")},                  // Fixed Case With Query Params +/
		{"/../path", fasthttp.StatusOK, ""},                                                                   // CleanPath (Not clean by router, this path is cleaned by fasthttp `ctx.Path()`)
		{"/nope", fasthttp.StatusNotFound, ""},                                                                // NotFound
		{"/sergio/status/", fasthttp.StatusMovedPermanently, buildLocation("/sergio/StaTus")},                 // Fixed Case With Params -/
		{"/users/atreugo/eNtriEs", fasthttp.StatusMovedPermanently, buildLocation("/USERS/atreugo/enTRies/")}, // Fixed Case With Params +/
		{"/STatiC/test.go", fasthttp.StatusMovedPermanently, buildLocation("/static/test.go")},                // Fixed Case Wildcard
	}

	for _, tr := range testRoutes {
		ctx := new(fasthttp.RequestCtx)

		ctx.Request.Header.SetMethod(fasthttp.MethodGet)
		ctx.Request.SetRequestURI(tr.route)
		ctx.Request.SetHost(host)
		router.Handler(ctx)

		statusCode := ctx.Response.StatusCode()
		location := string(ctx.Response.Header.Peek("Location"))
		if !(statusCode == tr.code && (statusCode == fasthttp.StatusNotFound || location == tr.location)) {
			t.Errorf("NotFound handling route %s failed: Code=%d, Header=%v", tr.route, statusCode, location)
		}
	}

	ctx := new(fasthttp.RequestCtx)

	// Test custom not found handler
	var notFound bool
	router.NotFound = func(ctx *fasthttp.RequestCtx) {
		ctx.SetStatusCode(fasthttp.StatusNotFound)
		notFound = true
	}

	ctx.Request.Header.SetMethod(fasthttp.MethodGet)
	ctx.Request.SetRequestURI("/nope")
	router.Handler(ctx)
	if !(ctx.Response.StatusCode() == fasthttp.StatusNotFound && notFound == true) {
		t.Errorf("Custom NotFound handler failed: Code=%d, Header=%v", ctx.Response.StatusCode(), ctx.Response.Header.String())
	}
	ctx.Response.Reset()

	// Test other method than GET (want 308 instead of 301)
	router.PATCH("/path", handlerFunc)

	ctx.Request.Header.SetMethod(fasthttp.MethodPatch)
	ctx.Request.SetRequestURI("/path/?key=val")
	ctx.Request.SetHost(host)
	router.Handler(ctx)
	if !(ctx.Response.StatusCode() == fasthttp.StatusPermanentRedirect && string(ctx.Response.Header.Peek("Location")) == buildLocation("/path?key=val")) {
		t.Errorf("Custom NotFound handler failed: Code=%d, Header=%v", ctx.Response.StatusCode(), ctx.Response.Header.String())
	}
	ctx.Response.Reset()

	// Test special case where no node for the prefix "/" exists
	router = New()
	router.GET("/a", handlerFunc)

	ctx.Request.Header.SetMethod(fasthttp.MethodPatch)
	ctx.Request.SetRequestURI("/")
	router.Handler(ctx)
	if !(ctx.Response.StatusCode() == fasthttp.StatusNotFound) {
		t.Errorf("NotFound handling route / failed: Code=%d", ctx.Response.StatusCode())
	}
}

func TestRouterPanicHandler(t *testing.T) {
	router := New()
	panicHandled := false

	router.PanicHandler = func(ctx *fasthttp.RequestCtx, p interface{}) {
		panicHandled = true
	}

	router.Handle(fasthttp.MethodPut, "/user/{name}", func(ctx *fasthttp.RequestCtx) {
		panic("oops!")
	})

	ctx := new(fasthttp.RequestCtx)
	ctx.Request.Header.SetMethod(fasthttp.MethodPut)
	ctx.Request.SetRequestURI("/user/gopher")

	defer func() {
		if rcv := recover(); rcv != nil {
			t.Fatal("handling panic failed")
		}
	}()

	router.Handler(ctx)

	if !panicHandled {
		t.Fatal("simulating failed")
	}
}

func TestRouterLookup(t *testing.T) {
	routed := false
	wantHandle := func(_ *fasthttp.RequestCtx) {
		routed = true
	}
	wantParams := map[string]string{"name": "gopher"}

	ctx := new(fasthttp.RequestCtx)
	router := New()

	// try empty router first
	handle, tsr := router.Lookup(fasthttp.MethodGet, "/nope", ctx)
	if handle != nil {
		t.Fatalf("Got handle for unregistered pattern: %v", handle)
	}
	if tsr {
		t.Error("Got wrong TSR recommendation!")
	}

	// insert route and try again
	router.GET("/user/{name}", wantHandle)
	handle, _ = router.Lookup(fasthttp.MethodGet, "/user/gopher", ctx)
	if handle == nil {
		t.Fatal("Got no handle!")
	} else {
		handle(nil)
		if !routed {
			t.Fatal("Routing failed!")
		}
	}

	for expectedKey, expectedVal := range wantParams {
		if ctx.UserValue(expectedKey) != expectedVal {
			t.Errorf("The values %s = %s is not save in context", expectedKey, expectedVal)
		}
	}

	routed = false

	// route without param
	router.GET("/user", wantHandle)
	handle, _ = router.Lookup(fasthttp.MethodGet, "/user", ctx)
	if handle == nil {
		t.Fatal("Got no handle!")
	} else {
		handle(nil)
		if !routed {
			t.Fatal("Routing failed!")
		}
	}

	for expectedKey, expectedVal := range wantParams {
		if ctx.UserValue(expectedKey) != expectedVal {
			t.Errorf("The values %s = %s is not save in context", expectedKey, expectedVal)
		}
	}

	handle, tsr = router.Lookup(fasthttp.MethodGet, "/user/gopher/", ctx)
	if handle != nil {
		t.Fatalf("Got handle for unregistered pattern: %v", handle)
	}
	if !tsr {
		t.Error("Got no TSR recommendation!")
	}

	handle, tsr = router.Lookup(fasthttp.MethodGet, "/nope", ctx)
	if handle != nil {
		t.Fatalf("Got handle for unregistered pattern: %v", handle)
	}
	if tsr {
		t.Error("Got wrong TSR recommendation!")
	}
}

func TestRouterMatchedRoutePath(t *testing.T) {
	route1 := "/user/{name}"
	routed1 := false
	handle1 := func(ctx *fasthttp.RequestCtx) {
		route := ctx.UserValue(MatchedRoutePathParam)
		if route != route1 {
			t.Fatalf("Wrong matched route: want %s, got %s", route1, route)
		}
		routed1 = true
	}

	route2 := "/user/{name}/details"
	routed2 := false
	handle2 := func(ctx *fasthttp.RequestCtx) {
		route := ctx.UserValue(MatchedRoutePathParam)
		if route != route2 {
			t.Fatalf("Wrong matched route: want %s, got %s", route2, route)
		}
		routed2 = true
	}

	route3 := "/"
	routed3 := false
	handle3 := func(ctx *fasthttp.RequestCtx) {
		route := ctx.UserValue(MatchedRoutePathParam)
		if route != route3 {
			t.Fatalf("Wrong matched route: want %s, got %s", route3, route)
		}
		routed3 = true
	}

	router := New()
	router.SaveMatchedRoutePath = true
	router.Handle(fasthttp.MethodGet, route1, handle1)
	router.Handle(fasthttp.MethodGet, route2, handle2)
	router.Handle(fasthttp.MethodGet, route3, handle3)

	ctx := new(fasthttp.RequestCtx)

	ctx.Request.Header.SetMethod(fasthttp.MethodGet)
	ctx.Request.SetRequestURI("/user/gopher")
	router.Handler(ctx)
	if !routed1 || routed2 || routed3 {
		t.Fatal("Routing failed!")
	}

	ctx.Request.Header.SetMethod(fasthttp.MethodGet)
	ctx.Request.SetRequestURI("/user/gopher/details")
	router.Handler(ctx)
	if !routed2 || routed3 {
		t.Fatal("Routing failed!")
	}

	ctx.Request.Header.SetMethod(fasthttp.MethodGet)
	ctx.Request.SetRequestURI("/")
	router.Handler(ctx)
	if !routed3 {
		t.Fatal("Routing failed!")
	}
}

func TestRouterServeFiles(t *testing.T) {
	r := New()

	recv := catchPanic(func() {
		r.ServeFiles("/noFilepath", os.TempDir())
	})
	if recv == nil {
		t.Fatal("registering path not ending with '{filepath:*}' did not panic")
	}
	body := []byte("fake ico")
	ioutil.WriteFile(os.TempDir()+"/favicon.ico", body, 0644)

	r.ServeFiles("/{filepath:*}", os.TempDir())

	assertWithTestServer(t, "GET /favicon.ico HTTP/1.1\r\n\r\n", r.Handler, func(rw *readWriter) {
		br := bufio.NewReader(&rw.w)
		var resp fasthttp.Response
		if err := resp.Read(br); err != nil {
			t.Fatalf("Unexpected error when reading response: %s", err)
		}
		if resp.Header.StatusCode() != 200 {
			t.Fatalf("Unexpected status code %d. Expected %d", resp.Header.StatusCode(), 200)
		}
		if !bytes.Equal(resp.Body(), body) {
			t.Fatalf("Unexpected body %q. Expected %q", resp.Body(), string(body))
		}
	})
}

func TestRouterServeFilesCustom(t *testing.T) {
	r := New()

	root := os.TempDir()

	fs := &fasthttp.FS{
		Root: root,
	}

	recv := catchPanic(func() {
		r.ServeFilesCustom("/noFilepath", fs)
	})
	if recv == nil {
		t.Fatal("registering path not ending with '{filepath:*}' did not panic")
	}
	body := []byte("fake ico")
	ioutil.WriteFile(root+"/favicon.ico", body, 0644)

	r.ServeFilesCustom("/{filepath:*}", fs)

	assertWithTestServer(t, "GET /favicon.ico HTTP/1.1\r\n\r\n", r.Handler, func(rw *readWriter) {
		br := bufio.NewReader(&rw.w)
		var resp fasthttp.Response
		if err := resp.Read(br); err != nil {
			t.Fatalf("Unexpected error when reading response: %s", err)
		}
		if resp.Header.StatusCode() != 200 {
			t.Fatalf("Unexpected status code %d. Expected %d", resp.Header.StatusCode(), 200)
		}
		if !bytes.Equal(resp.Body(), body) {
			t.Fatalf("Unexpected body %q. Expected %q", resp.Body(), string(body))
		}
	})
}

func TestRouterList(t *testing.T) {
	expected := map[string][]string{
		"GET":    {"/bar"},
		"PATCH":  {"/foo"},
		"POST":   {"/v1/users/{name}/{surname?}"},
		"DELETE": {"/v1/users/{id?}"},
	}

	r := New()
	r.GET("/bar", func(ctx *fasthttp.RequestCtx) {})
	r.PATCH("/foo", func(ctx *fasthttp.RequestCtx) {})

	v1 := r.Group("/v1")
	v1.POST("/users/{name}/{surname?}", func(ctx *fasthttp.RequestCtx) {})
	v1.DELETE("/users/{id?}", func(ctx *fasthttp.RequestCtx) {})

	result := r.List()

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Router.List() == %v, want %v", result, expected)
	}

}

func BenchmarkAllowed(b *testing.B) {
	handlerFunc := func(_ *fasthttp.RequestCtx) {}

	router := New()
	router.POST("/path", handlerFunc)
	router.GET("/path", handlerFunc)

	b.Run("Global", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = router.allowed("*", fasthttp.MethodOptions)
		}
	})
	b.Run("Path", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = router.allowed("/path", fasthttp.MethodOptions)
		}
	})
}

func BenchmarkRouterGet(b *testing.B) {
	r := New()
	r.GET("/", func(ctx *fasthttp.RequestCtx) {})

	ctx := new(fasthttp.RequestCtx)
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.SetRequestURI("/")

	for i := 0; i < b.N; i++ {
		r.Handler(ctx)
	}
}

func BenchmarkRouterANY(b *testing.B) {
	r := New()
	r.ANY("/", func(ctx *fasthttp.RequestCtx) {})

	ctx := new(fasthttp.RequestCtx)
	ctx.Request.Header.SetMethod("UNKNOWN")
	ctx.Request.SetRequestURI("/")

	for i := 0; i < b.N; i++ {
		r.Handler(ctx)
	}
}

func BenchmarkRouterNotFound(b *testing.B) {
	r := New()
	r.GET("/bench", func(ctx *fasthttp.RequestCtx) {})

	ctx := new(fasthttp.RequestCtx)
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.SetRequestURI("/notfound")

	for i := 0; i < b.N; i++ {
		r.Handler(ctx)
	}
}

func BenchmarkRouterCleanPath(b *testing.B) {
	r := New()
	r.GET("/bench", func(ctx *fasthttp.RequestCtx) {})

	ctx := new(fasthttp.RequestCtx)
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.SetRequestURI("/../bench/")

	for i := 0; i < b.N; i++ {
		r.Handler(ctx)
	}
}

func BenchmarkRouterRedirectTrailingSlash(b *testing.B) {
	r := New()
	r.GET("/bench/", func(ctx *fasthttp.RequestCtx) {})

	ctx := new(fasthttp.RequestCtx)
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.SetRequestURI("/bench")

	for i := 0; i < b.N; i++ {
		r.Handler(ctx)
	}
}
