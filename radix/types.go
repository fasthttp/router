package radix

import (
	"regexp"

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

	paramKeys  []string
	paramRegex *regexp.Regexp
}

type Tree struct {
	root *node
}

type wildPath struct {
	path  string
	keys  []string
	start int
	end   int
	pType nodeType

	pattern string
	regex   *regexp.Regexp
}
