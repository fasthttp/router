// Copyright 2013 Julien Schmidt. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.

package router

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/valyala/fasthttp"
)

func TestRouter(t *testing.T) {
	r := New()

	routed := false
	r.Handle("GET", "/user/:name", func(ctx *fasthttp.RequestCtx) {
		routed = true
		want := map[string]string{"name": "gopher"}

		if ctx.UserValue("name") != want["name"] {
			t.Fatalf("wrong wildcard values: want %v, got %v", want["name"], ctx.UserValue("name"))
		}
		ctx.Success("foo/bar", []byte("success"))
	})

	s := &fasthttp.Server{
		Handler: r.Handler,
	}

	rw := &readWriter{}
	rw.r.WriteString("GET /user/gopher?baz HTTP/1.1\r\n\r\n")

	ch := make(chan error)
	go func() {
		ch <- s.ServeConn(rw)
	}()

	select {
	case err := <-ch:
		if err != nil {
			t.Fatalf("return error %s", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("timeout")
	}

	if !routed {
		t.Fatal("routing failed")
	}
}

type handlerStruct struct {
	handeled *bool
}

func (h handlerStruct) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	*h.handeled = true
}

func TestRouterAPI(t *testing.T) {
	var get, head, options, post, put, patch, deleted bool

	r := New()
	r.GET("/GET", func(ctx *fasthttp.RequestCtx) {
		get = true
	})
	r.HEAD("/GET", func(ctx *fasthttp.RequestCtx) {
		head = true
	})
	r.OPTIONS("/GET", func(ctx *fasthttp.RequestCtx) {
		options = true
	})
	r.POST("/POST", func(ctx *fasthttp.RequestCtx) {
		post = true
	})
	r.PUT("/PUT", func(ctx *fasthttp.RequestCtx) {
		put = true
	})
	r.PATCH("/PATCH", func(ctx *fasthttp.RequestCtx) {
		patch = true
	})
	r.DELETE("/DELETE", func(ctx *fasthttp.RequestCtx) {
		deleted = true
	})

	s := &fasthttp.Server{
		Handler: r.Handler,
	}

	rw := &readWriter{}
	ch := make(chan error)

	rw.r.WriteString("GET /GET HTTP/1.1\r\n\r\n")
	go func() {
		ch <- s.ServeConn(rw)
	}()
	select {
	case err := <-ch:
		if err != nil {
			t.Fatalf("return error %s", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("timeout")
	}
	if !get {
		t.Error("routing GET failed")
	}

	rw.r.WriteString("HEAD /GET HTTP/1.1\r\n\r\n")
	go func() {
		ch <- s.ServeConn(rw)
	}()
	select {
	case err := <-ch:
		if err != nil {
			t.Fatalf("return error %s", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("timeout")
	}
	if !head {
		t.Error("routing HEAD failed")
	}

	rw.r.WriteString("OPTIONS /GET HTTP/1.1\r\n\r\n")
	go func() {
		ch <- s.ServeConn(rw)
	}()
	select {
	case err := <-ch:
		if err != nil {
			t.Fatalf("return error %s", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("timeout")
	}
	if !options {
		t.Error("routing OPTIONS failed")
	}

	rw.r.WriteString("POST /POST HTTP/1.1\r\n\r\n")
	go func() {
		ch <- s.ServeConn(rw)
	}()
	select {
	case err := <-ch:
		if err != nil {
			t.Fatalf("return error %s", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("timeout")
	}
	if !post {
		t.Error("routing POST failed")
	}

	rw.r.WriteString("PUT /PUT HTTP/1.1\r\n\r\n")
	go func() {
		ch <- s.ServeConn(rw)
	}()
	select {
	case err := <-ch:
		if err != nil {
			t.Fatalf("return error %s", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("timeout")
	}
	if !put {
		t.Error("routing PUT failed")
	}

	rw.r.WriteString("PATCH /PATCH HTTP/1.1\r\n\r\n")
	go func() {
		ch <- s.ServeConn(rw)
	}()
	select {
	case err := <-ch:
		if err != nil {
			t.Fatalf("return error %s", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("timeout")
	}
	if !patch {
		t.Error("routing PATCH failed")
	}

	rw.r.WriteString("DELETE /DELETE HTTP/1.1\r\n\r\n")
	go func() {
		ch <- s.ServeConn(rw)
	}()
	select {
	case err := <-ch:
		if err != nil {
			t.Fatalf("return error %s", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("timeout")
	}
	if !deleted {
		t.Error("routing DELETE failed")
	}
}

func TestRouterRoot(t *testing.T) {
	r := New()
	recv := catchPanic(func() {
		r.GET("noSlashRoot", nil)
	})
	if recv == nil {
		t.Fatal("registering path not beginning with '/' did not panic")
	}
}

func TestRouterChaining(t *testing.T) {
	r1 := New()
	r2 := New()
	r1.NotFound = r2.Handler

	fooHit := false
	r1.POST("/foo", func(ctx *fasthttp.RequestCtx) {
		fooHit = true
		ctx.SetStatusCode(fasthttp.StatusOK)
	})

	barHit := false
	r2.POST("/bar", func(ctx *fasthttp.RequestCtx) {
		barHit = true
		ctx.SetStatusCode(fasthttp.StatusOK)
	})

	s := &fasthttp.Server{
		Handler: r1.Handler,
	}

	rw := &readWriter{}
	ch := make(chan error)

	rw.r.WriteString("POST /foo HTTP/1.1\r\n\r\n")
	go func() {
		ch <- s.ServeConn(rw)
	}()
	select {
	case err := <-ch:
		if err != nil {
			t.Fatalf("return error %s", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("timeout")
	}
	br := bufio.NewReader(&rw.w)
	var resp fasthttp.Response
	if err := resp.Read(br); err != nil {
		t.Fatalf("Unexpected error when reading response: %s", err)
	}
	if !(resp.Header.StatusCode() == fasthttp.StatusOK && fooHit) {
		t.Errorf("Regular routing failed with router chaining.")
		t.FailNow()
	}

	rw.r.WriteString("POST /bar HTTP/1.1\r\n\r\n")
	go func() {
		ch <- s.ServeConn(rw)
	}()
	select {
	case err := <-ch:
		if err != nil {
			t.Fatalf("return error %s", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("timeout")
	}
	if err := resp.Read(br); err != nil {
		t.Fatalf("Unexpected error when reading response: %s", err)
	}
	if !(resp.Header.StatusCode() == fasthttp.StatusOK && barHit) {
		t.Errorf("Chained routing failed with router chaining.")
		t.FailNow()
	}

	rw.r.WriteString("POST /qax HTTP/1.1\r\n\r\n")
	go func() {
		ch <- s.ServeConn(rw)
	}()
	select {
	case err := <-ch:
		if err != nil {
			t.Fatalf("return error %s", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("timeout")
	}
	if err := resp.Read(br); err != nil {
		t.Fatalf("Unexpected error when reading response: %s", err)
	}
	if !(resp.Header.StatusCode() == fasthttp.StatusNotFound) {
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
	r6.ServeFiles("/static/*filepath", "./")
	r6.ServeFilesCustom("/custom/static/*filepath", &fasthttp.FS{Root: "./"})

	s := &fasthttp.Server{
		Handler: r1.Handler,
	}

	rw := &readWriter{}
	ch := make(chan error)

	requests := []string{
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

	for _, req := range requests {
		hit = false

		rw.r.WriteString(req)
		go func() {
			ch <- s.ServeConn(rw)
		}()
		select {
		case err := <-ch:
			if err != nil {
				t.Fatalf("return error %s", err)
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("timeout")
		}
		br := bufio.NewReader(&rw.w)
		var resp fasthttp.Response
		if err := resp.Read(br); err != nil {
			t.Fatalf("Unexpected error when reading response: %s", err)
		}
		if !(resp.Header.StatusCode() == fasthttp.StatusOK) {
			t.Fatalf("Status code %d, want %d", resp.Header.StatusCode(), fasthttp.StatusOK)
		}
		if !strings.Contains(req, "static") && !hit {
			t.Fatalf("Regular routing failed with router chaining. %s", req)
		}
	}

	// Not found
	rw.r.WriteString("POST /qax HTTP/1.1\r\n\r\n")
	go func() {
		ch <- s.ServeConn(rw)
	}()
	select {
	case err := <-ch:
		if err != nil {
			t.Fatalf("return error %s", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("timeout")
	}
	br := bufio.NewReader(&rw.w)
	var resp fasthttp.Response
	if err := resp.Read(br); err != nil {
		t.Fatalf("Unexpected error when reading response: %s", err)
	}
	if !(resp.Header.StatusCode() == fasthttp.StatusNotFound) {
		t.Errorf("NotFound behavior failed with router chaining.")
		t.FailNow()
	}
}

func TestRouterOPTIONS(t *testing.T) {
	// TODO: because fasthttp is not support OPTIONS method now,
	// these test cases will be used in the future.
	handlerFunc := func(_ *fasthttp.RequestCtx) {}

	r := New()
	r.POST("/path", handlerFunc)

	// test not allowed
	// * (server)
	s := &fasthttp.Server{
		Handler: r.Handler,
	}

	rw := &readWriter{}
	ch := make(chan error)

	rw.r.WriteString("OPTIONS * HTTP/1.1\r\nHost:\r\n\r\n")
	go func() {
		ch <- s.ServeConn(rw)
	}()
	select {
	case err := <-ch:
		if err != nil {
			t.Fatalf("return error %s", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("timeout")
	}
	br := bufio.NewReader(&rw.w)
	var resp fasthttp.Response
	if err := resp.Read(br); err != nil {
		t.Fatalf("Unexpected error when reading response: %s", err)
	}
	if resp.Header.StatusCode() != fasthttp.StatusOK {
		t.Errorf("OPTIONS handling failed: Code=%d, Header=%v",
			resp.Header.StatusCode(), resp.Header.String())
	} else if allow := string(resp.Header.Peek("Allow")); allow != "POST, OPTIONS" {
		t.Error("unexpected Allow header value: " + allow)
	}

	// path
	rw.r.WriteString("OPTIONS /path HTTP/1.1\r\n\r\n")
	go func() {
		ch <- s.ServeConn(rw)
	}()
	select {
	case err := <-ch:
		if err != nil {
			t.Fatalf("return error %s", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("timeout")
	}
	if err := resp.Read(br); err != nil {
		t.Fatalf("Unexpected error when reading response: %s", err)
	}
	if resp.Header.StatusCode() != fasthttp.StatusOK {
		t.Errorf("OPTIONS handling failed: Code=%d, Header=%v",
			resp.Header.StatusCode(), resp.Header.String())
	} else if allow := string(resp.Header.Peek("Allow")); allow != "POST, OPTIONS" {
		t.Error("unexpected Allow header value: " + allow)
	}

	rw.r.WriteString("OPTIONS /doesnotexist HTTP/1.1\r\n\r\n")
	go func() {
		ch <- s.ServeConn(rw)
	}()
	select {
	case err := <-ch:
		if err != nil {
			t.Fatalf("return error %s", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("timeout")
	}
	if err := resp.Read(br); err != nil {
		t.Fatalf("Unexpected error when reading response: %s", err)
	}
	if !(resp.Header.StatusCode() == fasthttp.StatusNotFound) {
		t.Errorf("OPTIONS handling failed: Code=%d, Header=%v",
			resp.Header.StatusCode(), resp.Header.String())
	}

	// add another method
	r.GET("/path", handlerFunc)

	// test again
	// * (server)
	rw.r.WriteString("OPTIONS * HTTP/1.1\r\n\r\n")
	go func() {
		ch <- s.ServeConn(rw)
	}()
	select {
	case err := <-ch:
		if err != nil {
			t.Fatalf("return error %s", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("timeout")
	}
	if err := resp.Read(br); err != nil {
		t.Fatalf("Unexpected error when reading response: %s", err)
	}
	if resp.Header.StatusCode() != fasthttp.StatusOK {
		t.Errorf("OPTIONS handling failed: Code=%d, Header=%v",
			resp.Header.StatusCode(), resp.Header.String())
	} else if allow := string(resp.Header.Peek("Allow")); allow != "POST, GET, OPTIONS" && allow != "GET, POST, OPTIONS" {
		t.Error("unexpected Allow header value: " + allow)
	}

	// path
	rw.r.WriteString("OPTIONS /path HTTP/1.1\r\n\r\n")
	go func() {
		ch <- s.ServeConn(rw)
	}()
	select {
	case err := <-ch:
		if err != nil {
			t.Fatalf("return error %s", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("timeout")
	}
	if err := resp.Read(br); err != nil {
		t.Fatalf("Unexpected error when reading response: %s", err)
	}
	if resp.Header.StatusCode() != fasthttp.StatusOK {
		t.Errorf("OPTIONS handling failed: Code=%d, Header=%v",
			resp.Header.StatusCode(), resp.Header.String())
	} else if allow := string(resp.Header.Peek("Allow")); allow != "POST, GET, OPTIONS" && allow != "GET, POST, OPTIONS" {
		t.Error("unexpected Allow header value: " + allow)
	}

	// custom handler
	var custom bool
	r.OPTIONS("/path", func(_ *fasthttp.RequestCtx) {
		custom = true
	})

	// test again
	// * (server)
	rw.r.WriteString("OPTIONS * HTTP/1.1\r\n\r\n")
	go func() {
		ch <- s.ServeConn(rw)
	}()
	select {
	case err := <-ch:
		if err != nil {
			t.Fatalf("return error %s", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("timeout")
	}
	if err := resp.Read(br); err != nil {
		t.Fatalf("Unexpected error when reading response: %s", err)
	}
	if resp.Header.StatusCode() != fasthttp.StatusOK {
		t.Errorf("OPTIONS handling failed: Code=%d, Header=%v",
			resp.Header.StatusCode(), resp.Header.String())
	} else if allow := string(resp.Header.Peek("Allow")); allow != "POST, GET, OPTIONS" && allow != "GET, POST, OPTIONS" {
		t.Error("unexpected Allow header value: " + allow)
	}
	if custom {
		t.Error("custom handler called on *")
	}

	// path
	rw.r.WriteString("OPTIONS /path HTTP/1.1\r\n\r\n")
	go func() {
		ch <- s.ServeConn(rw)
	}()
	select {
	case err := <-ch:
		if err != nil {
			t.Fatalf("return error %s", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("timeout")
	}
	if err := resp.Read(br); err != nil {
		t.Fatalf("Unexpected error when reading response: %s", err)
	}
	if resp.Header.StatusCode() != fasthttp.StatusOK {
		t.Errorf("OPTIONS handling failed: Code=%d, Header=%v",
			resp.Header.StatusCode(), resp.Header.String())
	}
	if !custom {
		t.Error("custom handler not called")
	}
}

func TestRouterNotAllowed(t *testing.T) {
	handlerFunc := func(_ *fasthttp.RequestCtx) {}

	r := New()
	r.POST("/path", handlerFunc)

	// Test not allowed
	s := &fasthttp.Server{
		Handler: r.Handler,
	}

	rw := &readWriter{}
	ch := make(chan error)

	rw.r.WriteString("GET /path HTTP/1.1\r\n\r\n")
	go func() {
		ch <- s.ServeConn(rw)
	}()
	select {
	case err := <-ch:
		if err != nil {
			t.Fatalf("return error %s", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("timeout")
	}
	br := bufio.NewReader(&rw.w)
	var resp fasthttp.Response
	if err := resp.Read(br); err != nil {
		t.Fatalf("Unexpected error when reading response: %s", err)
	}
	if !(resp.Header.StatusCode() == fasthttp.StatusMethodNotAllowed) {
		t.Errorf("NotAllowed handling failed: Code=%d", resp.Header.StatusCode())
	} else if allow := string(resp.Header.Peek("Allow")); allow != "POST, OPTIONS" {
		t.Error("unexpected Allow header value: " + allow)
	}

	// add another method
	r.DELETE("/path", handlerFunc)
	r.OPTIONS("/path", handlerFunc) // must be ignored

	// test again
	rw.r.WriteString("GET /path HTTP/1.1\r\n\r\n")
	go func() {
		ch <- s.ServeConn(rw)
	}()
	select {
	case err := <-ch:
		if err != nil {
			t.Fatalf("return error %s", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("timeout")
	}
	if err := resp.Read(br); err != nil {
		t.Fatalf("Unexpected error when reading response: %s", err)
	}
	if !(resp.Header.StatusCode() == fasthttp.StatusMethodNotAllowed) {
		t.Errorf("NotAllowed handling failed: Code=%d", resp.Header.StatusCode())
	} else if allow := string(resp.Header.Peek("Allow")); allow != "POST, DELETE, OPTIONS" && allow != "DELETE, POST, OPTIONS" {
		t.Error("unexpected Allow header value: " + allow)
	}

	responseText := "custom method"
	r.MethodNotAllowed = fasthttp.RequestHandler(func(ctx *fasthttp.RequestCtx) {
		ctx.SetStatusCode(fasthttp.StatusTeapot)
		ctx.Write([]byte(responseText))
	})
	rw.r.WriteString("GET /path HTTP/1.1\r\n\r\n")
	go func() {
		ch <- s.ServeConn(rw)
	}()
	select {
	case err := <-ch:
		if err != nil {
			t.Fatalf("return error %s", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("timeout")
	}
	if err := resp.Read(br); err != nil {
		t.Fatalf("Unexpected error when reading response: %s", err)
	}
	if !bytes.Equal(resp.Body(), []byte(responseText)) {
		t.Errorf("unexpected response got %q want %q", string(resp.Body()), responseText)
	}
	if resp.Header.StatusCode() != fasthttp.StatusTeapot {
		t.Errorf("unexpected response code %d want %d", resp.Header.StatusCode(), fasthttp.StatusTeapot)
	}
	if allow := string(resp.Header.Peek("Allow")); allow != "POST, DELETE, OPTIONS" && allow != "DELETE, POST, OPTIONS" {
		t.Error("unexpected Allow header value: " + allow)
	}
}

func TestRouterNotFound(t *testing.T) {
	handlerFunc := func(_ *fasthttp.RequestCtx) {}

	r := New()
	r.GET("/path", handlerFunc)
	r.GET("/dir/", handlerFunc)
	r.GET("/", handlerFunc)

	testRoutes := []struct {
		route string
		code  int
	}{
		{"/path/", 301},          // TSR -/
		{"/dir", 301},            // TSR +/
		{"/", 200},               // TSR +/
		{"/PATH", 301},           // Fixed Case
		{"/DIR", 301},            // Fixed Case
		{"/PATH/", 301},          // Fixed Case -/
		{"/DIR/", 301},           // Fixed Case +/
		{"/paTh/?name=foo", 301}, // Fixed Case With Params +/
		{"/paTh?name=foo", 301},  // Fixed Case With Params +/
		{"/../path", 200},        // CleanPath (Not clean by router, this path is cleaned by fasthttp `ctx.Path()`)
		{"/nope", 404},           // NotFound
	}

	s := &fasthttp.Server{
		Handler: r.Handler,
	}

	rw := &readWriter{}
	br := bufio.NewReader(&rw.w)
	var resp fasthttp.Response
	ch := make(chan error)
	for _, tr := range testRoutes {
		rw.r.WriteString(fmt.Sprintf("GET %s HTTP/1.1\r\n\r\n", tr.route))
		go func() {
			ch <- s.ServeConn(rw)
		}()
		select {
		case err := <-ch:
			if err != nil {
				t.Fatalf("return error %s", err)
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("timeout")
		}
		if err := resp.Read(br); err != nil {
			t.Fatalf("Unexpected error when reading response: %s", err)
		}
		if !(resp.Header.StatusCode() == tr.code) {
			t.Errorf("NotFound handling route %s failed: Code=%d want=%d",
				tr.route, resp.Header.StatusCode(), tr.code)
		}
	}

	// Test custom not found handler
	var notFound bool
	r.NotFound = fasthttp.RequestHandler(func(ctx *fasthttp.RequestCtx) {
		ctx.SetStatusCode(404)
		notFound = true
	})
	rw.r.WriteString("GET /nope HTTP/1.1\r\n\r\n")
	go func() {
		ch <- s.ServeConn(rw)
	}()
	select {
	case err := <-ch:
		if err != nil {
			t.Fatalf("return error %s", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("timeout")
	}
	if err := resp.Read(br); err != nil {
		t.Fatalf("Unexpected error when reading response: %s", err)
	}
	if !(resp.Header.StatusCode() == 404 && notFound == true) {
		t.Errorf("Custom NotFound handler failed: Code=%d, Header=%v", resp.Header.StatusCode(), string(resp.Header.Peek("Location")))
	}

	// Test other method than GET (want 307 instead of 301)
	r.PATCH("/path", handlerFunc)
	rw.r.WriteString("PATCH /path/ HTTP/1.1\r\n\r\n")
	go func() {
		ch <- s.ServeConn(rw)
	}()
	select {
	case err := <-ch:
		if err != nil {
			t.Fatalf("return error %s", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("timeout")
	}
	if err := resp.Read(br); err != nil {
		t.Fatalf("Unexpected error when reading response: %s", err)
	}
	if !(resp.Header.StatusCode() == 307) {
		t.Errorf("Custom NotFound handler failed: Code=%d, Header=%v", resp.Header.StatusCode(), string(resp.Header.Peek("Location")))
	}

	// Test special case where no node for the prefix "/" exists
	r = New()
	r.GET("/a", handlerFunc)
	s.Handler = r.Handler
	rw.r.WriteString("GET / HTTP/1.1\r\n\r\n")
	go func() {
		ch <- s.ServeConn(rw)
	}()
	select {
	case err := <-ch:
		if err != nil {
			t.Fatalf("return error %s", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("timeout")
	}
	if err := resp.Read(br); err != nil {
		t.Fatalf("Unexpected error when reading response: %s", err)
	}
	if !(resp.Header.StatusCode() == 404) {
		t.Errorf("NotFound handling route / failed: Code=%d", resp.Header.StatusCode())
	}
}

func TestRouterPanicHandler(t *testing.T) {
	r := New()
	panicHandled := false

	r.PanicHandler = func(ctx *fasthttp.RequestCtx, p interface{}) {
		panicHandled = true
	}

	r.Handle("PUT", "/user/:name", func(_ *fasthttp.RequestCtx) {
		panic("oops!")
	})

	defer func() {
		if rcv := recover(); rcv != nil {
			t.Fatal("handling panic failed")
		}
	}()

	s := &fasthttp.Server{
		Handler: r.Handler,
	}

	rw := &readWriter{}
	ch := make(chan error)

	rw.r.WriteString(string("PUT /user/gopher HTTP/1.1\r\n\r\n"))
	go func() {
		ch <- s.ServeConn(rw)
	}()
	select {
	case err := <-ch:
		if err != nil {
			t.Fatalf("return error %s", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("timeout")
	}

	if !panicHandled {
		t.Fatal("simulating failed")
	}
}

func TestRouterLookup(t *testing.T) {
	routed := false
	wantHandle := func(_ *fasthttp.RequestCtx) {
		routed = true
	}

	r := New()
	ctx := &fasthttp.RequestCtx{}

	// try empty router first
	handle, tsr := r.Lookup("GET", "/nope", ctx)
	if handle != nil {
		t.Fatalf("Got handle for unregistered pattern: %v", handle)
	}
	if tsr {
		t.Error("Got wrong TSR recommendation!")
	}

	// insert route and try again
	r.GET("/user/:name", wantHandle)

	handle, _ = r.Lookup("GET", "/user/gopher", ctx)
	if handle == nil {
		t.Fatal("Got no handle!")
	} else {
		handle(nil)
		if !routed {
			t.Fatal("Routing failed!")
		}
	}
	if ctx.UserValue("name") != "gopher" {
		t.Error("Param not set!")
	}

	handle, tsr = r.Lookup("GET", "/user/gopher/", ctx)
	if handle != nil {
		t.Fatalf("Got handle for unregistered pattern: %v", handle)
	}
	if !tsr {
		t.Error("Got no TSR recommendation!")
	}

	handle, tsr = r.Lookup("GET", "/nope", ctx)
	if handle != nil {
		t.Fatalf("Got handle for unregistered pattern: %v", handle)
	}
	if tsr {
		t.Error("Got wrong TSR recommendation!")
	}
}

func TestRouterServeFiles(t *testing.T) {
	r := New()

	recv := catchPanic(func() {
		r.ServeFiles("/noFilepath", os.TempDir())
	})
	if recv == nil {
		t.Fatal("registering path not ending with '*filepath' did not panic")
	}
	body := []byte("fake ico")
	ioutil.WriteFile(os.TempDir()+"/favicon.ico", body, 0644)

	r.ServeFiles("/*filepath", os.TempDir())

	s := &fasthttp.Server{
		Handler: r.Handler,
	}

	rw := &readWriter{}
	ch := make(chan error)

	rw.r.WriteString(string("GET /favicon.ico HTTP/1.1\r\n\r\n"))
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

	br := bufio.NewReader(&rw.w)
	var resp fasthttp.Response
	if err := resp.Read(br); err != nil {
		t.Fatalf("Unexpected error when reading response: %s", err)
	}
	if resp.Header.StatusCode() != 200 {
		t.Fatalf("Unexpected status code %d. Expected %d", resp.Header.StatusCode(), 423)
	}
	if !bytes.Equal(resp.Body(), body) {
		t.Fatalf("Unexpected body %q. Expected %q", resp.Body(), string(body))
	}
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
		t.Fatal("registering path not ending with '*filepath' did not panic")
	}
	body := []byte("fake ico")
	ioutil.WriteFile(root+"/favicon.ico", body, 0644)

	r.ServeFilesCustom("/*filepath", fs)

	s := &fasthttp.Server{
		Handler: r.Handler,
	}

	rw := &readWriter{}
	ch := make(chan error)

	rw.r.WriteString(string("GET /favicon.ico HTTP/1.1\r\n\r\n"))
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
}

func TestRouterList(t *testing.T) {
	expected := map[string][]string{
		"GET":    []string{"/bar"},
		"PATCH":  []string{"/foo"},
		"POST":   []string{"/v1/users/:name/:surname?"},
		"DELETE": []string{"/v1/users/:id?"},
	}

	r := New()
	r.GET("/bar", func(ctx *fasthttp.RequestCtx) {})
	r.PATCH("/foo", func(ctx *fasthttp.RequestCtx) {})

	v1 := r.Group("/v1")
	v1.POST("/users/:name/:surname?", func(ctx *fasthttp.RequestCtx) {})
	v1.DELETE("/users/:id?", func(ctx *fasthttp.RequestCtx) {})

	result := r.List()

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Router.List() == %v, want %v", result, expected)
	}

}

type readWriter struct {
	net.Conn
	r bytes.Buffer
	w bytes.Buffer
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

func BenchmarkRouterGet(b *testing.B) {
	resp := []byte("Bench GET")

	r := New()
	r.GET("/bench", func(ctx *fasthttp.RequestCtx) {
		ctx.Success("text/plain", resp)
	})

	ctx := new(fasthttp.RequestCtx)
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.SetRequestURI("/bench")

	for i := 0; i < b.N; i++ {
		r.Handler(ctx)
	}
}

func BenchmarkRouterNotFound(b *testing.B) {
	resp := []byte("Bench Not Found")

	r := New()
	r.GET("/bench", func(ctx *fasthttp.RequestCtx) {
		ctx.Success("text/plain", resp)
	})

	ctx := new(fasthttp.RequestCtx)
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.SetRequestURI("/notfound")

	for i := 0; i < b.N; i++ {
		r.Handler(ctx)
	}
}

func BenchmarkRouterCleanPath(b *testing.B) {
	resp := []byte("Bench GET")

	r := New()
	r.GET("/bench", func(ctx *fasthttp.RequestCtx) {
		ctx.Success("text/plain", resp)
	})

	ctx := new(fasthttp.RequestCtx)
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.SetRequestURI("/../bench/")

	for i := 0; i < b.N; i++ {
		r.Handler(ctx)
	}
}

func BenchmarkRouterRedirectTrailingSlash(b *testing.B) {
	resp := []byte("Bench GET")

	r := New()
	r.GET("/bench/", func(ctx *fasthttp.RequestCtx) {
		ctx.Success("text/plain", resp)
	})

	ctx := new(fasthttp.RequestCtx)
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.SetRequestURI("/bench")

	for i := 0; i < b.N; i++ {
		r.Handler(ctx)
	}
}
