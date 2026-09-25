package auth

import (
	"golang.org/x/crypto/bcrypt"
	"testing"
	"time"
)

func TestTokensAndLimiter(t *testing.T) {
	a, b := Token(), Token()
	if a == b || len(a) != 64 || Hash(a) == a {
		t.Fatal("invalid random token")
	}
	if !CheckCSRF(a, CSRF(a)) || CheckCSRF(a, CSRF(b)) || CheckCSRF(a, "") {
		t.Fatal("CSRF mismatch")
	}
	l := Limiter{Limit: 2, Window: time.Minute}
	now := time.Now()
	if !l.Allow(now) || !l.Allow(now) || l.Allow(now) || !l.Allow(now.Add(time.Minute)) {
		t.Fatal("limiter window")
	}
}
func TestDummyHashCost(t *testing.T) {
	cost, err := bcrypt.Cost([]byte(dummyHash))
	if err != nil || cost != 12 {
		t.Fatalf("dummy hash cost %d %v", cost, err)
	}
}
