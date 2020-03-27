package radix

import (
	"sort"
	"strings"

	"github.com/valyala/fasthttp"
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

func (n *node) findEndIndexAndValues(path string) (int, []string) {
	index := n.paramRegex.FindStringSubmatchIndex(path)
	if len(index) == 0 {
		return -1, nil
	}

	values := []string{}
	end := index[1]
	index = index[2:]

	for j := range index {
		if (j+1)%2 != 0 {
			continue
		}

		values = append(values, path[index[j-1]:index[j]])
	}

	return end, values
}

func (n *node) split(i int) {
	cloneChild := n.clone()
	cloneChild.path = cloneChild.path[i:]
	cloneChild.nType = static
	cloneChild.paramKeys = nil
	cloneChild.paramRegex = nil

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
	newNode := &node{path: path, nType: static}

	wp := findWildPath(path, fullPath)
	if wp != nil {
		// Finds a valid wilcard/param

		if wp.pType == wildcard && len(path) == end && n.path[len(n.path)-1] != '/' {
			panic("no / before wildcard in path '" + fullPath + "'")
		}

		// Set the index to end the new node path and starts the path of the next node
		j := end
		if wp.start > 0 {
			// If the wild path index it's greater than 0, sets it as the index
			j = wp.start
		}

		if wp.start == 0 {
			newNode.path = wp.path
			newNode.nType = wp.pType
			newNode.paramKeys = wp.keys
			newNode.paramRegex = wp.regex
			j = wp.end
		} else {
			newNode.path = path[:j]
		}

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
			wp := findWildPath(path, fullPath)

			// if wp == nil {
			// 	continue
			// }

			isParam := wp.start == 0 && wp.pType == param
			hasHandler := child.handler != nil || handler == nil

			if len(path) == wp.end && isParam && hasHandler {
				// The current segment is a param and it's duplicated

				child.wildPathConflict(path, fullPath)
			}

			if len(path) > i {
				if child.path == wp.path {
					return child.add(path[i:], fullPath, handler)
				}

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
							for _, key := range child.wildcard.paramKeys {
								ctx.SetUserValue(key, path)
							}
						}

						return child.wildcard.handler, false
					}

					return nil, false
				}

			case param:
				end := segmentEndIndex(path)
				values := []string{path[:end]}

				if child.paramRegex != nil {
					end, values = child.findEndIndexAndValues(path[:end])
					if end == -1 {
						continue
					}
				}

				if child.handler != nil {
					if end == len(path) {
						if child.tsr {
							return nil, true
						}

						if ctx != nil {
							for i, key := range child.paramKeys {
								ctx.SetUserValue(key, values[i])
							}
						}

						return child.handler, false

					} else if path[end:] == "/" {

						if !child.tsr {
							return nil, true
						}

						if ctx != nil {
							for i, key := range child.paramKeys {
								ctx.SetUserValue(key, values[i])
							}
						}

						return child.handler, false

					}
				} else if len(path[end:]) == 0 {
					return nil, false
				}

				h, tsr := child.getFromChild(path[end:], ctx)
				if tsr {
					return nil, tsr
				} else if h != nil {
					for i, key := range child.paramKeys {
						ctx.SetUserValue(key, values[i])
					}

					return h, false
				}

			default:
				panic("invalid node type")
			}
		}

		if n.wildcard != nil {
			if ctx != nil {
				ctx.SetUserValue(n.wildcard.paramKeys[0], path)
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

			if child.paramRegex != nil {
				end, _ = child.findEndIndexAndValues(path[:end])
				if end == -1 {
					continue
				}
			}

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

	if len(n.paramKeys) > 0 {
		cloneNode.paramKeys = make([]string, len(n.paramKeys))

		for i, key := range n.paramKeys {
			cloneNode.paramKeys[i] = key
		}
	}
	cloneNode.paramRegex = n.paramRegex

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
