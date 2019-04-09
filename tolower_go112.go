// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the "third_party_licenses/Go (golang)" file.

//+build go1.12

package router

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// toLower returns a copy of the string s with all Unicode letters mapped to their lower case.
func toLower(s string) string {
	isASCII, hasUpper := true, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= utf8.RuneSelf {
			isASCII = false
			break
		}
		hasUpper = hasUpper || (c >= 'A' && c <= 'Z')
	}

	if isASCII { // optimize for ASCII-only strings.
		if !hasUpper {
			return s
		}
		var b strings.Builder
		b.Grow(len(s))
		for i := 0; i < len(s); i++ {
			c := s[i]
			if c >= 'A' && c <= 'Z' {
				c += 'a' - 'A'
			}
			b.WriteByte(c)
		}
		return b.String()
	}
	return stringsMap(unicode.ToLower, s)
}

// Map returns a copy of the string s with all its characters modified
// according to the mapping function. If mapping returns a negative value, the character is
// dropped from the string with no replacement.
func stringsMap(mapping func(rune) rune, s string) string {
	// In the worst case, the string can grow when mapped, making
	// things unpleasant. But it's so rare we barge in assuming it's
	// fine. It could also shrink but that falls out naturally.

	// The output buffer b is initialized on demand, the first
	// time a character differs.
	var b strings.Builder

	for i, c := range s {
		r := mapping(c)
		if r == c {
			continue
		}

		b.Grow(len(s) + utf8.UTFMax)
		b.WriteString(s[:i])
		if r >= 0 {
			b.WriteRune(r)
		}

		if c == utf8.RuneError {
			// RuneError is the result of either decoding
			// an invalid sequence or '\uFFFD'. Determine
			// the correct number of bytes we need to advance.
			_, w := utf8.DecodeRuneInString(s[i:])
			i += w
		} else {
			i += utf8.RuneLen(c)
		}

		s = s[i:]
		break
	}

	// Fast path for unchanged input
	if b.Cap() == 0 { // didn't call b.Grow above
		return s
	}

	for _, c := range s {
		r := mapping(c)

		if r >= 0 {
			// common case
			// Due to inlining, it is more performant to determine if WriteByte should be
			// invoked rather than always call WriteRune
			if r < utf8.RuneSelf {
				b.WriteByte(byte(r))
			} else {
				// r is not a ASCII rune.
				b.WriteRune(r)
			}
		}
	}

	return b.String()
}
