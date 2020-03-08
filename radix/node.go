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

// add adds the handler to node for the given path
func (n *node) add(path, fullPath string, handler fasthttp.RequestHandler) (*node, string) {
	end := segmentEndIndex(path)

	for _, child := range n.children {
		switch child.nType {
		case static:

			// Find the longest common prefix.
			// This also implies that the common prefix contains no ':' or '*'
			// since the existing key can't contain those chars.
			i := longestCommonPrefix(path, child.path)

			if i > 0 && len(child.path) > i {
				// Splits edge because has the same prefix

				cloneChild := child.clone()
				cloneChild.path = cloneChild.path[i:]

				child.path = child.path[:i]
				child.handler = nil
				child.wildcard = nil
				child.children = append(child.children[:0], cloneChild)

				if len(path[i:]) == 0 {
					//It's the last segment

					child.handler = handler

					return child, ""
				}

				// Adds the next segment to the child
				return child.add(path[i:], fullPath, handler)
			}

			if len(path) > len(child.path) {
				// Checks if the path prefix is equal than the child path

				if path[:len(child.path)] == child.path {
					// Adds the next segment to the child

					return child.add(path[len(child.path):], fullPath, handler)
				}
			} else if path == child.path {
				// Last segment, so adds the handler to the node

				if child.handler != nil {
					panic("a handle is already registered for path '" + fullPath + "'")
				}

				child.handler = handler

				return child, ""
			}
		case param:

			if path[0] == ':' && len(path) == end && (child.handler != nil || handler == nil) {
				// The current segment is a param and it's duplicated

				child.wildPathConflict(path, fullPath)
			}

			if path == child.path {
				// Last segment, so adds the handler to the pre-registered node without handler

				child.handler = handler

				return child, ""
			} else if path[:end] == child.path {
				// Checks if the path prefix is equal than the child path

				return child.add(path[end:], fullPath, handler)
			}
		}
	}

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
			return newNode.add(path[j:], fullPath, handler)
		}
	}

	newNode.handler = handler

	if newNode.nType == wildcard {
		// If the new node is a wildcard, set it as wildcard in the current node

		if n.wildcard != nil {
			n.wildcard.wildPathConflict(path, fullPath)
		}

		n.wildcard = newNode

		if len(n.path) > 2 && n.path[len(n.path)-1] == '/' {
			// Splits edge if the path of the current node finish with a slash

			i := len(n.path) - 1
			cloneChild := n.clone()
			cloneChild.path = n.path[i:]

			n.path = n.path[:i]
			n.handler = nil
			n.wildcard = nil
			n.tsr = true
			n.children = append(n.children[:0], cloneChild)
		}
	} else {
		// Adds the new node as a child

		n.children = append(n.children, newNode)
	}

	return newNode, ""
}

// get gets the child node for the given path
func (n *node) get(path string, ctx *fasthttp.RequestCtx) (*node, string) {
	for _, child := range n.children {
		switch child.nType {
		case static:

			// Checks if the first node of the path and child path is equal
			// It's faster than compare strings
			if path[0] != child.path[0] {
				continue
			}

			if len(path) > len(child.path) {
				// Checks if the path prefix is equal than the child path

				if path[:len(child.path)] == child.path {
					// Same prefix, so returns the child node

					return child, path[len(child.path):]
				}
			} else if path == child.path {
				// Same path, so returns the child node

				return child, path[len(child.path):]
			}
		case param:
			end := segmentEndIndex(path)

			if end == len(path) {
				// Last segment, so sets the value and finish

				if ctx != nil {
					ctx.SetUserValue(child.path[1:], path)
				}

				return child, ""
			} else if path[end:] == "/" {
				// Last segment, so sets the value and finishes it returning the remaining slash

				if ctx != nil {
					ctx.SetUserValue(child.path[1:], path[:end])
				}

				return child, "/"
			}

			// Recursive search to know if the current branch it's correct
			n2, path2 := child.get(path[end:], ctx)
			if n2 != nil {
				// It's the correct branch so sets the value and continues
				if ctx != nil {
					ctx.SetUserValue(child.path[1:], path[:end])
				}

				return n2, path2
			}
		default:
			panic("invalid node type")
		}
	}

	return n.wildcard, path
}

// getInsensitive gets the child node for the given case insensitive path
func (n *node) getInsensitive(path string, buf []byte) (*node, string, []byte) {
	for _, child := range n.children {
		switch child.nType {
		case static:
			// if !isIndexEqual(path, child.path) {
			// 	continue
			// }

			if len(path) > len(child.path) {
				if !strings.EqualFold(path[:len(child.path)], child.path) {
					// Checks if the path prefix is equal than the child path
					continue
				}

				if path[len(child.path):] == "/" {
					// Found it, because it's the last segment with a trailing slash

					return child, "/", append(buf, child.path...)
				}

				// Recursive search to know if the current branch it's correct
				n2, path2, buf2 := child.getInsensitive(path[len(child.path):], append(buf, child.path...))
				if n2 != nil {
					// It's the correct branch so continues

					return n2, path2, buf2
				}

			} else if strings.EqualFold(path, child.path) {
				// Found it

				return child, path[len(child.path):], append(buf, child.path...)
			}
		case param:
			end := segmentEndIndex(path)

			if end == len(path) {
				// Found it

				return child, "", append(buf, path...)
			} else if path[end:] == "/" {
				// Found it, because it's the last segment with a trailing slash

				return child, "/", append(buf, path[:end]...)
			}

			// Recursive search to know if the current branch it's correct
			n2, path2, _ := child.getInsensitive(path[end:], buf)
			if n2 != nil {
				// It's the correct branch so sets the value and continues

				buf = append(buf, path[:end]...)
				return n2, path2, append(buf, n2.path...)
			}
		default:
			panic("invalid node type")
		}
	}

	return n.wildcard, path, buf
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
