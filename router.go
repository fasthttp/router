package router

import (
	"fmt"
	"strings"

	"github.com/fasthttp/router/radix"
	"github.com/savsgio/gotils"
	"github.com/valyala/bytebufferpool"
	"github.com/valyala/fasthttp"
)

// MethodWild wild HTTP method
const MethodWild = radix.MethodWild

var (
	defaultContentType = []byte("text/plain; charset=utf-8")
	questionMark       = byte('?')

	// MatchedRoutePathParam is the param name under which the path of the matched
	// route is stored, if Router.SaveMatchedRoutePath is set.
	MatchedRoutePathParam = fmt.Sprintf("__matchedRoutePath::%s__", gotils.RandBytes(make([]byte, 15)))
)

// New returns a new initialized Router.
// Path auto-correction, including trailing slashes, is enabled by default.
func New() *Router {
	return &Router{
		tree:                   radix.New(),
		beginPath:              "/",
		registeredPaths:        make(map[string][]string),
		RedirectTrailingSlash:  true,
		RedirectFixedPath:      true,
		HandleMethodNotAllowed: true,
		HandleOPTIONS:          true,
	}
}

// Group returns a new grouped Router.
// Path auto-correction, including trailing slashes, is enabled by default.
func (r *Router) Group(path string) *Router {
	g := New()
	g.parent = r
	g.beginPath = path

	return g
}

func (r *Router) saveMatchedRoutePath(path string, handler fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		ctx.SetUserValue(MatchedRoutePathParam, path)
		handler(ctx)
	}
}

// GET is a shortcut for router.Handle(fasthttp.MethodGet, path, handler)
func (r *Router) GET(path string, handler fasthttp.RequestHandler) {
	r.Handle(fasthttp.MethodGet, path, handler)
}

// HEAD is a shortcut for router.Handle(fasthttp.MethodHead, path, handler)
func (r *Router) HEAD(path string, handler fasthttp.RequestHandler) {
	r.Handle(fasthttp.MethodHead, path, handler)
}

// OPTIONS is a shortcut for router.Handle(fasthttp.MethodOptions, path, handler)
func (r *Router) OPTIONS(path string, handler fasthttp.RequestHandler) {
	r.Handle(fasthttp.MethodOptions, path, handler)
}

// POST is a shortcut for router.Handle(fasthttp.MethodPost, path, handler)
func (r *Router) POST(path string, handler fasthttp.RequestHandler) {
	r.Handle(fasthttp.MethodPost, path, handler)
}

// PUT is a shortcut for router.Handle(fasthttp.MethodPut, path, handler)
func (r *Router) PUT(path string, handler fasthttp.RequestHandler) {
	r.Handle(fasthttp.MethodPut, path, handler)
}

// PATCH is a shortcut for router.Handle(fasthttp.MethodPatch, path, handler)
func (r *Router) PATCH(path string, handler fasthttp.RequestHandler) {
	r.Handle(fasthttp.MethodPatch, path, handler)
}

// DELETE is a shortcut for router.Handle(fasthttp.MethodDelete, path, handler)
func (r *Router) DELETE(path string, handler fasthttp.RequestHandler) {
	r.Handle(fasthttp.MethodDelete, path, handler)
}

// ANY is a shortcut for router.Handle(router.MethodWild, path, handler)
//
// WARNING: Use only for routes where the request method is not important
func (r *Router) ANY(path string, handler fasthttp.RequestHandler) {
	r.Handle(MethodWild, path, handler)
}

// Handle registers a new request handler with the given path and method.
//
// For GET, POST, PUT, PATCH and DELETE requests the respective shortcut
// functions can be used.
//
// This function is intended for bulk loading and to allow the usage of less
// frequently used, non-standardized or custom methods (e.g. for internal
// communication with a proxy).
func (r *Router) Handle(method, path string, handler fasthttp.RequestHandler) {
	switch {
	case len(method) == 0:
		panic("method must not be empty")
	case len(path) < 1 || path[0] != '/':
		panic("path must begin with '/' in path '" + path + "'")
	case handler == nil:
		panic("handler must not be nil")
	}

	if r.beginPath != "/" {
		path = r.beginPath + path
	}

	newMethod := r.registeredPaths[method] == nil
	r.registeredPaths[method] = append(r.registeredPaths[method], path)

	// Call to the parent recursively until main router to register paths in it
	if r.parent != nil {
		r.parent.Handle(method, path, handler)
		return
	}

	if r.SaveMatchedRoutePath {
		handler = r.saveMatchedRoutePath(path, handler)
	}

	if newMethod {
		r.globalAllowed = r.allowed("*", "")
	}

	optionalPaths := getOptionalPaths(path)

	// if not has optional paths, adds the original
	if len(optionalPaths) == 0 {
		r.tree.Add(method, path, handler)
	} else {
		for _, p := range optionalPaths {
			r.tree.Add(method, p, handler)
		}
	}
}

// ServeFiles serves files from the given file system root.
// The path must end with "/{filepath:*}", files are then served from the local
// path /defined/root/dir/{filepath:*}.
// For example if root is "/etc" and {filepath:*} is "passwd", the local file
// "/etc/passwd" would be served.
// Internally a fasthttp.FSHandler is used, therefore http.NotFound is used instead
// Use:
//     router.ServeFiles("/src/{filepath:*}", "./")
func (r *Router) ServeFiles(path string, rootPath string) {
	suffix := "/{filepath:*}"

	if !strings.HasSuffix(path, suffix) {
		panic("path must end with " + suffix + " in path '" + path + "'")
	}

	if r.beginPath != "/" {
		path = r.beginPath + path
	}

	if r.parent != nil {
		r.parent.ServeFiles(path, rootPath)
		return
	}

	prefix := path[:len(path)-len(suffix)]
	fileHandler := fasthttp.FSHandler(rootPath, strings.Count(prefix, "/"))

	r.GET(path, fileHandler)
}

// ServeFilesCustom serves files from the given file system settings.
// The path must end with "/{filepath:*}", files are then served from the local
// path /defined/root/dir/{filepath:*}.
// For example if root is "/etc" and {filepath:*} is "passwd", the local file
// "/etc/passwd" would be served.
// Internally a fasthttp.FSHandler is used, therefore http.NotFound is used instead
// of the Router's NotFound handler.
// Use:
//     router.ServeFilesCustom("/src/{filepath:*}", *customFS)
func (r *Router) ServeFilesCustom(path string, fs *fasthttp.FS) {
	suffix := "/{filepath:*}"

	if !strings.HasSuffix(path, suffix) {
		panic("path must end with " + suffix + " in path '" + path + "'")
	}

	if r.beginPath != "/" {
		path = r.beginPath + path
	}

	if r.parent != nil {
		r.parent.ServeFilesCustom(path, fs)
		return
	}

	prefix := path[:len(path)-len(suffix)]
	stripSlashes := strings.Count(prefix, "/")

	if fs.PathRewrite == nil && stripSlashes > 0 {
		fs.PathRewrite = fasthttp.NewPathSlashesStripper(stripSlashes)
	}
	fileHandler := fs.NewRequestHandler()

	r.GET(path, fileHandler)
}

func (r *Router) recv(ctx *fasthttp.RequestCtx) {
	if rcv := recover(); rcv != nil {
		r.PanicHandler(ctx, rcv)
	}
}

// Lookup allows the manual lookup of a method + path combo.
// This is e.g. useful to build a framework around this router.
// If the path was found, it returns the handler function and the path parameter
// values. Otherwise the third return value indicates whether a redirection to
// the same path with an extra / without the trailing slash should be performed.
func (r *Router) Lookup(method, path string, ctx *fasthttp.RequestCtx) (fasthttp.RequestHandler, bool) {
	return r.tree.Get(method, path, ctx)
}

func (r *Router) allowed(path, reqMethod string) (allow string) {
	allowed := make([]string, 0, 9)

	if path == "*" || path == "/*" { // server-wide{ // server-wide
		// empty method is used for internal calls to refresh the cache
		if reqMethod == "" {
			for method := range r.registeredPaths {
				if method == fasthttp.MethodOptions {
					continue
				}
				// Add request method to list of allowed methods
				allowed = append(allowed, method)
			}
		} else {
			return r.globalAllowed
		}
	} else { // specific path
		for method, routes := range r.registeredPaths {
			// Skip the requested method - we already tried this one
			if method == reqMethod || method == fasthttp.MethodOptions {
				continue
			}

			if gotils.StringSliceInclude(routes, path) {
				allowed = append(allowed, method)
			}
		}
	}

	if len(allowed) > 0 {
		// Add request method to list of allowed methods
		allowed = append(allowed, fasthttp.MethodOptions)

		// Sort allowed methods.
		// sort.Strings(allowed) unfortunately causes unnecessary allocations
		// due to allowed being moved to the heap and interface conversion
		for i, l := 1, len(allowed); i < l; i++ {
			for j := i; j > 0 && allowed[j] < allowed[j-1]; j-- {
				allowed[j], allowed[j-1] = allowed[j-1], allowed[j]
			}
		}

		// return as comma separated list
		return strings.Join(allowed, ", ")
	}
	return
}

// Handler makes the router implement the http.Handler interface.
func (r *Router) Handler(ctx *fasthttp.RequestCtx) {
	if r.PanicHandler != nil {
		defer r.recv(ctx)
	}

	path := gotils.B2S(ctx.Path())
	method := gotils.B2S(ctx.Method())

	if handler, tsr := r.tree.Get(method, path, ctx); handler != nil {
		handler(ctx)
		return
	} else if method != fasthttp.MethodConnect && path != "/" {
		// Moved Permanently, request with GET method
		code := fasthttp.StatusMovedPermanently
		if method != fasthttp.MethodGet {
			// Permanent Redirect, request with same method
			code = fasthttp.StatusPermanentRedirect
		}

		if tsr && r.RedirectTrailingSlash {
			uri := bytebufferpool.Get()

			if len(path) > 1 && path[len(path)-1] == '/' {
				uri.SetString(path[:len(path)-1])
			} else {
				uri.SetString(path)
				uri.WriteString("/")
			}

			queryBuf := ctx.URI().QueryString()
			if len(queryBuf) > 0 {
				uri.WriteByte(questionMark)
				uri.Write(queryBuf)
			}

			ctx.Redirect(uri.String(), code)

			bytebufferpool.Put(uri)
			return
		}

		// Try to fix the request path
		if r.RedirectFixedPath {
			uri := bytebufferpool.Get()
			found := r.tree.FindCaseInsensitivePath(
				method,
				CleanPath(path),
				r.RedirectTrailingSlash,
				uri,
			)

			if found {
				queryBuf := ctx.URI().QueryString()
				if len(queryBuf) > 0 {
					uri.WriteByte(questionMark)
					uri.Write(queryBuf)
				}

				ctx.RedirectBytes(uri.Bytes(), code)

				bytebufferpool.Put(uri)

				return
			}
		}
	}

	if r.HandleOPTIONS && method == fasthttp.MethodOptions {
		// Handle OPTIONS requests
		if allow := r.allowed(path, fasthttp.MethodOptions); allow != "" {
			ctx.Response.Header.Set("Allow", allow)
			if r.GlobalOPTIONS != nil {
				r.GlobalOPTIONS(ctx)
			}
			return
		}
	} else if r.HandleMethodNotAllowed { // Handle 405
		if allow := r.allowed(path, method); allow != "" {
			ctx.Response.Header.Set("Allow", allow)
			if r.MethodNotAllowed != nil {
				r.MethodNotAllowed(ctx)
			} else {
				ctx.SetStatusCode(fasthttp.StatusMethodNotAllowed)
				ctx.SetBodyString(fasthttp.StatusMessage(fasthttp.StatusMethodNotAllowed))
			}
			return
		}
	}

	// Handle 404
	if r.NotFound != nil {
		r.NotFound(ctx)
	} else {
		ctx.Error(fasthttp.StatusMessage(fasthttp.StatusNotFound), fasthttp.StatusNotFound)
	}
}

// List returns all registered routes grouped by method
func (r *Router) List() map[string][]string {
	return r.registeredPaths
}
