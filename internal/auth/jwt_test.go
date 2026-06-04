package auth

import (
	"strings"
	"testing"
	"time"
)

const testSecret = "unit-test-jwt-secret-32-chars-ok"
const testRefreshSecret = "unit-test-refresh-secret-32chars"

// ── CreateAccessToken ─────────────────────────────────────────────────────────

func TestCreateAccessToken_ReturnsNonEmptyToken(t *testing.T) {
	token, err := CreateAccessToken(testSecret, "user123", "owner")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token string")
	}
}

func TestCreateAccessToken_TokenHasThreeParts(t *testing.T) {
	token, _ := CreateAccessToken(testSecret, "user123", "owner")
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Errorf("JWT must have 3 dot-separated parts, got %d", len(parts))
	}
}

func TestCreateRefreshToken_ReturnsNonEmptyToken(t *testing.T) {
	token, err := CreateRefreshToken(testRefreshSecret, "user123", "owner")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty refresh token")
	}
}

func TestRefreshTokenExpiry_LongerThanAccessToken(t *testing.T) {
	access, _ := CreateAccessToken(testSecret, "user123", "owner")
	refresh, _ := CreateRefreshToken(testRefreshSecret, "user123", "owner")

	accessClaims, _ := ParseToken(testSecret, access)
	refreshClaims, _ := ParseToken(testRefreshSecret, refresh)

	if !refreshClaims.ExpiresAt.After(accessClaims.ExpiresAt.Time) {
		t.Error("refresh token must expire after access token")
	}
}

func TestAccessToken_ExpiryApproximately60Minutes(t *testing.T) {
	token, _ := CreateAccessToken(testSecret, "user123", "owner")
	claims, _ := ParseToken(testSecret, token)

	ttl := time.Until(claims.ExpiresAt.Time)
	expected := 60 * time.Minute
	delta := 5 * time.Second

	if ttl > expected+delta || ttl < expected-delta {
		t.Errorf("access token TTL: want ~%v, got %v", expected, ttl)
	}
}

// ── ParseToken — valid inputs ─────────────────────────────────────────────────

func TestParseToken_ExtractsSubjectCorrectly(t *testing.T) {
	userID := "507f1f77bcf86cd799439011"
	token, _ := CreateAccessToken(testSecret, userID, "owner")

	claims, err := ParseToken(testSecret, token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.Subject != userID {
		t.Errorf("subject: want %q, got %q", userID, claims.Subject)
	}
}

func TestParseToken_ExtractsOwnerRoleCorrectly(t *testing.T) {
	token, _ := CreateAccessToken(testSecret, "user123", "owner")

	claims, err := ParseToken(testSecret, token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.Role != "owner" {
		t.Errorf("role: want %q, got %q", "owner", claims.Role)
	}
}

func TestParseToken_ExtractsStaffRoleCorrectly(t *testing.T) {
	token, _ := CreateAccessToken(testSecret, "staff-uuid-abc", "staff")

	claims, err := ParseToken(testSecret, token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.Role != "staff" {
		t.Errorf("role: want %q, got %q", "staff", claims.Role)
	}
}

// ── ParseToken — invalid inputs ───────────────────────────────────────────────

func TestParseToken_WrongSecret_ReturnsError(t *testing.T) {
	token, _ := CreateAccessToken(testSecret, "user123", "owner")

	_, err := ParseToken("wrong-secret-entirely", token)
	if err == nil {
		t.Fatal("expected error when verifying with wrong secret, got nil")
	}
}

func TestParseToken_TamperedToken_ReturnsError(t *testing.T) {
	_, err := ParseToken(testSecret, "header.payload.invalidsignature")
	if err == nil {
		t.Fatal("expected error for tampered token, got nil")
	}
}

func TestParseToken_RandomString_ReturnsError(t *testing.T) {
	_, err := ParseToken(testSecret, "notavalidjwtatall")
	if err == nil {
		t.Fatal("expected error for random string, got nil")
	}
}

func TestParseToken_EmptyString_ReturnsError(t *testing.T) {
	_, err := ParseToken(testSecret, "")
	if err == nil {
		t.Fatal("expected error for empty token, got nil")
	}
}

func TestParseToken_ExpiredToken_ReturnsError(t *testing.T) {
	expired, err := CreateToken(testSecret, "user123", "owner", -1*time.Second)
	if err != nil {
		t.Fatalf("failed to create expired token: %v", err)
	}

	_, err = ParseToken(testSecret, expired)
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
}

func TestParseToken_AccessTokenRejectedByRefreshSecret(t *testing.T) {
	// Tokens signed with access secret must be rejected by refresh secret.
	token, _ := CreateAccessToken(testSecret, "user123", "owner")

	_, err := ParseToken(testRefreshSecret, token)
	if err == nil {
		t.Fatal("access token must not be accepted by refresh secret")
	}
}

func TestParseToken_RefreshTokenRejectedByAccessSecret(t *testing.T) {
	// Refresh tokens must be rejected when verified with the access secret.
	refresh, _ := CreateRefreshToken(testRefreshSecret, "user123", "owner")

	_, err := ParseToken(testSecret, refresh)
	if err == nil {
		t.Fatal("refresh token must not be accepted by access secret")
	}
}
