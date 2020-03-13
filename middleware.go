package router

import "github.com/valyala/fasthttp"

type Middleware interface {
	Handle(*fasthttp.RequestCtx)
}

type MiddlewareFunc func(*fasthttp.RequestCtx)

func (fn MiddlewareFunc) Handle(ctx *fasthttp.RequestCtx) {
	fn(ctx)
}
