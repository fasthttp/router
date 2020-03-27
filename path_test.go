// Copyright 2013 Julien Schmidt. All rights reserved.
// Based on the path package, Copyright 2009 The Go Authors.
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.

package router

import (
	"reflect"
	"strings"
	"testing"

	"github.com/valyala/fasthttp"
)

type cleanPathTest struct {
	path, result string
}

var cleanTests = []cleanPathTest{
	// Already clean
	{"/", "/"},
	{"/abc", "/abc"},
	{"/a/b/c", "/a/b/c"},
	{"/abc/", "/abc/"},
	{"/a/b/c/", "/a/b/c/"},

	// missing root
	{"", "/"},
	{"a/", "/a/"},
	{"abc", "/abc"},
	{"abc/def", "/abc/def"},
	{"a/b/c", "/a/b/c"},

	// Remove doubled slash
	{"//", "/"},
	{"/abc//", "/abc/"},
	{"/abc/def//", "/abc/def/"},
	{"/a/b/c//", "/a/b/c/"},
	{"/abc//def//ghi", "/abc/def/ghi"},
	{"//abc", "/abc"},
	{"///abc", "/abc"},
	{"//abc//", "/abc/"},

	// Remove . elements
	{".", "/"},
	{"./", "/"},
	{"/abc/./def", "/abc/def"},
	{"/./abc/def", "/abc/def"},
	{"/abc/.", "/abc/"},

	// Remove .. elements
	{"..", "/"},
	{"../", "/"},
	{"../../", "/"},
	{"../..", "/"},
	{"../../abc", "/abc"},
	{"/abc/def/ghi/../jkl", "/abc/def/jkl"},
	{"/abc/def/../ghi/../jkl", "/abc/jkl"},
	{"/abc/def/..", "/abc"},
	{"/abc/def/../..", "/"},
	{"/abc/def/../../..", "/"},
	{"/abc/def/../../..", "/"},
	{"/abc/def/../../../ghi/jkl/../../../mno", "/mno"},

	// Combinations
	{"abc/./../def", "/def"},
	{"abc//./../def", "/def"},
	{"abc/../../././../def", "/def"},
}

func TestPathClean(t *testing.T) {
	for _, test := range cleanTests {
		if s := CleanPath(test.path); s != test.result {
			t.Errorf("CleanPath(%q) = %q, want %q", test.path, s, test.result)
		}
		if s := CleanPath(test.result); s != test.result {
			t.Errorf("CleanPath(%q) = %q, want %q", test.result, s, test.result)
		}
	}
}

func TestPathCleanMallocs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping malloc count in short mode")
	}

	for _, test := range cleanTests {
		allocs := testing.AllocsPerRun(100, func() { CleanPath(test.result) })
		if allocs > 0 {
			t.Errorf("CleanPath(%q): %v allocs, want zero", test.result, allocs)
		}
	}
}

func BenchmarkPathClean(b *testing.B) {
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		for _, test := range cleanTests {
			CleanPath(test.path)
		}
	}
}

func genLongPaths() (testPaths []cleanPathTest) {
	for i := 1; i <= 1234; i++ {
		ss := strings.Repeat("a", i)

		correctPath := "/" + ss
		testPaths = append(testPaths, cleanPathTest{
			path:   correctPath,
			result: correctPath,
		}, cleanPathTest{
			path:   ss,
			result: correctPath,
		}, cleanPathTest{
			path:   "//" + ss,
			result: correctPath,
		}, cleanPathTest{
			path:   "/" + ss + "/b/..",
			result: correctPath,
		})
	}
	return
}

func TestPathCleanLong(t *testing.T) {
	cleanTests := genLongPaths()

	for _, test := range cleanTests {
		if s := CleanPath(test.path); s != test.result {
			t.Errorf("CleanPath(%q) = %q, want %q", test.path, s, test.result)
		}
		if s := CleanPath(test.result); s != test.result {
			t.Errorf("CleanPath(%q) = %q, want %q", test.result, s, test.result)
		}
	}
}

func TestGetOptionalPath(t *testing.T) {
	handler := func(ctx *fasthttp.RequestCtx) {
		ctx.SetStatusCode(fasthttp.StatusOK)
	}

	expected := []struct {
		path    string
		tsr     bool
		handler fasthttp.RequestHandler
	}{
		{"/show/{name}", true, nil},
		{"/show/{name}/", false, handler},
		{"/show/{name}/{surname}", true, nil},
		{"/show/{name}/{surname}/", false, handler},
		{"/show/{name}/{surname}/at", true, nil},
		{"/show/{name}/{surname}/at/", false, handler},
		{"/show/{name}/{surname}/at/{address}", true, nil},
		{"/show/{name}/{surname}/at/{address}/", false, handler},
		{"/show/{name}/{surname}/at/{address}/{id}", true, nil},
		{"/show/{name}/{surname}/at/{address}/{id}/", false, handler},
		{"/show/{name}/{surname}/at/{address}/{id}/{phone:.*}", false, handler},
		{"/show/{name}/{surname}/at/{address}/{id}/{phone:.*}/", true, nil},
	}

	r := New()
	r.GET("/show/{name}/{surname?}/at/{address?}/{id}/{phone?:.*}", handler)

	for _, e := range expected {
		ctx := new(fasthttp.RequestCtx)

		h, tsr := r.Lookup("GET", e.path, ctx)

		if tsr != e.tsr {
			t.Errorf("TSR (path: %s) == %v, want %v", e.path, tsr, e.tsr)
		}

		if reflect.ValueOf(h).Pointer() != reflect.ValueOf(e.handler).Pointer() {
			t.Errorf("Handler (path: %s) == %p, want %p", e.path, h, e.handler)
		}
	}
}

func BenchmarkPathCleanLong(b *testing.B) {
	cleanTests := genLongPaths()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		for _, test := range cleanTests {
			CleanPath(test.path)
		}
	}
}
