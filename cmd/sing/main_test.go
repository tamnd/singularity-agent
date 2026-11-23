package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestExitCodes(t *testing.T) {
	for _, tc := range []struct {
		args []string
		code int
		out  string
	}{
		{nil, 0, "Usage: sing"},
		{[]string{"version"}, 0, "sing dev"},
		{[]string{"--version"}, 0, "sing dev"},
		{[]string{"version", "x"}, 2, ""},
		{[]string{"nothing"}, 2, ""},
		{[]string{"lab"}, 3, ""},
		{[]string{"trust"}, 3, ""},
		{[]string{"site"}, 2, ""},
		{[]string{"status", "-only", "done"}, 2, ""},
		{[]string{"check", "-content", "/does/not/exist"}, 1, ""},
	} {
		var out, errb bytes.Buffer
		if got := run(tc.args, &out, &errb); got != tc.code {
			t.Errorf("sing %v: exit %d, want %d (%s)", tc.args, got, tc.code, errb.String())
		}
		if !strings.Contains(out.String(), tc.out) {
			t.Errorf("sing %v: the output does not have %q", tc.args, tc.out)
		}
	}
}

func TestNamesAreUnique(t *testing.T) {
	n := names()
	for i := 1; i < len(n); i++ {
		if n[i] == n[i-1] {
			t.Errorf("two commands have the name %s", n[i])
		}
	}
}
