//+build !go1.12

package router

import "strings"

func toLower(s string) string {
	return strings.ToLower(s)
}
