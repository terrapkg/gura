package util

import "testing"

func TestSplitOnce(t *testing.T) {
	left, right := SplitOnce("foo:bar:baz", ':')
	if left != "foo" || right != "bar:baz" {
		t.Errorf("SplitOnce failed: got (%q, %q), want (\"foo\", \"bar:baz\")", left, right)
	}
	left, right = SplitOnce("foobar", ':')
	if left != "foobar" || right != "" {
		t.Errorf("SplitOnce failed: got (%q, %q), want (\"foobar\", \"\")", left, right)
	}
}

func TestSanitizeWhite(t *testing.T) {
	got := SanitizeWhite("hello\nworld\rand\tspace")
	want := "hello world and\tspace"
	if got != want {
		t.Errorf("SanitizeWhite failed: got %q, want %q", got, want)
	}
}
