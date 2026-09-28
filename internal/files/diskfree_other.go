//go:build !unix

package files

func diskFree(string) (uint64, bool) { return 0, false }
