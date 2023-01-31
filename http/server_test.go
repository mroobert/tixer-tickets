package http_test

import (
	"testing"
)

func Test_SomeMethod(t *testing.T) {
	t.Parallel()

	want := true

	got := true
	if got != want {
		t.Fatal("Invalid method test")
	}
}

func Test_OtherMethod(t *testing.T) {
	t.Parallel()

	want := true

	got := true
	if got != want {
		t.Fatal("Invalid other method test")
	}
}
