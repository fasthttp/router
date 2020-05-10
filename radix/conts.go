// Copyright 2020-present Sergio Andres Virviescas Santana, fasthttp
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.

// Package radix is a high performance HTTP routes storage.
package radix

const stackBufSize = 128

const (
	root nodeType = iota
	static
	param
	wildcard
)

// MethodWild wild HTTP method
const MethodWild = "*"
