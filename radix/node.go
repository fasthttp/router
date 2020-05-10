// Copyright 2020-present Sergio Andres Virviescas Santana, fasthttp
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.

// Package radix is a high performance HTTP routes storage.
package radix

import (
	"sort"
	"strings"

	"github.com/valyala/bytebufferpool"
	"github.com/valyala/fasthttp"
)

func newNodeAndHandler(method, path string, lastSegment bool) (*node, *nodeHandler) {
	nHandler := &nodeHandler{tsr: lastSegment && path != "/" && path[len(path)-1] == '/'}

	n := &node{
		nType:    static,
		path:     path,
		handlers: map[string]*nodeHandler{method: nHandler},
	}

	if !lastSegment || path == "/" {
		return n, nHandler
	}

	nHandlerTSR := &nodeHandler{tsr: !nHandler.tsr}

	if nHandler.tsr {
		n.path = n.path[:len(n.path)-1]
	}

	n.children = append(n.children, &node{
		nType:    static,
		path:     "/",
		handlers: map[string]*nodeHandler{method: nHandlerTSR},
	})

	if !nHandler.tsr {
		return n, nHandler
	}

	return n, nHandlerTSR
}

// conflict raises a panic with some details
func (n *nodeWildcard) conflict(path, fullPath string) {
	prefix := fullPath[:strings.Index(fullPath, path)] + n.path

	panicf(
		"'%s' in new path '%s' conflicts with existing wildcard '%s' in existing prefix '%s'",
		path, fullPath, n.path, prefix,
	)
}

// wildPathConflict raises a panic with some details
func (n *node) wildPathConflict(path, fullPath string) {
	pathSeg := strings.SplitN(path, "/", 2)[0]
	prefix := fullPath[:strings.Index(fullPath, pathSeg)] + n.path

	panicf(
		"'%s' in new path '%s' conflicts with existing wildcard '%s' in existing prefix '%s'",
		pathSeg, fullPath, n.path, prefix,
	)
}

// clone clones the current node in a new pointer
func (n node) clone() *node {
	cloneNode := new(node)
	cloneNode.nType = n.nType
	cloneNode.path = n.path
	cloneNode.handlers = n.handlers

	if len(n.children) > 0 {
		cloneNode.children = make([]*node, len(n.children))

		for i, child := range n.children {
			cloneNode.children[i] = child.clone()
		}
	}

	if len(n.paramKeys) > 0 {
		cloneNode.paramKeys = make([]string, len(n.paramKeys))
		copy(cloneNode.paramKeys, n.paramKeys)
	}
	cloneNode.paramRegex = n.paramRegex

	return cloneNode
}

func (n *node) split(i int) {
	cloneChild := n.clone()
	cloneChild.nType = static
	cloneChild.path = cloneChild.path[i:]
	cloneChild.paramKeys = nil
	cloneChild.paramRegex = nil

	n.path = n.path[:i]
	n.handlers = nil
	n.children = append(n.children[:0], cloneChild)
}

func (n *node) findEndIndexAndValues(path string) (int, []string) {
	index := n.paramRegex.FindStringSubmatchIndex(path)
	if len(index) == 0 {
		return -1, nil
	}

	end := index[1]

	index = index[2:]
	values := make([]string, len(index)/2)

	i := 0
	for j := range index {
		if (j+1)%2 != 0 {
			continue
		}

		values[i] = path[index[j-1]:index[j]]

		i++
	}

	return end, values
}

func (n *node) setHandler(method string, handler fasthttp.RequestHandler, fullPath string) {
	if n.handlers == nil {
		n.handlers = make(map[string]*nodeHandler)
	}

	nHandler := n.handlers[method]
	if nHandler == nil {
		nHandler = new(nodeHandler)
		n.handlers[method] = nHandler
	}

	if nHandler.handler != nil || nHandler.tsr {
		panicf("a handle is already registered for path '%s'", fullPath)
	}

	nHandler.handler = handler

	// Set TSR in method
	for i := range n.children {
		nTSR := n.children[i]

		if nTSR.path != "/" {
			continue
		}

		if nTSR.handlers == nil {
			nTSR.handlers = make(map[string]*nodeHandler)
		}

		nTSR.handlers[method] = &nodeHandler{tsr: true}
	}
}

func (n *node) insert(method, path, fullPath string, handler fasthttp.RequestHandler) *node {
	end := segmentEndIndex(path, true)
	newNode, newNodeHandler := newNodeAndHandler(method, path, true)

	wp := findWildPath(path, fullPath)
	if wp != nil {
		lastSegment := len(path) == wp.end

		j := end
		if wp.start > 0 {
			j = wp.start
			lastSegment = false
		}

		newNode, newNodeHandler = newNodeAndHandler(method, path[:j], lastSegment)

		if wp.start > 0 {
			n.children = append(n.children, newNode)

			if !newNodeHandler.tsr {
				newNode.handlers = nil
			}

			return newNode.insert(method, path[j:], fullPath, handler)
		}

		switch wp.pType {
		case param:
			// newNode.path = newNode.path[:wp.end]
			newNode.nType = wp.pType
			newNode.paramKeys = wp.keys
			newNode.paramRegex = wp.regex
		case wildcard:
			if len(path) == end && n.path[len(n.path)-1] != '/' {
				panicf("no / before wildcard in path '%s'", fullPath)
			} else if len(path) != end {
				panicf("wildcard routes are only allowed at the end of the path in path '%s'", fullPath)
			}

			if n.path != "/" && n.path[len(n.path)-1] == '/' {
				n.split(len(n.path) - 1)
				n.handlers = map[string]*nodeHandler{method: {tsr: true}}

				n = n.children[0]
			}

			newNodeWildcard := &nodeWildcard{
				path:     wp.path,
				paramKey: wp.keys[0],
				handler:  handler,
			}

			nHandler := n.handlers[method]
			if nHandler != nil {
				if nHandler.wildcard != nil {
					nHandler.wildcard.conflict(path, fullPath)
				}

				nHandler.wildcard = newNodeWildcard

			} else {
				if n.handlers == nil {
					n.handlers = make(map[string]*nodeHandler)
				}

				newNodeHandler.wildcard = newNodeWildcard
				n.handlers[method] = newNodeHandler
			}

			return n
		}

		path = path[wp.end:]

		if len(path) > 0 && len(newNode.children) == 0 {
			n.children = append(n.children, newNode)

			if !newNodeHandler.tsr {
				newNode.handlers = nil
			}

			return newNode.insert(method, path, fullPath, handler)
		}
	}

	newNodeHandler.handler = handler
	n.children = append(n.children, newNode)

	if newNode.path == "/" {
		// Add TSR when split a edge and the remain path to insert is "/"
		n.handlers = map[string]*nodeHandler{method: {tsr: true}}
	}

	if len(newNode.children) == 1 {
		// New node has a TSR, so get the child node with path "/"
		return newNode.children[0]
	}

	return newNode
}

// add adds the handler to node for the given path
func (n *node) add(method, path, fullPath string, handler fasthttp.RequestHandler) *node {
	if n.path == path || len(path) == 0 {
		n.setHandler(method, handler, fullPath)

		return n
	}

	for _, child := range n.children {
		i := longestCommonPrefix(path, child.path)
		if i == 0 {
			continue
		}

		switch child.nType {
		case static:
			if len(child.path) > i {
				child.split(i)
			}

			if len(path) > i {
				return child.add(method, path[i:], fullPath, handler)
			}
		case param:
			wp := findWildPath(path, fullPath)

			isParam := wp.start == 0 && wp.pType == param
			hasHandler := (child.handlers != nil && child.handlers[method] != nil) || handler == nil

			if len(path) == wp.end && isParam && hasHandler {
				// The current segment is a param and it's duplicated

				child.wildPathConflict(path, fullPath)
			}

			if len(path) > i {
				if child.path == wp.path {
					return child.add(method, path[i:], fullPath, handler)
				}

				return n.insert(method, path, fullPath, handler)
			}
		}

		child.setHandler(method, handler, fullPath)

		return child
	}

	return n.insert(method, path, fullPath, handler)
}

func (n *node) getFromChild(method, path string, ctx *fasthttp.RequestCtx) (fasthttp.RequestHandler, bool) {
walk:
	for {
		for _, child := range n.children {
			switch child.nType {
			case static:

				// Checks if the first byte is equal
				// It's faster than compare strings
				if path[0] != child.path[0] {
					continue
				}

				if len(path) > len(child.path) {
					if path[:len(child.path)] != child.path {
						continue
					}

					path = path[len(child.path):]

					n = child
					continue walk

				} else if path == child.path {
					nHandler := child.handlers[method]

					switch {
					case nHandler == nil:
						return nil, false
					case nHandler.tsr:
						return nil, true
					case nHandler.handler != nil:
						return nHandler.handler, false
					case nHandler.wildcard != nil:
						if ctx != nil {
							ctx.SetUserValue(nHandler.wildcard.paramKey, path)
						}

						return nHandler.wildcard.handler, false
					}
				}

			case param:
				end := segmentEndIndex(path, false)
				values := []string{path[:end]}

				if child.paramRegex != nil {
					end, values = child.findEndIndexAndValues(path[:end])
					if end == -1 {
						continue
					}
				}

				if len(path) > end {
					h, tsr := child.getFromChild(method, path[end:], ctx)
					if tsr {
						return nil, tsr
					} else if h != nil {
						if ctx != nil {
							for i, key := range child.paramKeys {
								ctx.SetUserValue(key, values[i])
							}
						}

						return h, false
					}

				} else if len(path) == end {
					nHandler := child.handlers[method]

					switch {
					case nHandler == nil:
						return nil, false
					case nHandler.tsr:
						return nil, true
					case ctx != nil:
						for i, key := range child.paramKeys {
							ctx.SetUserValue(key, values[i])
						}
					}

					return nHandler.handler, false
				}

			default:
				panic("invalid node type")
			}
		}

		if n.handlers != nil {
			nHandler := n.handlers[method]

			if nHandler != nil && nHandler.wildcard != nil {
				if ctx != nil {
					ctx.SetUserValue(nHandler.wildcard.paramKey, path)
				}

				return nHandler.wildcard.handler, false
			}
		}

		return nil, false
	}
}

func (n *node) find(method, path string, buf *bytebufferpool.ByteBuffer) (bool, bool) {
	if len(path) > len(n.path) {
		if !strings.EqualFold(path[:len(n.path)], n.path) {
			return false, false
		}

		path = path[len(n.path):]
		buf.WriteString(n.path)

		found, tsr := n.findFromChild(method, path, buf)
		if found {
			return found, tsr
		}

		bufferRemoveString(buf, n.path)

	} else if strings.EqualFold(path, n.path) {
		nHandler := n.handlers[method]
		if nHandler == nil {
			return false, false
		}

		buf.WriteString(n.path)

		if nHandler.tsr {
			if n.path == "/" {
				bufferRemoveString(buf, n.path)
			} else {
				buf.WriteByte('/')
			}

			return true, true
		}

		return true, false
	}

	return false, false
}

func (n *node) findFromChild(method, path string, buf *bytebufferpool.ByteBuffer) (bool, bool) {
	for _, child := range n.children {
		switch child.nType {
		case static:
			found, tsr := child.find(method, path, buf)
			if found {
				return found, tsr
			}

		case param:
			end := segmentEndIndex(path, false)

			if child.paramRegex != nil {
				end, _ = child.findEndIndexAndValues(path[:end])
				if end == -1 {
					continue
				}
			}

			buf.WriteString(path[:end])

			if len(path) > end {
				found, tsr := child.findFromChild(method, path[end:], buf)
				if found {
					return found, tsr
				}

			} else if len(path) == end {
				nHandler := child.handlers[method]
				if nHandler == nil {
					return false, false
				}

				if nHandler.tsr {
					buf.WriteByte('/')

					return true, true
				}

				return true, false
			}

			bufferRemoveString(buf, path[:end])

		default:
			panic("invalid node type")
		}
	}

	if n.handlers != nil {
		nHandler := n.handlers[method]

		if nHandler != nil && nHandler.wildcard != nil {
			buf.WriteString(path)

			return true, false
		}
	}

	return false, false
}

// sort sorts the current node and their children
func (n *node) sort() {
	for _, child := range n.children {
		child.sort()
	}

	sort.Sort(n)
}

// Len returns the total number of children the node has
func (n *node) Len() int {
	return len(n.children)
}

// Swap swaps the order of children nodes
func (n *node) Swap(i, j int) {
	n.children[i], n.children[j] = n.children[j], n.children[i]
}

// Less checks if the node 'i' has less priority than the node 'j'
func (n *node) Less(i, j int) bool {
	if n.children[i].nType < n.children[j].nType {
		return true
	} else if n.children[i].nType > n.children[j].nType {
		return false
	}

	return len(n.children[i].children) > len(n.children[j].children)
}
