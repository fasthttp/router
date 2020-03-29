package radix

import (
	"strings"

	"github.com/valyala/bytebufferpool"
	"github.com/valyala/fasthttp"
)

func New() *Tree {
	return &Tree{
		root: &node{
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

	fullPath := path

	i := longestCommonPrefix(path, t.root.path)
	if i > 0 && len(t.root.path) > i {
		t.root.split(i)
	}

	tsr := false
	if path != "/" {
		if strings.HasPrefix(path, t.root.path) {
			path = path[len(t.root.path):]
		}

		if strings.HasSuffix(path, "/") {
			tsr = true
			path = path[:len(path)-1]
		}

		if len(path) == 0 {
			t.root.setHandler(handler, fullPath)

			return
		}
	}

	n := t.root.add(path, fullPath, handler)
	n.tsr = tsr

	if len(t.root.path) == 0 {
		t.root = t.root.children[0]
		t.root.nType = root
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

	if len(path) > len(n.path) {
		if path[:len(n.path)] != n.path {
			return nil, false
		}

		path = path[len(n.path):]

		if len(path) == 1 {
			if path == "/" && n.handler != nil {
				if n.tsr {
					return n.handler, false
				}

				return nil, true
			}
		}

		return n.getFromChild(path, ctx)

	} else if path == n.path {

		switch {
		case n.tsr:
			return nil, true
		case n.handler != nil:
			return n.handler, false
		case n.wildcard != nil:
			if ctx != nil {
				ctx.SetUserValue(n.wildcard.paramKeys[0], "/")
			}

			return n.wildcard.handler, false
		}

	}

	return nil, false
}

// FindCaseInsensitivePath makes a case-insensitive lookup of the given path
// and tries to find a handler.
// It can optionally also fix trailing slashes.
// It returns the case-corrected path and a bool indicating whether the lookup
// was successful.
func (t *Tree) FindCaseInsensitivePath(path string, fixTrailingSlash bool, buf *bytebufferpool.ByteBuffer) bool {
	found, tsr := t.root.find(path, buf)

	if !found || (tsr && !fixTrailingSlash) {
		buf.Reset()

		return false
	}

	return true
}
