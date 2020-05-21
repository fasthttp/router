package radix

const stackBufSize = 128

const (
	root nodeType = iota
	static
	param
	wildcard
)
