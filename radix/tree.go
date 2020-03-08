package radix

import (
	"strings"

	"github.com/valyala/fasthttp"
)

type Tree struct {
	root *node
}

const stackBufSize = 128

func New() *Tree {
	return &Tree{
		root: &node{
			path:  "/",
			nType: root,
		},
	}
}

// Add adds a node with the given handle to the path.
//
// WARNING: Not concurrency-safe!
func (t *Tree) Add(path string, handler fasthttp.RequestHandler) {
	if !strings.HasPrefix(path, "/") {
		panic("path must begin with '/' in path '" + path + "'")
	}

	if path == "/" {
		if t.root.handler != nil {
			panic("Duplicated path: " + path)
		}

		t.root.handler = handler

		return
	}

	n := t.root
	fullPath := path

	// Checks if the path has a trailing slash
	tsr := false
	if strings.HasSuffix(path, "/") {
		tsr = true
		path = path[:len(path)-1]
	}

	// Remove the initial '/'
	path = path[1:]

	for {
		// Recursive addition inside the nodes
		n, path = n.add(path, fullPath, handler)

		// N
		if len(path) == 0 {
			n.tsr = tsr
			break
		}
	}

	// Reorder the nodes
	t.root.sort()
}

// Get returns the handle registered with the given path (key). The values of
// param/wildcard are saved as ctx.UserValue.
// If no handle can be found, a TSR (trailing slash redirect) recommendation is
// made if a handle exists with an extra (without the) trailing slash for the
// given path.
func (t *Tree) Get(path string, ctx *fasthttp.RequestCtx) (fasthttp.RequestHandler, bool) {
	n := t.root

	if path == "/" {
		if n.handler != nil {
			// Found it
			return n.handler, false

		} else if n.wildcard != nil {
			// Not found, but the node has a wildcard

			if ctx != nil {
				// Save the wildcard value
				ctx.SetUserValue(n.wildcard.path[1:], path)
			}

			return n.wildcard.handler, false
		}

		// Not found
		return nil, false
	}

	// Remove the initial '/'
	path = path[1:]

	for {
		// Recursive search inside the children nodes
		n, path = n.get(path, ctx)

		switch {
		case n == nil:
			// Not found
			return nil, false

		case n.nType == wildcard:
			// Not found, but the node has a wildcard

			if ctx != nil {
				// Save the wildcard value
				ctx.SetUserValue(n.path[1:], path)
			}

			return n.handler, false

		case len(path) == 0:
			// Search is finished

			if n.tsr {
				// The handler is found, but the route is registered with a trailing slash.
				// So tries to force the redirect
				return nil, true

			} else if n.handler != nil {
				// Found it
				return n.handler, false

			} else if n.wildcard != nil {
				// Not found, but the node has a wildcard

				if ctx != nil {
					// Save the wildcard value
					ctx.SetUserValue(n.wildcard.path[1:], "/")
				}

				return n.wildcard.handler, false
			}

			return nil, false
		case path == "/" && n.handler != nil:
			// Search is finished but the requested path has a trainling slash

			if n.tsr {
				// Found it, because the path is registered with a trailing slash
				return n.handler, false
			}

			return nil, true
		}
	}
}

// FindCaseInsensitivePath makes a case-insensitive lookup of the given path
// and tries to find a handler.
// It can optionally also fix trailing slashes.
// It returns the case-corrected path and a bool indicating whether the lookup
// was successful.
func (t *Tree) FindCaseInsensitivePath(path string, fixTrailingSlash bool) ([]byte, bool) {
	n := t.root

	// Remove the initial '/'
	path = path[1:]

	// Use a static sized buffer on the stack in the common case.
	// If the path is too long, allocate a buffer on the heap instead.
	buf := make([]byte, 0, stackBufSize)
	if l := len(path) + 1; l > stackBufSize {
		buf = make([]byte, 0, l)
	}

	buf = append(buf, '/')

	for {
		// Recursive case insensitive search inside the children nodes
		n, path, buf = n.getInsensitive(path, buf)

		switch {
		case n == nil:
			// Not found
			return nil, false

		case n.nType == wildcard:
			// Not found an static/param path but has a wildcard
			buf = append(buf, path...)

			return buf, true

		case len(path) == 0:
			// Search is finished

			if n.handler == nil {
				// Not found
				return nil, false

			} else if n.tsr {
				// Check if the route is registered with a trailing slash

				if !fixTrailingSlash {
					// Force the non redirect to the requested path with a trailing slash
					return nil, false
				}

				buf = append(buf, '/')
			}

			return buf, true

		case path == "/" && n.handler != nil:
			// Search is finished but the requested path has a trainling slash

			if n.tsr {
				// Adds the traling slash because the route is registed with a trailing slash
				buf = append(buf, '/')

			} else if !fixTrailingSlash {
				// Force the non redirect to the requested path with a trailing slash
				return nil, false
			}

			return buf, true
		}
	}
}
