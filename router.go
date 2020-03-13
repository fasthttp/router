package router

import (
	"sync"

	"github.com/valyala/fasthttp"
)

type Router struct {
	*BaseRouter

	lock    *sync.Mutex
	befores []Middleware
	afters  []Middleware

	children []*Router
}

func New() *Router {
	return &Router{
		BaseRouter: new_base_router(),
		lock:       &sync.Mutex{},
		befores:    make([]Middleware, 0),
		afters:     make([]Middleware, 0),
		children:   make([]*Router, 0),
	}
}

func (mr *Router) Group(path string) *Router {
	child := &Router{
		BaseRouter: mr.BaseRouter.Group(path),
		lock:       &sync.Mutex{},
		befores:    make([]Middleware, 0),
		afters:     make([]Middleware, 0),
		children:   make([]*Router, 0),
	}

	mr.children = append(mr.children, child)
	return child
}

// Lock protect middleware slices.
func (mr *Router) Lock() {
	mr.lock.Lock()
	for _, child := range mr.children {
		child.Lock()
	}
}

func (mr *Router) Before(mw Middleware) {
	mr.lock.Lock()
	defer mr.lock.Unlock()
	mr.befores = append(mr.befores, mw)
}

func (mr *Router) After(mw Middleware) {
	mr.lock.Lock()
	defer mr.lock.Unlock()
	mr.afters = append(mr.afters, mw)
}

func (mr *Router) Handle(method string, path string, handler fasthttp.RequestHandler) {
	bl := len(mr.befores)
	al := len(mr.afters)

	new_handler := handler
	if bl == 0 && al != 0 {
		new_handler = func(ctx *fasthttp.RequestCtx) {
			handler(ctx)
			for _, mw := range mr.afters {
				mw.Handle(ctx)
			}
		}
	} else if bl != 0 && al == 0 {
		new_handler = func(ctx *fasthttp.RequestCtx) {
			for _, mw := range mr.befores {
				mw.Handle(ctx)
			}
			handler(ctx)
		}
	} else if bl != 0 && al != 0 {
		new_handler = func(ctx *fasthttp.RequestCtx) {
			for _, mw := range mr.befores {
				mw.Handle(ctx)
			}
			handler(ctx)
			for _, mw := range mr.afters {
				mw.Handle(ctx)
			}
		}
	}

	mr.BaseRouter.Handle(method, path, new_handler)
}

// GET is a shortcut for router.Handle(fasthttp.MethodGet, path, handle)
func (mr *Router) GET(path string, handle fasthttp.RequestHandler) {
	mr.Handle(fasthttp.MethodGet, path, handle)
}

// HEAD is a shortcut for router.Handle(fasthttp.MethodHead, path, handle)
func (mr *Router) HEAD(path string, handle fasthttp.RequestHandler) {
	mr.Handle(fasthttp.MethodHead, path, handle)
}

// OPTIONS is a shortcut for router.Handle(fasthttp.MethodOptions, path, handle)
func (mr *Router) OPTIONS(path string, handle fasthttp.RequestHandler) {
	mr.Handle(fasthttp.MethodOptions, path, handle)
}

// POST is a shortcut for router.Handle(fasthttp.MethodPost, path, handle)
func (mr *Router) POST(path string, handle fasthttp.RequestHandler) {
	mr.Handle(fasthttp.MethodPost, path, handle)
}

// PUT is a shortcut for router.Handle(fasthttp.MethodPut, path, handle)
func (mr *Router) PUT(path string, handle fasthttp.RequestHandler) {
	mr.Handle(fasthttp.MethodPut, path, handle)
}

// PATCH is a shortcut for router.Handle(fasthttp.MethodPatch, path, handle)
func (mr *Router) PATCH(path string, handle fasthttp.RequestHandler) {
	mr.Handle(fasthttp.MethodPatch, path, handle)
}

// DELETE is a shortcut for router.Handle(fasthttp.MethodDelete, path, handle)
func (mr *Router) DELETE(path string, handle fasthttp.RequestHandler) {
	mr.Handle(fasthttp.MethodDelete, path, handle)
}
