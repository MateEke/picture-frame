package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/MateEke/picture-frame/internal/auth"
)

func TestHashPasswordRoundTrips(t *testing.T) {
	var out bytes.Buffer
	if err := hashPassword(strings.NewReader("hunter2\n"), &out); err != nil {
		t.Fatalf("hashPassword: %v", err)
	}
	hash := strings.TrimSpace(out.String())
	if hash == "" || hash == "hunter2" {
		t.Fatalf("unexpected hash output: %q", hash)
	}
	if !auth.CheckPassword(hash, "hunter2") {
		t.Fatal("CheckPassword rejected the hash for the original password")
	}
}

func TestHashPasswordWithoutTrailingNewline(t *testing.T) {
	var out bytes.Buffer
	if err := hashPassword(strings.NewReader("s3cret"), &out); err != nil {
		t.Fatalf("hashPassword: %v", err)
	}
	if !auth.CheckPassword(strings.TrimSpace(out.String()), "s3cret") {
		t.Fatal("password without a trailing newline did not round-trip")
	}
}

// CRLF (the case the double TrimSuffix exists for) must strip to the bare password.
func TestHashPasswordStripsCRLF(t *testing.T) {
	var out bytes.Buffer
	if err := hashPassword(strings.NewReader("p4ss\r\n"), &out); err != nil {
		t.Fatalf("hashPassword: %v", err)
	}
	if !auth.CheckPassword(strings.TrimSpace(out.String()), "p4ss") {
		t.Fatal("CRLF-terminated password did not round-trip to the bare password")
	}
}
