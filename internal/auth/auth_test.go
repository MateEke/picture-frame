package auth_test

import (
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/MateEke/picture-frame/internal/auth"
)

func TestHashPasswordRoundTrip(t *testing.T) {
	hash, err := auth.HashPassword("hunter2")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "" {
		t.Fatal("HashPassword returned empty hash")
	}
	if hash == "hunter2" {
		t.Fatal("HashPassword returned the plaintext")
	}
	if !auth.CheckPassword(hash, "hunter2") {
		t.Fatal("CheckPassword rejected the correct password")
	}
	if auth.CheckPassword(hash, "wrong") {
		t.Fatal("CheckPassword accepted the wrong password")
	}
}

func TestHashPasswordRejectsOverlongInput(t *testing.T) {
	// bcrypt refuses passwords longer than 72 bytes.
	long := strings.Repeat("x", 73)
	if _, err := auth.HashPassword(long); err == nil {
		t.Fatal("expected an error hashing an over-72-byte password")
	}
}

func TestHashPasswordSaltsEachCall(t *testing.T) {
	h1, _ := auth.HashPassword("same")
	h2, _ := auth.HashPassword("same")
	if h1 == h2 {
		t.Fatal("expected distinct hashes for the same password (missing salt)")
	}
}

func TestCheckPasswordRejectsMalformedHash(t *testing.T) {
	if auth.CheckPassword("not-a-bcrypt-hash", "anything") {
		t.Fatal("CheckPassword accepted against a malformed hash")
	}
}

func TestIssueVerifyRoundTrip(t *testing.T) {
	a := auth.New()
	hash, _ := auth.HashPassword("pw")
	cookie := a.Issue(hash)
	if cookie == "" {
		t.Fatal("Issue returned empty cookie")
	}
	if !a.Verify(hash, cookie) {
		t.Fatal("Verify rejected a freshly issued cookie")
	}
}

func TestVerifyRejectsDifferentHash(t *testing.T) {
	a := auth.New()
	hashA, _ := auth.HashPassword("a")
	hashB, _ := auth.HashPassword("b")
	cookie := a.Issue(hashA)
	if a.Verify(hashB, cookie) {
		t.Fatal("Verify accepted a cookie signed under a different password hash")
	}
}

func TestVerifyRejectsTamperedCookie(t *testing.T) {
	a := auth.New()
	hash, _ := auth.HashPassword("pw")
	cookie := a.Issue(hash)
	tampered := cookie + "x"
	if a.Verify(hash, tampered) {
		t.Fatal("Verify accepted a tampered cookie")
	}
}

func TestVerifyRejectsMalformedCookie(t *testing.T) {
	a := auth.New()
	hash, _ := auth.HashPassword("pw")
	for _, c := range []string{"", "garbage", "1", "abc.def.ghi", "notanumber.sig"} {
		if a.Verify(hash, c) {
			t.Fatalf("Verify accepted malformed cookie %q", c)
		}
	}
}

func TestVerifyRejectsExpiredCookie(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		a := auth.New()
		hash, _ := auth.HashPassword("pw")
		cookie := a.Issue(hash)
		if !a.Verify(hash, cookie) {
			t.Fatal("cookie should be valid immediately after issue")
		}
		time.Sleep(a.TTL + time.Minute)
		if a.Verify(hash, cookie) {
			t.Fatal("Verify accepted an expired cookie")
		}
	})
}

func TestVerifyAcceptsBeforeExpiry(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		a := auth.New()
		hash, _ := auth.HashPassword("pw")
		cookie := a.Issue(hash)
		time.Sleep(a.TTL - time.Minute)
		if !a.Verify(hash, cookie) {
			t.Fatal("Verify rejected a cookie that has not yet expired")
		}
	})
}

func TestVerifyRejectsCookieAtExactExpiry(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		a := &auth.Authenticator{TTL: 0} // exp == now
		hash := "$2a$10$fakehashfakehashfakehash"
		cookie := a.Issue(hash)
		if a.Verify(hash, cookie) {
			t.Error("cookie at exact expiry must be invalid")
		}
	})
}
