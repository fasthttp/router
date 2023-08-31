package router

import (
	"github.com/valyala/fasthttp"
)

// Group returns a new group.
// Path auto-correction, including trailing slashes, is enabled by default.
func (g *Group) Group(path string) *Group {
	validatePath(path)

	if len(g.prefix) > 0 && path == "/" {
		return g
	}

	return g.router.Group(g.prefix + path)
}

// GET is a shortcut for group.Handle(fasthttp.MethodGet, path, handler)
func (g *Group) GET(path string, handler fasthttp.RequestHandler) {
	validatePath(path)
	handler = g.applyMiddleware(handler)
	g.router.GET(g.prefix+path, handler)
}

// HEAD is a shortcut for group.Handle(fasthttp.MethodHead, path, handler)
func (g *Group) HEAD(path string, handler fasthttp.RequestHandler) {
	validatePath(path)
	handler = g.applyMiddleware(handler)
	g.router.HEAD(g.prefix+path, handler)
}

// POST is a shortcut for group.Handle(fasthttp.MethodPost, path, handler)
func (g *Group) POST(path string, handler fasthttp.RequestHandler) {
	validatePath(path)
	handler = g.applyMiddleware(handler)
	g.router.POST(g.prefix+path, handler)
}

// PUT is a shortcut for group.Handle(fasthttp.MethodPut, path, handler)
func (g *Group) PUT(path string, handler fasthttp.RequestHandler) {
	validatePath(path)
	handler = g.applyMiddleware(handler)
	g.router.PUT(g.prefix+path, handler)
}

// PATCH is a shortcut for group.Handle(fasthttp.MethodPatch, path, handler)
func (g *Group) PATCH(path string, handler fasthttp.RequestHandler) {
	validatePath(path)
	handler = g.applyMiddleware(handler)
	g.router.PATCH(g.prefix+path, handler)
}

// DELETE is a shortcut for group.Handle(fasthttp.MethodDelete, path, handler)
func (g *Group) DELETE(path string, handler fasthttp.RequestHandler) {
	validatePath(path)
	handler = g.applyMiddleware(handler)
	g.router.DELETE(g.prefix+path, handler)
}

// CONNECT is a shortcut for group.Handle(fasthttp.MethodConnect, path, handler)
func (g *Group) CONNECT(path string, handler fasthttp.RequestHandler) {
	validatePath(path)
	handler = g.applyMiddleware(handler)
	g.router.CONNECT(g.prefix+path, handler)
}

// OPTIONS is a shortcut for group.Handle(fasthttp.MethodOptions, path, handler)
func (g *Group) OPTIONS(path string, handler fasthttp.RequestHandler) {
	validatePath(path)
	handler = g.applyMiddleware(handler)
	g.router.OPTIONS(g.prefix+path, handler)
}

// TRACE is a shortcut for group.Handle(fasthttp.MethodTrace, path, handler)
func (g *Group) TRACE(path string, handler fasthttp.RequestHandler) {
	validatePath(path)
	handler = g.applyMiddleware(handler)
	g.router.TRACE(g.prefix+path, handler)
}

// ANY is a shortcut for group.Handle(router.MethodWild, path, handler)
//
// WARNING: Use only for routes where the request method is not important
func (g *Group) ANY(path string, handler fasthttp.RequestHandler) {
	validatePath(path)
	handler = g.applyMiddleware(handler)
	g.router.ANY(g.prefix+path, handler)
}

// ServeFiles serves files from the given file system root.
// The path must end with "/{filepath:*}", files are then served from the local
// path /defined/root/dir/{filepath:*}.
// For example if root is "/etc" and {filepath:*} is "passwd", the local file
// "/etc/passwd" would be served.
// Internally a fasthttp.FSHandler is used, therefore http.NotFound is used instead
// Use:
//
//	router.ServeFiles("/src/{filepath:*}", "./")
func (g *Group) ServeFiles(path string, rootPath string) {
	validatePath(path)

	g.router.ServeFiles(g.prefix+path, rootPath)
}

// ServeFilesCustom serves files from the given file system settings.
// The path must end with "/{filepath:*}", files are then served from the local
// path /defined/root/dir/{filepath:*}.
// For example if root is "/etc" and {filepath:*} is "passwd", the local file
// "/etc/passwd" would be served.
// Internally a fasthttp.FSHandler is used, therefore http.NotFound is used instead
// of the Router's NotFound handler.
// Use:
//
//	router.ServeFilesCustom("/src/{filepath:*}", *customFS)
func (g *Group) ServeFilesCustom(path string, fs *fasthttp.FS) {
	validatePath(path)

	g.router.ServeFilesCustom(g.prefix+path, fs)
}

// Handle registers a new request handler with the given path and method.
//
// For GET, POST, PUT, PATCH and DELETE requests the respective shortcut
// functions can be used.
//
// This function is intended for bulk loading and to allow the usage of less
// frequently used, non-standardized or custom methods (e.g. for internal
// communication with a proxy).
func (g *Group) Handle(method, path string, handler fasthttp.RequestHandler) {
	validatePath(path)
	handler = g.applyMiddleware(handler)
	g.router.Handle(method, g.prefix+path, handler)
}

func (g *Group) AddMiddleware(h func(fasthttp.RequestHandler) fasthttp.RequestHandler) {
	g.middleware = append(g.middleware, h)
}

func (g *Group) applyMiddleware(handler fasthttp.RequestHandler) fasthttp.RequestHandler {
	if len(g.middleware) == 0 {
		return handler
	}

	for i := len(g.middleware) - 1; i >= 0; i-- {
		handler = g.middleware[i](handler)
	}

	return handler
}
