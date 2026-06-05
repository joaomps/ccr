package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersion(t *testing.T) {
	var out, errb bytes.Buffer
	if code := Run([]string{"version"}, &out, &errb); code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, errb.String())
	}
	if strings.TrimSpace(out.String()) == "" {
		t.Fatal("version printed nothing")
	}
}

func TestUnknownSubcommand(t *testing.T) {
	var out, errb bytes.Buffer
	if code := Run([]string{"bogus"}, &out, &errb); code == 0 {
		t.Fatal("expected non-zero exit for unknown subcommand")
	}
}

func TestNoArgs(t *testing.T) {
	var out, errb bytes.Buffer
	if code := Run(nil, &out, &errb); code == 0 {
		t.Fatal("expected non-zero exit for no args")
	}
}
