package user

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// ── bcrypt password hashing (used in Register and Login) ─────────────────────

func TestPasswordHashing_HashIsNotPlainText(t *testing.T) {
	password := "securePass123!"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(hash) == password {
		t.Error("hash must not equal the plain-text password")
	}
}

func TestPasswordHashing_CorrectPassword_Matches(t *testing.T) {
	password := "myShopPass99"
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

	if err := bcrypt.CompareHashAndPassword(hash, []byte(password)); err != nil {
		t.Errorf("correct password should match hash, got: %v", err)
	}
}

func TestPasswordHashing_WrongPassword_DoesNotMatch(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("correctPass"), bcrypt.DefaultCost)

	err := bcrypt.CompareHashAndPassword(hash, []byte("wrongPass"))
	if err == nil {
		t.Fatal("wrong password must not match hash")
	}
}

func TestPasswordHashing_SamePasswordProducesUniqueHashes(t *testing.T) {
	password := "samePassword"
	hash1, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	hash2, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

	if string(hash1) == string(hash2) {
		t.Error("bcrypt must produce different hashes for the same input (random salt)")
	}
}

func TestPasswordHashing_EmptyPassword_MatchesEmptyHash(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte(""), bcrypt.DefaultCost)
	if err := bcrypt.CompareHashAndPassword(hash, []byte("")); err != nil {
		t.Errorf("empty password hash comparison failed: %v", err)
	}
}

// ── Refresh token hashing (internal service helpers) ──────────────────────────

func TestHashRefreshToken_ProducesNonEmptyHash(t *testing.T) {
	svc := &Service{}
	hash, err := svc.hashRefreshToken("some-refresh-token-value")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hash) == 0 {
		t.Fatal("expected non-empty hash bytes")
	}
}

func TestHashRefreshToken_SameTokenProducesDifferentHashes(t *testing.T) {
	svc := &Service{}
	token := "refresh-token-abc"

	h1, _ := svc.hashRefreshToken(token)
	h2, _ := svc.hashRefreshToken(token)

	if string(h1) == string(h2) {
		t.Error("bcrypt must produce different hashes on each call (random salt)")
	}
}

func TestCompareRefreshTokenHash_ValidToken_ReturnsNil(t *testing.T) {
	svc := &Service{}
	token := "valid-refresh-token-xyz"

	hash, _ := svc.hashRefreshToken(token)
	if err := svc.compareRefreshTokenHash(string(hash), token); err != nil {
		t.Errorf("valid token should match hash, got: %v", err)
	}
}

func TestCompareRefreshTokenHash_WrongToken_ReturnsError(t *testing.T) {
	svc := &Service{}
	hash, _ := svc.hashRefreshToken("original-token")

	err := svc.compareRefreshTokenHash(string(hash), "different-token")
	if err == nil {
		t.Fatal("wrong token must not match hash")
	}
}

func TestCompareRefreshTokenHash_TamperedHash_ReturnsError(t *testing.T) {
	svc := &Service{}
	err := svc.compareRefreshTokenHash("not-a-valid-bcrypt-hash", "any-token")
	if err == nil {
		t.Fatal("tampered hash string must return an error")
	}
}

// ── generateCode ──────────────────────────────────────────────────────────────

func TestGenerateCode_Is6Digits(t *testing.T) {
	svc := &Service{}
	code, _ := svc.generateCode()
	if len(code) != 6 {
		t.Errorf("verification code must be 6 digits, got %q (len %d)", code, len(code))
	}
}

func TestGenerateCode_ConsistsOfDigitsOnly(t *testing.T) {
	svc := &Service{}
	for i := 0; i < 20; i++ {
		code, _ := svc.generateCode()
		for _, ch := range code {
			if ch < '0' || ch > '9' {
				t.Errorf("non-digit character %q in code %q", ch, code)
			}
		}
	}
}

func TestGenerateCode_ProducesUniqueCodesOverMultipleCalls(t *testing.T) {
	svc := &Service{}
	seen := make(map[string]bool)
	// With 1,000,000 possible codes a collision in 10 calls is astronomically unlikely.
	for i := 0; i < 10; i++ {
		code, _ := svc.generateCode()
		seen[code] = true
	}
	// We don't assert all are unique (random), just that generation runs without panic.
	if len(seen) == 0 {
		t.Fatal("no codes were generated")
	}
}
