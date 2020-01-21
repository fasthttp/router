// Copyright 2013 Julien Schmidt. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.

// Package router is a trie based high performance HTTP request router.
//
// A trivial example is:
//
//  package main
//
//  import (
//      "fmt"
//      "github.com/julienschmidt/httprouter"
//      "net/http"
//      "log"
//  )
//
//  func Index(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
//      fmt.Fprint(w, "Welcome!\n")
//  }
//
//  func Hello(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
//      fmt.Fprintf(w, "hello, %s!\n", ps.ByName("name"))
//  }
//
//  func main() {
//      router := httprouter.New()
//      router.GET("/", Index)
//      router.GET("/hello/:name", Hello)
//
//      log.Fatal(http.ListenAndServe(":8080", router))
//  }
//
// The router matches incoming requests by the request method and the path.
// If a handle is registered for this path and method, the router delegates the
// request to that function.
// For the methods GET, POST, PUT, PATCH, DELETE and OPTIONS shortcut functions exist to
// register handles, for all other methods router.Handle can be used.
//
// The registered path, against which the router matches incoming requests, can
// contain two types of parameters:
//  Syntax    Type
//  :name     named parameter
//  *name     catch-all parameter
//
// Named parameters are dynamic path segments. They match anything until the
// next '/' or the path end:
//  Path: /blog/:category/:post
//
//  Requests:
//   /blog/go/request-routers            match: category="go", post="request-routers"
//   /blog/go/request-routers/           no match, but the router would redirect
//   /blog/go/                           no match
//   /blog/go/request-routers/comments   no match
//
// Catch-all parameters match anything until the path end, including the
// directory index (the '/' before the catch-all). Since they match anything
// until the end, catch-all parameters must always be the final path element.
//  Path: /files/*filepath
//
//  Requests:
//   /files/                             match: filepath="/"
//   /files/LICENSE                      match: filepath="/LICENSE"
//   /files/templates/article.html       match: filepath="/templates/article.html"
//   /files                              no match, but the router would redirect
//
// The value of parameters is saved as a slice of the Param struct, consisting
// each of a key and a value. The slice is passed to the Handle func as a third
// parameter.
// There are two ways to retrieve the value of a parameter:
//  // by the name of the parameter
//  user := ps.ByName("user") // defined by :user or *user
//
//  // by the index of the parameter. This way you can also get the name (key)
//  thirdKey   := ps[2].Key   // the name of the 3rd parameter
//  thirdValue := ps[2].Value // the value of the 3rd parameter
package router

import (
	"strings"

	"github.com/savsgio/gotils"
	"github.com/valyala/bytebufferpool"
	"github.com/valyala/fasthttp"
)

var (
	defaultContentType = []byte("text/plain; charset=utf-8")
	questionMark       = []byte("?")
)

// Handle is a function that can be registered to a route to handle HTTP
// requests. Like http.HandlerFunc, but has a third parameter for the values of
// wildcards (path variables).
type Handle func(*fasthttp.RequestCtx)

// MatchedRoutePathParam is the Param name under which the path of the matched
// route is stored, if Router.SaveMatchedRoutePath is set.
var MatchedRoutePathParam = "$matchedRoutePath"

// Router is a http.Handler which can be used to dispatch requests to different
// handler functions via configurable routes
type Router struct {
	parent          *Router
	beginPath       string
	registeredPaths map[string][]string

	trees map[string]*node

	// If enabled, adds the matched route path onto the http.Request context
	// before invoking the handler.
	// The matched route path is only added to handlers of routes that were
	// registered when this option was enabled.
	SaveMatchedRoutePath bool

	// Enables automatic redirection if the current route can't be matched but a
	// handler for the path with (without) the trailing slash exists.
	// For example if /foo/ is requested but a route only exists for /foo, the
	// client is redirected to /foo with http status code 301 for GET requests
	// and 308 for all other request methods.
	RedirectTrailingSlash bool

	// If enabled, the router tries to fix the current request path, if no
	// handle is registered for it.
	// First superfluous path elements like ../ or // are removed.
	// Afterwards the router does a case-insensitive lookup of the cleaned path.
	// If a handle can be found for this route, the router makes a redirection
	// to the corrected path with status code 301 for GET requests and 308 for
	// all other request methods.
	// For example /FOO and /..//Foo could be redirected to /foo.
	// RedirectTrailingSlash is independent of this option.
	RedirectFixedPath bool

	// If enabled, the router checks if another method is allowed for the
	// current route, if the current request can not be routed.
	// If this is the case, the request is answered with 'Method Not Allowed'
	// and HTTP status code 405.
	// If no other Method is allowed, the request is delegated to the NotFound
	// handler.
	HandleMethodNotAllowed bool

	// If enabled, the router automatically replies to OPTIONS requests.
	// Custom OPTIONS handlers take priority over automatic replies.
	HandleOPTIONS bool

	// An optional http.Handler that is called on automatic OPTIONS requests.
	// The handler is only called if HandleOPTIONS is true and no OPTIONS
	// handler for the specific path was set.
	// The "Allowed" header is set before calling the handler.
	GlobalOPTIONS fasthttp.RequestHandler

	// Cached value of global (*) allowed methods
	globalAllowed string

	// Configurable http.Handler which is called when no matching route is
	// found. If it is not set, http.NotFound is used.
	NotFound fasthttp.RequestHandler

	// Configurable http.Handler which is called when a request
	// cannot be routed and HandleMethodNotAllowed is true.
	// If it is not set, http.Error with http.StatusMethodNotAllowed is used.
	// The "Allow" header with allowed request methods is set before the handler
	// is called.
	MethodNotAllowed fasthttp.RequestHandler

	// Function to handle panics recovered from http handlers.
	// It should be used to generate a error page and return the http error code
	// 500 (Internal Server Error).
	// The handler can be used to keep your server from crashing because of
	// unrecovered panics.
	PanicHandler func(*fasthttp.RequestCtx, interface{})
}

// Make sure the Router conforms with the http.Handler interface
// var _ http.Handler = New()

// New returns a new initialized Router.
// Path auto-correction, including trailing slashes, is enabled by default.
func New() *Router {
	return &Router{
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

func (r *Router) saveMatchedRoutePath(path string, handle Handle) Handle {
	return func(ctx *fasthttp.RequestCtx) {
		ctx.SetUserValue(MatchedRoutePathParam, path)
		handle(ctx)

	}
}

// GET is a shortcut for router.Handle(http.MethodGet, path, handle)
func (r *Router) GET(path string, handle Handle) {
	r.Handle(fasthttp.MethodGet, path, handle)
}

// HEAD is a shortcut for router.Handle(http.MethodHead, path, handle)
func (r *Router) HEAD(path string, handle Handle) {
	r.Handle(fasthttp.MethodHead, path, handle)
}

// OPTIONS is a shortcut for router.Handle(http.MethodOptions, path, handle)
func (r *Router) OPTIONS(path string, handle Handle) {
	r.Handle(fasthttp.MethodOptions, path, handle)
}

// POST is a shortcut for router.Handle(http.MethodPost, path, handle)
func (r *Router) POST(path string, handle Handle) {
	r.Handle(fasthttp.MethodPost, path, handle)
}

// PUT is a shortcut for router.Handle(http.MethodPut, path, handle)
func (r *Router) PUT(path string, handle Handle) {
	r.Handle(fasthttp.MethodPut, path, handle)
}

// PATCH is a shortcut for router.Handle(http.MethodPatch, path, handle)
func (r *Router) PATCH(path string, handle Handle) {
	r.Handle(fasthttp.MethodPatch, path, handle)
}

// DELETE is a shortcut for router.Handle(http.MethodDelete, path, handle)
func (r *Router) DELETE(path string, handle Handle) {
	r.Handle(fasthttp.MethodDelete, path, handle)
}

// Handle registers a new request handle with the given path and method.
//
// For GET, POST, PUT, PATCH and DELETE requests the respective shortcut
// functions can be used.
//
// This function is intended for bulk loading and to allow the usage of less
// frequently used, non-standardized or custom methods (e.g. for internal
// communication with a proxy).
func (r *Router) Handle(method, path string, handle Handle) {
	varsCount := uint16(0)

	if method == "" {
		panic("method must not be empty")
	}
	if len(path) < 1 || path[0] != '/' {
		panic("path must begin with '/' in path '" + path + "'")
	}
	if handle == nil {
		panic("handle must not be nil")
	}

	if r.beginPath != "/" {
		path = r.beginPath + path
	}

	r.registeredPaths[method] = append(r.registeredPaths[method], path)

	// Call to the parent recursively until main router to register paths in it
	if r.parent != nil {
		r.parent.Handle(method, path, handle)
		return
	}

	if r.SaveMatchedRoutePath {
		varsCount++
		handle = r.saveMatchedRoutePath(path, handle)
	}

	if r.trees == nil {
		r.trees = make(map[string]*node)
	}

	root := r.trees[method]
	if root == nil {
		root = new(node)
		r.trees[method] = root

		r.globalAllowed = r.allowed("*", "")
	}

	optionalPaths := getOptionalPaths(path)

	// if not has optional paths, adds the original
	if len(optionalPaths) == 0 {
		root.addRoute(path, handle)
	} else {
		for _, p := range optionalPaths {
			root.addRoute(p, handle)
		}
	}
}

// ServeFiles serves files from the given file system root.
// The path must end with "/*filepath", files are then served from the local
// path /defined/root/dir/*filepath.
// For example if root is "/etc" and *filepath is "passwd", the local file
// "/etc/passwd" would be served.
// Internally a http.FileServer is used, therefore http.NotFound is used instead
// of the Router's NotFound handler.
// To use the operating system's file system implementation,
// use http.Dir:
//     router.ServeFiles("/src/*filepath", http.Dir("/var/www"))
func (r *Router) ServeFiles(path string, rootPath string) {
	if len(path) < 10 || path[len(path)-10:] != "/*filepath" {
		panic("path must end with /*filepath in path '" + path + "'")
	}

	if r.beginPath != "/" {
		path = r.beginPath + path
	}

	if r.parent != nil {
		r.parent.ServeFiles(path, rootPath)
		return
	}

	prefix := path[:len(path)-10]
	fileHandler := fasthttp.FSHandler(rootPath, strings.Count(prefix, "/"))

	r.GET(path, func(ctx *fasthttp.RequestCtx) {
		fileHandler(ctx)
	})
}

// ServeFilesCustom serves files from the given file system settings.
// The path must end with "/*filepath", files are then served from the local
// path /defined/root/dir/*filepath.
// For example if root is "/etc" and *filepath is "passwd", the local file
// "/etc/passwd" would be served.
// Internally a http.FileServer is used, therefore http.NotFound is used instead
// of the Router's NotFound handler.
//     router.ServeFilesCustom("/src/*filepath", *customFS)
func (r *Router) ServeFilesCustom(path string, fs *fasthttp.FS) {
	if len(path) < 10 || path[len(path)-10:] != "/*filepath" {
		panic("path must end with /*filepath in path '" + path + "'")
	}

	if r.beginPath != "/" {
		path = r.beginPath + path
	}

	if r.parent != nil {
		r.parent.ServeFilesCustom(path, fs)
		return
	}

	prefix := path[:len(path)-10]
	stripSlashes := strings.Count(prefix, "/")

	if fs.PathRewrite == nil && stripSlashes > 0 {
		fs.PathRewrite = fasthttp.NewPathSlashesStripper(stripSlashes)
	}
	fileHandler := fs.NewRequestHandler()

	r.GET(path, func(ctx *fasthttp.RequestCtx) {
		fileHandler(ctx)
	})
}

func (r *Router) recv(ctx *fasthttp.RequestCtx) {
	if rcv := recover(); rcv != nil {
		r.PanicHandler(ctx, rcv)
	}
}

// Lookup allows the manual lookup of a method + path combo.
// This is e.g. useful to build a framework around this router.
// If the path was found, it returns the handle function and the path parameter
// values. Otherwise the third return value indicates whether a redirection to
// the same path with an extra / without the trailing slash should be performed.
func (r *Router) Lookup(method, path string, ctx *fasthttp.RequestCtx) (Handle, bool) {
	if root := r.trees[method]; root != nil {
		handle, tsr := root.getValue(path, ctx)
		if handle == nil {
			return nil, tsr
		}

		return handle, tsr
	}
	return nil, false
}

func (r *Router) allowed(path, reqMethod string) (allow string) {
	allowed := make([]string, 0, 9)

	if path == "*" || path == "/*" { // server-wide{ // server-wide
		// empty method is used for internal calls to refresh the cache
		if reqMethod == "" {
			for method := range r.trees {
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
		for method := range r.trees {
			// Skip the requested method - we already tried this one
			if method == reqMethod || method == fasthttp.MethodOptions {
				continue
			}

			handle, _ := r.trees[method].getValue(path, nil)
			if handle != nil {
				// Add request method to list of allowed methods
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

	if root := r.trees[method]; root != nil {
		if handle, tsr := root.getValue(path, ctx); handle != nil {
			handle(ctx)
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
					uri.Write(questionMark)
					uri.Write(queryBuf)
				}

				ctx.Redirect(uri.String(), code)

				bytebufferpool.Put(uri)
				return
			}

			// Try to fix the request path
			if r.RedirectFixedPath {
				fixedPath, found := root.findCaseInsensitivePath(
					CleanPath(path),
					r.RedirectTrailingSlash,
				)
				if found {
					queryBuf := ctx.URI().QueryString()
					if len(queryBuf) > 0 {
						fixedPath = append(fixedPath, questionMark...)
						fixedPath = append(fixedPath, queryBuf...)
					}

					ctx.RedirectBytes(fixedPath, code)
					return
				}
			}
		}
	}

	if method == fasthttp.MethodOptions && r.HandleOPTIONS {
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
