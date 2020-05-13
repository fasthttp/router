// Copyright 2020-present Sergio Andres Virviescas Santana, fasthttp
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.

// Package radix is a high performance HTTP routes storage.
package radix

import (
	"strings"

	"github.com/valyala/bytebufferpool"
	"github.com/valyala/fasthttp"
)

// New returns an empty routes storage
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
func (t *Tree) Add(method, path string, handler fasthttp.RequestHandler) {
	if !strings.HasPrefix(path, "/") {
		panicf("path must begin with '/' in path '%s'", path)
	} else if handler == nil {
		panic("nil handler")
	}

	fullPath := path

	i := longestCommonPrefix(path, t.root.path)
	if i > 0 {
		if len(t.root.path) > i {
			t.root.split(i)
		}

		path = path[i:]
	}

	t.root.add(method, path, fullPath, handler)

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
func (t *Tree) Get(method, path string, ctx *fasthttp.RequestCtx) (fasthttp.RequestHandler, bool) {
	if len(path) > len(t.root.path) {
		if path[:len(t.root.path)] != t.root.path {
			return nil, false
		}

		path = path[len(t.root.path):]

		return t.root.getFromChild(method, path, ctx)

	} else if path == t.root.path {
		nHandler := t.root.handlers[method]

		switch {
		case nHandler == nil:
		case nHandler.tsr:
			return nil, true
		case nHandler.handler != nil:
			return nHandler.handler, false
		case nHandler.wildcard != nil:
			if ctx != nil {
				ctx.SetUserValue(nHandler.wildcard.paramKey, "/")
			}

			return nHandler.wildcard.handler, false
		}

		return t.root.getFromMethodWild(ctx, "/")

	}

	return nil, false
}

// FindCaseInsensitivePath makes a case-insensitive lookup of the given path
// and tries to find a handler.
// It can optionally also fix trailing slashes.
// It returns the case-corrected path and a bool indicating whether the lookup
// was successful.
func (t *Tree) FindCaseInsensitivePath(method, path string, fixTrailingSlash bool, buf *bytebufferpool.ByteBuffer) bool {
	found, tsr := t.root.find(method, path, buf)

	if !found || (tsr && !fixTrailingSlash) {
		buf.Reset()

		return false
	}

	return true
}
