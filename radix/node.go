package radix

import (
	"sort"
	"strings"

	"github.com/valyala/fasthttp"
)

type nodeType uint8

type node struct {
	path     string
	handler  fasthttp.RequestHandler
	nType    nodeType
	tsr      bool
	children []*node
	wildcard *node
}

const (
	root nodeType = iota
	static
	param
	wildcard
)

// wildPathConflict raises a panic with some details
func (n *node) wildPathConflict(path, fullPath string) {
	pathSeg := path
	if n.nType != wildcard {
		pathSeg = strings.SplitN(pathSeg, "/", 2)[0]
	}
	prefix := fullPath[:strings.Index(fullPath, pathSeg)] + n.path

	panic("'" + pathSeg +
		"' in new path '" + fullPath +
		"' conflicts with existing wildcard '" + n.path +
		"' in existing prefix '" + prefix +
		"'")
}

func (n *node) split(i int) {
	cloneChild := n.clone()
	cloneChild.path = cloneChild.path[i:]
	cloneChild.nType = pathNodeType(cloneChild.path)

	n.path = n.path[:i]
	n.handler = nil
	n.wildcard = nil
	n.children = append(n.children[:0], cloneChild)
}

func (n *node) setHandler(handler fasthttp.RequestHandler, fullPath string) {
	if n.handler != nil {
		panic("a handle is already registered for path '" + fullPath + "'")
	}

	n.handler = handler
}

func (n *node) insert(path, fullPath string, handler fasthttp.RequestHandler) *node {
	end := segmentEndIndex(path)
	newNode := &node{path: path, nType: pathNodeType(path)}

	wildPath, i, valid := findWildPath(path)
	if valid && i >= 0 {
		// Finds a valid wilcard/param

		if len(wildPath) < 2 {
			panic("wildcards must be named with a non-empty name in path '" + fullPath + "'")
		} else if wildPath[0] == '*' && len(path) == end && (wildPath != path || n.path[len(n.path)-1] != '/') {
			panic("no / before wildcard in path '" + fullPath + "'")
		}

		// Set the index to end the new node path and starts the path of the next node
		j := end
		if i > 0 {
			// If the wild path index it's greater than 0, sets it as the index
			j = i
		}

		newNode.path = path[:j]
		newNode.nType = pathNodeType(newNode.path)

		if newNode.path == "/" && n.handler == nil {
			// The current path is '/' and it don't has a handler,
			// so force the trailing slash redirect
			n.tsr = true
		}

		if newNode.nType == wildcard && len(path) != end {
			panic("wildcard routes are only allowed at the end of the path in path '" + fullPath + "'")
		}

		if len(path[j:]) > 0 {
			// Adds the next segment to the new node

			n.children = append(n.children, newNode)
			return newNode.insert(path[j:], fullPath, handler)
		}
	}

	newNode.handler = handler

	if newNode.nType == wildcard {
		// If the new node is a wildcard, set it as wildcard in the current node

		if n.wildcard != nil {
			n.wildcard.wildPathConflict(path, fullPath)
		}

		if len(n.path) > 2 && n.path[len(n.path)-1] == '/' {
			// Splits edge if the path of the current node finish with a slash

			i := len(n.path) - 1
			n.split(i)

			n.tsr = true
			n = n.children[0]
		}

		n.wildcard = newNode

	} else {
		// Adds the new node as a child

		n.children = append(n.children, newNode)
	}

	return newNode
}

// add adds the handler to node for the given path
func (n *node) add(path, fullPath string, handler fasthttp.RequestHandler) *node {
	if n.path == path {
		n.setHandler(handler, fullPath)

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
				return child.add(path[i:], fullPath, handler)
			}
		case param:
			end := segmentEndIndex(path)

			if path[0] == ':' && len(path) == end && (child.handler != nil || handler == nil) {
				// The current segment is a param and it's duplicated

				child.wildPathConflict(path, fullPath)
			}

			if len(path) > end && path[:end] == child.path {
				return child.add(path[end:], fullPath, handler)

			} else if i < 2 {
				// New param, the common string is ':'
				return n.insert(path, fullPath, handler)
			}
		}

		child.setHandler(handler, fullPath)

		return child
	}

	return n.insert(path, fullPath, handler)
}

func (n *node) getFromChild(path string, ctx *fasthttp.RequestCtx) (fasthttp.RequestHandler, bool) {
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

					if len(path) == 1 {
						if path == "/" && child.handler != nil {
							if child.tsr {
								return child.handler, false
							}

							return nil, true
						}
					}

					n = child
					continue walk

				} else if path == child.path {

					switch {
					case child.tsr:
						return nil, true
					case child.handler != nil:
						return child.handler, false
					case child.wildcard != nil:
						if ctx != nil {
							ctx.SetUserValue(child.wildcard.path[1:], path)
						}

						return child.wildcard.handler, false
					}

					return nil, false
				}

			case param:
				end := segmentEndIndex(path)

				if child.handler != nil {
					if end == len(path) {

						if child.tsr {
							return nil, true
						}

						if ctx != nil {
							ctx.SetUserValue(child.path[1:], path[:end])
						}

						return child.handler, false

					} else if path[end:] == "/" {

						if !child.tsr {
							return nil, true
						}

						if ctx != nil {
							ctx.SetUserValue(child.path[1:], path[:end])
						}

						return child.handler, false

					}
				} else if len(path[end:]) == 0 {
					return nil, false
				}

				n2, tsr := child.getFromChild(path[end:], ctx)
				if tsr {
					return nil, tsr
				} else if n2 != nil {
					if ctx != nil {
						ctx.SetUserValue(child.path[1:], path[:end])
					}

					return n2, false
				}

			default:
				panic("invalid node type")
			}
		}

		if n.wildcard != nil {
			if ctx != nil {
				ctx.SetUserValue(n.wildcard.path[1:], path)
			}

			return n.wildcard.handler, false
		}

		return nil, false
	}
}

func (n *node) find(path string, buf []byte) ([]byte, bool) {
	if len(path) > len(n.path) {
		if strings.EqualFold(path[:len(n.path)], n.path) {

			path = path[len(n.path):]
			buf = append(buf, n.path...)

			if len(path) == 1 {
				if path == "/" && n.handler != nil {
					if n.tsr {
						buf = append(buf, '/')

						return buf, false
					}

					return buf, true
				}
			}

			return n.findChild(path, buf)
		}
	} else if strings.EqualFold(path, n.path) && n.handler != nil {
		buf = append(buf, n.path...)

		if n.tsr {
			buf = append(buf, '/')

			return buf, true
		}

		return buf, false
	}

	return nil, false
}

func (n *node) findChild(path string, buf []byte) ([]byte, bool) {
	for _, child := range n.children {
		switch child.nType {
		case static:
			buf2, tsr := child.find(path, buf)
			if buf2 != nil || tsr {
				return buf2, tsr
			}

		case param:
			end := segmentEndIndex(path)

			if child.handler != nil {
				if end == len(path) {
					buf = append(buf, path...)

					if child.tsr {
						buf = append(buf, '/')

						return buf, true
					}

					return buf, false
				} else if path[end:] == "/" {
					buf = append(buf, path[:end]...)

					if child.tsr {
						buf = append(buf, '/')

						return buf, false
					}

					return buf, true
				}
			} else if len(path[end:]) == 0 {
				return nil, false
			}

			buf2, tsr := child.findChild(path[end:], append(buf, path[:end]...))
			if buf2 != nil || tsr {
				return buf2, tsr
			}

		default:
			panic("invalid node type")
		}
	}

	if n.wildcard != nil {
		buf = append(buf, path...)

		return buf, false
	}

	return nil, false
}

// clone clones the current node in a new pointer
func (n node) clone() *node {
	cloneNode := new(node)
	cloneNode.path = n.path
	cloneNode.handler = n.handler
	cloneNode.nType = n.nType
	cloneNode.tsr = n.tsr

	if n.wildcard != nil {
		cloneNode.wildcard = n.wildcard.clone()
	}

	if len(n.children) > 0 {
		cloneNode.children = make([]*node, len(n.children))

		for i, child := range n.children {
			cloneNode.children[i] = child.clone()
		}
	}

	return cloneNode
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
