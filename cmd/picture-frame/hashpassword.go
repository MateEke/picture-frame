package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/MateEke/picture-frame/internal/auth"
)

// hashPassword writes the bcrypt hash of a password read from in to out. Reading
// stdin (not argv) keeps the password out of process listings.
func hashPassword(in io.Reader, out io.Writer) error {
	raw, err := io.ReadAll(in)
	if err != nil {
		return fmt.Errorf("read password: %w", err)
	}
	// Strip one trailing newline (echo/heredoc).
	plain := strings.TrimSuffix(strings.TrimSuffix(string(raw), "\n"), "\r")
	hash, err := auth.HashPassword(plain)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	if _, err := fmt.Fprintln(out, hash); err != nil {
		return fmt.Errorf("write hash: %w", err)
	}
	return nil
}
