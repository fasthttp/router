// Copyright 2013 Julien Schmidt. All rights reserved.
// Based on the path package, Copyright 2009 The Go Authors.
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.

package router

import (
	"strings"
	"sync"

	"github.com/savsgio/gotils"
)

type cleanPathBuffer struct {
	n        int
	r        int
	w        int
	trailing bool
	buf      []byte
}

var cleanPathBufferPool = sync.Pool{
	New: func() interface{} {
		return &cleanPathBuffer{
			n:        0,
			r:        0,
			w:        1,
			trailing: false,
			buf:      make([]byte, 140),
		}
	},
}

func (cpb *cleanPathBuffer) reset() {
	cpb.n = 0
	cpb.r = 0
	cpb.w = 1
	cpb.trailing = false
	// cpb.buf = cpb.buf[:0]
}

func acquireCleanPathBuffer() *cleanPathBuffer {
	return cleanPathBufferPool.Get().(*cleanPathBuffer)
}

func releaseCleanPathBuffer(cpb *cleanPathBuffer) {
	cpb.reset()
	cleanPathBufferPool.Put(cpb)
}

// CleanPath is the URL version of path.Clean, it returns a canonical URL path
// for path, eliminating . and .. elements.
//
// The following rules are applied iteratively until no further processing can
// be done:
//	1. Replace multiple slashes with a single slash.
//	2. Eliminate each . path name element (the current directory).
//	3. Eliminate each inner .. path name element (the parent directory)
//	   along with the non-.. element that precedes it.
//	4. Eliminate .. elements that begin a rooted path:
//	   that is, replace "/.." by "/" at the beginning of a path.
//
// If the result of this process is an empty string, "/" is returned
func CleanPath(path string) string {
	cpb := acquireCleanPathBuffer()
	cleanPathWithBuffer(cpb, path)

	s := string(cpb.buf)
	releaseCleanPathBuffer(cpb)

	return s
}

func cleanPathWithBuffer(cpb *cleanPathBuffer, path string) {
	// Turn empty string into "/"
	if path == "" {
		cpb.buf = append(cpb.buf[:0], '/')
		return
	}

	cpb.n = len(path)
	cpb.buf = gotils.ExtendByteSlice(cpb.buf, len(path)+1)
	cpb.buf[0] = '/'

	cpb.trailing = cpb.n > 2 && path[cpb.n-1] == '/'

	// A bit more clunky without a 'lazybuf' like the path package, but the loop
	// gets completely inlined (bufApp). So in contrast to the path package this
	// loop has no expensive function calls (except 1x make)

	for cpb.r < cpb.n {
		// println(path[:cpb.r], " ####### ", string(path[cpb.r]), " ####### ", string(cpb.buf))
		switch {
		case path[cpb.r] == '/':
			// empty path element, trailing slash is added after the end
			cpb.r++

		case path[cpb.r] == '.' && cpb.r+1 == cpb.n:
			cpb.trailing = true
			cpb.r++

		case path[cpb.r] == '.' && path[cpb.r+1] == '/':
			// . element
			cpb.r++

		case path[cpb.r] == '.' && path[cpb.r+1] == '.' && (cpb.r+2 == cpb.n || path[cpb.r+2] == '/'):
			// .. element: remove to last /
			cpb.r += 2

			if cpb.w > 1 {
				// can backtrack
				cpb.w--

				for cpb.w > 1 && cpb.buf[cpb.w] != '/' {
					cpb.w--
				}

			}

		default:
			// real path element.
			// add slash if needed
			if cpb.w > 1 {
				cpb.buf[cpb.w] = '/'
				cpb.w++
			}

			// copy element
			for cpb.r < cpb.n && path[cpb.r] != '/' {
				cpb.buf[cpb.w] = path[cpb.r]
				cpb.w++
				cpb.r++
			}
		}
	}

	// re-append trailing slash
	if cpb.trailing && cpb.w > 1 {
		cpb.buf[cpb.w] = '/'
		cpb.w++
	}

	cpb.buf = cpb.buf[:cpb.w]
}

// returns all possible paths when the original path has optional arguments
func getOptionalPaths(path string) []string {
	paths := make([]string, 0)

	index := 0
	newParam := false
	for i := 0; i < len(path); i++ {
		c := path[i]

		if c == ':' {
			index = i
			newParam = true
		} else if i > 0 && newParam && c == '?' {
			p := strings.Replace(path[:index], "?", "", -1)
			if !gotils.StringSliceInclude(paths, p) {
				paths = append(paths, p)
			}

			p = strings.Replace(path[:i], "?", "", -1) + "/"
			if !gotils.StringSliceInclude(paths, p) {
				paths = append(paths, p)
			}

			newParam = false
		}
	}

	return paths
}
