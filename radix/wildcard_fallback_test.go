package radix

import (
	"testing"

	"github.com/valyala/fasthttp"
)

func TestWildcardParentMatchesAfterFailingStatic(t *testing.T) {
	tree := New()
	tree.Add("/{path:*}", func(ctx *fasthttp.RequestCtx) {
		ctx.WriteString("hello")
	})
	tree.Add("/test/{path:*}", func(ctx *fasthttp.RequestCtx) {
		ctx.WriteString("world")
	})

	tests := []struct {
		path     string
		wantBody string
	}{
		{"/tes", "hello"},
		{"/test/foo", "world"},
		{"/test", "hello"},
		{"/", "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			rc := &fasthttp.RequestCtx{}
			h, tsr := tree.Get(tt.path, rc)
			if h == nil {
				t.Fatalf("expected handler for %s, got nil (tsr=%v)", tt.path, tsr)
			}
			hctx := &fasthttp.RequestCtx{}
			h(hctx)
			got := string(hctx.Response.Body())
			if got != tt.wantBody {
				t.Errorf("%s => %q, want %q", tt.path, got, tt.wantBody)
			}
		})
	}
}
