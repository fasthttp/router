package radix

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/savsgio/gotils"
	"github.com/valyala/fasthttp"
)

func generateHandler() fasthttp.RequestHandler {
	hex := gotils.RandBytes(make([]byte, 10))

	return func(ctx *fasthttp.RequestCtx) {
		ctx.Write(hex)
	}
}

func testHandlerAndParams(
	t *testing.T, tree *Tree, requestedPath string, handler fasthttp.RequestHandler, wantTSR bool, params map[string]interface{},
) {
	ctx := new(fasthttp.RequestCtx)

	h, tsr := tree.Get(requestedPath, ctx)
	if reflect.ValueOf(handler).Pointer() != reflect.ValueOf(h).Pointer() {
		t.Errorf("Path '%s' handler == %p, want %p", requestedPath, h, handler)
	}

	if wantTSR != tsr {
		t.Errorf("Path '%s' tsr == %v, want %v", requestedPath, tsr, wantTSR)
	}

	resultParams := make(map[string]interface{})
	if params == nil {
		params = make(map[string]interface{})
	}

	ctx.VisitUserValues(func(key []byte, value interface{}) {
		resultParams[string(key)] = value
	})

	if !reflect.DeepEqual(resultParams, params) {
		t.Errorf("User values == %v, want %v", resultParams, params)
	}
}

func Test_Tree(t *testing.T) {
	type args struct {
		path          string
		requestedPath string
		handler       fasthttp.RequestHandler
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
				path:          "/users/:name",
				requestedPath: "/users/atreugo",
				handler:       generateHandler(),
			},
			want: want{
				params: map[string]interface{}{
					"name": "atreugo",
				},
			},
		},
		{
			args: args{
				path:          "/users",
				requestedPath: "/users",
				handler:       generateHandler(),
			},
			want: want{
				params: nil,
			},
		},
		{
			args: args{
				path:          "/",
				requestedPath: "/",
				handler:       generateHandler(),
			},
			want: want{
				params: nil,
			},
		},
		{
			args: args{
				path:          "/users/:name/jobs",
				requestedPath: "/users/atreugo/jobs",
				handler:       generateHandler(),
			},
			want: want{
				params: map[string]interface{}{
					"name": "atreugo",
				},
			},
		},
		{
			args: args{
				path:          "/users/admin",
				requestedPath: "/users/admin",
				handler:       generateHandler(),
			},
			want: want{
				params: nil,
			},
		},
		{
			args: args{
				path:          "/users/:status/proc",
				requestedPath: "/users/active/proc",
				handler:       generateHandler(),
			},
			want: want{
				params: map[string]interface{}{
					"status": "active",
				},
			},
		},
		{
			args: args{
				path:          "/static/*filepath",
				requestedPath: "/static/assets/js/main.js",
				handler:       generateHandler(),
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
		tree.Add(test.args.path, test.args.handler)
	}

	emptyParams := make(map[string]interface{})
	for _, test := range tests {
		if test.want.params == nil {
			test.want.params = emptyParams
		}

		testHandlerAndParams(t, tree, test.args.requestedPath, test.args.handler, false, test.want.params)
	}

	testHandlerAndParams(t, tree, "/fuck/notfound", nil, false, emptyParams)

	filepathHandler := generateHandler()
	tree.Add("/*filepath", filepathHandler)

	testHandlerAndParams(t, tree, "/js/main.js", filepathHandler, false, map[string]interface{}{
		"filepath": "js/main.js",
	})
}

func Test_Get(t *testing.T) {
	handler := generateHandler()

	tree := New()
	tree.Add("/api", handler)

	testHandlerAndParams(t, tree, "/api/", nil, true, nil)

	tree = New()
	tree.Add("/api/", handler)

	testHandlerAndParams(t, tree, "/api", nil, true, nil)
	testHandlerAndParams(t, tree, "/api/", handler, false, nil)
	testHandlerAndParams(t, tree, "/data", nil, false, nil)
}

func Test_TreeRootWildcard(t *testing.T) {
	tree := New()

	handler := generateHandler()
	tree.Add("/*filepath", handler)

	testHandlerAndParams(t, tree, "/", handler, false, map[string]interface{}{
		"filepath": "/",
	})
}

func Benchmark_Get(b *testing.B) {
	tree := New()

	for i := 0; i < 3000; i++ {
		tree.Add(
			fmt.Sprintf("/%s", gotils.RandBytes(make([]byte, 15))), generateHandler(),
		)
	}

	tree.Add("/plaintext", generateHandler())
	tree.Add("/json", generateHandler())
	tree.Add("/fortune", generateHandler())
	tree.Add("/fortune-quick", generateHandler())
	tree.Add("/db", generateHandler())
	tree.Add("/queries", generateHandler())
	tree.Add("/update", generateHandler())

	ctx := new(fasthttp.RequestCtx)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		tree.Get("/fortune-quick", ctx)
	}
}

func Benchmark_FindCaseInsensitivePath(b *testing.B) {
	tree := New()
	tree.Add("/endpoint", generateHandler())

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		tree.FindCaseInsensitivePath("/ENdpOiNT", false)
	}
}
