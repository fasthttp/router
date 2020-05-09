// Copyright 2020-present Sergio Andres Virviescas Santana, fasthttp
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.
package radix

const stackBufSize = 128

const (
	root nodeType = iota
	static
	param
	wildcard
)
