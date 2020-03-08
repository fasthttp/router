package radix

import (
	"unicode/utf8"
)

func min(a, b int) int {
	if a <= b {
		return a
	}
	return b
}

// func isIndexEqual(a, b string) bool {
// 	ra, _ := utf8.DecodeRuneInString(a)
// 	rb, _ := utf8.DecodeRuneInString(b)

// 	return unicode.ToLower(ra) == unicode.ToLower(rb)
// }

// longestCommonPrefix finds the longest common prefix.
// This also implies that the common prefix contains no ':' or '*'
// since the existing key can't contain those chars.
func longestCommonPrefix(a, b string) int {
	i := 0
	max := min(utf8.RuneCountInString(a), utf8.RuneCountInString(b))

	for i < max {
		ra, sizeA := utf8.DecodeRuneInString(a)
		rb, sizeB := utf8.DecodeRuneInString(b)

		a = a[sizeA:]
		b = b[sizeB:]

		if ra != rb {
			return i
		}

		i += sizeA
	}

	return i
}

// segmentEndIndex returns the index where the segment ends from the given path
func segmentEndIndex(path string) int {
	end := 0
	for end < len(path) && path[end] != '/' {
		end++
	}

	return end
}

// pathNodeType returns the node type of the given path
func pathNodeType(path string) nodeType {
	switch path[0] {
	case ':':
		return param
	case '*':
		return wildcard
	}

	return static
}

// findWildPath searchs for a wild path segment and check the name for invalid characters.
// Returns -1 as index, if no param/wildcard was found.
func findWildPath(path string) (wilcard string, i int, valid bool) {
	// Find start
	for start, c := range []byte(path) {
		// A wildcard starts with ':' (param) or '*' (wildcard)
		if c != ':' && c != '*' {
			continue
		}

		// Find end and check for invalid characters
		valid = true
		for end, c := range []byte(path[start+1:]) {
			switch c {
			case '/':
				return path[start : start+1+end], start, valid
			case ':', '*':
				panic("only one wildcard per path segment is allowed")
			}
		}
		return path[start:], start, valid
	}
	return "", -1, false
}
