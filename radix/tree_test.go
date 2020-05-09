// Copyright 2020-present Sergio Andres Virviescas Santana, fasthttp
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.
package radix

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"

	"github.com/savsgio/gotils"
	"github.com/valyala/bytebufferpool"
	"github.com/valyala/fasthttp"
)

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
}

func randomHTTPMethod() string {
	return httpMethods[rand.Intn(len(httpMethods)-1)]
}

func generateHandler() fasthttp.RequestHandler {
	hex := gotils.RandBytes(make([]byte, 10))

	return func(ctx *fasthttp.RequestCtx) {
		ctx.Write(hex)
	}
}

func testHandlerAndParams(
	t *testing.T, tree *Tree, reqMethod, reqPath string, handler fasthttp.RequestHandler, wantTSR bool, params map[string]interface{},
) {
	for _, ctx := range []*fasthttp.RequestCtx{new(fasthttp.RequestCtx), nil} {

		h, tsr := tree.Get(reqMethod, reqPath, ctx)
		if reflect.ValueOf(handler).Pointer() != reflect.ValueOf(h).Pointer() {
			t.Errorf("Method '%s' Path '%s' handler == %p, want %p", reqMethod, reqPath, h, handler)
		}

		if wantTSR != tsr {
			t.Errorf("Method '%s' Path '%s' tsr == %v, want %v", reqMethod, reqPath, tsr, wantTSR)
		}

		if ctx != nil {
			resultParams := make(map[string]interface{})
			if params == nil {
				params = make(map[string]interface{})
			}

			ctx.VisitUserValues(func(key []byte, value interface{}) {
				resultParams[string(key)] = value
			})

			if !reflect.DeepEqual(resultParams, params) {
				t.Errorf("Method '%s' Path '%s' User values == %v, want %v", reqMethod, reqPath, resultParams, params)
			}
		}
	}
}

func Test_Tree(t *testing.T) {
	type args struct {
		method  string
		path    string
		reqPath string
		handler fasthttp.RequestHandler
	}

	type want struct {
		tsr    bool
		params map[string]interface{}
	}

	tests := []struct {
		args args
		want want
	}{
		{
			args: args{
				method:  randomHTTPMethod(),
				path:    "/users/{name}",
				reqPath: "/users/atreugo",
				handler: generateHandler(),
			},
			want: want{
				params: map[string]interface{}{
					"name": "atreugo",
				},
			},
		},
		{
			args: args{
				method:  randomHTTPMethod(),
				path:    "/users",
				reqPath: "/users",
				handler: generateHandler(),
			},
			want: want{
				params: nil,
			},
		},
		{
			args: args{
				method:  randomHTTPMethod(),
				path:    "/",
				reqPath: "/",
				handler: generateHandler(),
			},
			want: want{
				params: nil,
			},
		},
		{
			args: args{
				method:  randomHTTPMethod(),
				path:    "/users/{name}/jobs",
				reqPath: "/users/atreugo/jobs",
				handler: generateHandler(),
			},
			want: want{
				params: map[string]interface{}{
					"name": "atreugo",
				},
			},
		},
		{
			args: args{
				method:  randomHTTPMethod(),
				path:    "/users/admin",
				reqPath: "/users/admin",
				handler: generateHandler(),
			},
			want: want{
				params: nil,
			},
		},
		{
			args: args{
				method:  randomHTTPMethod(),
				path:    "/users/{status}/proc",
				reqPath: "/users/active/proc",
				handler: generateHandler(),
			},
			want: want{
				params: map[string]interface{}{
					"status": "active",
				},
			},
		},
		{
			args: args{
				method:  randomHTTPMethod(),
				path:    "/static/{filepath:*}",
				reqPath: "/static/assets/js/main.js",
				handler: generateHandler(),
			},
			want: want{
				params: map[string]interface{}{
					"filepath": "assets/js/main.js",
				},
			},
		},
	}

	tree := New()

	for _, test := range tests {
		tree.Add(test.args.method, test.args.path, test.args.handler)
	}

	for _, test := range tests {
		testHandlerAndParams(t, tree, test.args.method, test.args.reqPath, test.args.handler, false, test.want.params)
		testHandlerAndParams(t, tree, "NOTFOUND", test.args.reqPath, nil, false, nil)
	}

	for _, method := range httpMethods {
		testHandlerAndParams(t, tree, method, "/fuck/notfound", nil, false, nil)
	}

	filepathHandler := generateHandler()
	filepathMethod := randomHTTPMethod()

	tree.Add(filepathMethod, "/{filepath:*}", filepathHandler)

	testHandlerAndParams(t, tree, filepathMethod, "/js/main.js", filepathHandler, false, map[string]interface{}{
		"filepath": "js/main.js",
	})
}

func Test_Get(t *testing.T) {
	handler := generateHandler()

	for _, method := range httpMethods {
		tree := New()
		tree.Add(method, "/api", handler)
		tree.Add(method, "/api/users", handler)

		testHandlerAndParams(t, tree, method, "/api/", nil, true, nil)

		testHandlerAndParams(t, tree, method, "/a", nil, false, nil)
		testHandlerAndParams(t, tree, method, "/api/user", nil, false, nil)
	}

	for _, method := range httpMethods {
		tree := New()
		tree.Add(method, "/api/", handler)

		testHandlerAndParams(t, tree, method, "/api", nil, true, nil)
		testHandlerAndParams(t, tree, method, "/api/", handler, false, nil)
		testHandlerAndParams(t, tree, method, "/data", nil, false, nil)
	}
}

func Test_AddWithParam(t *testing.T) {
	handler := generateHandler()

	for _, method := range httpMethods {
		tree := New()
		tree.Add(method, "/test", handler)
		tree.Add(method, "/api/prefix{version:V[0-9]}_{name:[a-z]+}_sufix/files", handler)
		tree.Add(method, "/api/prefix{version:V[0-9]}_{name:[a-z]+}_sufix/data", handler)
		tree.Add(method, "/api/prefix/files", handler)
		tree.Add(method, "/prefix{name:[a-z]+}suffix/data", handler)
		tree.Add(method, "/prefix{name:[a-z]+}/data", handler)
		tree.Add(method, "/api/{file}.json", handler)

		testHandlerAndParams(t, tree, method, "/api/prefixV1_atreugo_sufix/files", handler, false, map[string]interface{}{
			"version": "V1", "name": "atreugo",
		})
		testHandlerAndParams(t, tree, method, "/api/prefixV1_atreugo_sufix/data", handler, false, map[string]interface{}{
			"version": "V1", "name": "atreugo",
		})
		testHandlerAndParams(t, tree, method, "/prefixatreugosuffix/data", handler, false, map[string]interface{}{
			"name": "atreugo",
		})
		testHandlerAndParams(t, tree, method, "/prefixatreugo/data", handler, false, map[string]interface{}{
			"name": "atreugo",
		})
		testHandlerAndParams(t, tree, method, "/api/name.json", handler, false, map[string]interface{}{
			"file": "name",
		})

		// Not found
		testHandlerAndParams(t, tree, method, "/api/prefixV1_1111_sufix/fake", nil, false, nil)
	}

}

func Test_TreeRootWildcard(t *testing.T) {
	handler := generateHandler()

	for _, method := range httpMethods {
		tree := New()
		tree.Add(method, "/{filepath:*}", handler)

		testHandlerAndParams(t, tree, method, "/", handler, false, map[string]interface{}{
			"filepath": "/",
		})
	}
}

func Test_TreeNilHandler(t *testing.T) {
	const panicMsg = "nil handler"

	tree := New()

	err := catchPanic(func() {
		tree.Add(randomHTTPMethod(), "/", nil)
	})

	if err == nil {
		t.Fatal("Expected panic")
	}

	if err != nil && panicMsg != fmt.Sprint(err) {
		t.Errorf("Invalid conflict error text (%v)", err)
	}
}

func Benchmark_Get(b *testing.B) {
	tree := New()
	method := "GET"

	// for i := 0; i < 3000; i++ {
	// 	tree.Add(
	// 		method, fmt.Sprintf("/%s", gotils.RandBytes(make([]byte, 15))), generateHandler(),
	// 	)
	// }

	tree.Add(method, "/plaintext", generateHandler())
	tree.Add(method, "/json", generateHandler())
	tree.Add(method, "/fortune", generateHandler())
	tree.Add(method, "/fortune-quick", generateHandler())
	tree.Add(method, "/db", generateHandler())
	tree.Add(method, "/queries", generateHandler())
	tree.Add(method, "/update", generateHandler())

	ctx := new(fasthttp.RequestCtx)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		tree.Get(method, "/update", ctx)
	}
}

func Benchmark_GetWithRegex(b *testing.B) {
	method := randomHTTPMethod()

	tree := New()
	ctx := new(fasthttp.RequestCtx)

	tree.Add(method, "/api/{version:v[0-9]}/data", generateHandler())

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		tree.Get(method, "/api/v1/data", ctx)
	}
}

func Benchmark_GetWithParams(b *testing.B) {
	method := randomHTTPMethod()

	tree := New()
	ctx := new(fasthttp.RequestCtx)

	tree.Add(method, "/api/{version}/data", generateHandler())

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		tree.Get(method, "/api/v1/data", ctx)
	}
}

func Benchmark_FindCaseInsensitivePath(b *testing.B) {
	method := randomHTTPMethod()

	tree := New()
	buf := bytebufferpool.Get()

	tree.Add(method, "/endpoint", generateHandler())

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		tree.FindCaseInsensitivePath(method, "/ENdpOiNT", false, buf)
		buf.Reset()
	}
}

// func Test_foo(t *testing.T) {
// 	tree := New()
// 	tree.Add("GET", "/data", generateHandler())
// 	tree.Add("GET", "/data/pacp", generateHandler())
// 	tree.Add("GET", "/data/eeee", generateHandler())
// 	// tree.Add("GET", "/data", generateHandler())
// 	tree.Add("GET", "/data/pepe", generateHandler())
// 	tree.Add("GET", "/da/juan", generateHandler())
// 	tree.Add("GET", "/", generateHandler())
// 	tree.Add("GET", "/{filepath:*}", generateHandler())
// 	// tree.Add("GET", "/{param:*}", generateHandler())
// 	tree.Add("GET", "/{param}/", generateHandler())
// 	// tree.Add("GET", "/{param}", generateHandler())
// 	tree.Add("GET", "/{param:[a-zA-Z]{10}}/paco/", generateHandler())
// 	tree.Add("GET", "/juan/hello{param:[a-zA-Z]{10}}/paco/", generateHandler())
// 	tree.Add("GET", "/juan/hello{param:[a-zA-Z]{10}}jesus/paco/", generateHandler())

// 	tree.Get("GET", "/data/", nil)

// 	println("Hola")
// }
