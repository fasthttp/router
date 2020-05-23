package router

import "github.com/savsgio/gotils"

// cleanPath removes the '.' if it is the last character of the route
func cleanPath(path string) string {
	lenPath := len(path)

	if path[lenPath-1] == '.' {
		path = path[:lenPath-1]
	}

	return path
}

// getOptionalPaths returns all possible paths when the original path
// has optional arguments
func getOptionalPaths(path string) []string {
	paths := make([]string, 0)

	start := 0
walk:
	for {
		if start >= len(path) {
			return paths
		}

		c := path[start]
		start++

		if c != '{' {
			continue
		}

		newPath := ""
		questionMarkIndex := -1

		for end, c := range []byte(path[start:]) {
			switch c {
			case '}':
				if questionMarkIndex == -1 {
					continue walk
				}

				end++
				newPath += path[questionMarkIndex+1 : start+end]

				path = path[:questionMarkIndex] + path[questionMarkIndex+1:] // remove '?'
				paths = append(paths, newPath)
				start += end - 1

				continue walk

			case '?':
				questionMarkIndex = start + end
				newPath += path[:questionMarkIndex]

				// include the path without the wildcard
				// -2 due to remove the '/' and '{'
				if !gotils.StringSliceInclude(paths, path[:start-2]) {
					paths = append(paths, path[:start-2])
				}
			}
		}
	}
}
