package router

import "strings"

func validatePath(path string) {
	switch {
	case len(path) == 0 || !strings.HasPrefix(path, "/"):
		panic("path must begin with '/' in path '" + path + "'")
	}
}
