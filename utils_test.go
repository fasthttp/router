package router

import "testing"

func Test_validatePath(t *testing.T) {
	if err := catchPanic(func() { validatePath("") }); err == nil {
		t.Error("an error was expected with an empty path")
	}

	if err := catchPanic(func() { validatePath("foo") }); err == nil {
		t.Error("an error was expected with an empty path")
	}

	if err := catchPanic(func() { validatePath("/foo") }); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
